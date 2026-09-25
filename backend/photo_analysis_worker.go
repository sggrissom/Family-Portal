package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"family/cfg"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"go.hasen.dev/vbolt"
)

const currentAnalysisVersion = 1

const analysisMaxDimension = 2048

type PhotoAnalysisJob struct {
	ImageId  int
	FamilyId int
}

type photoAnalysisWorker struct {
	workerLifecycle
	jobQueue chan PhotoAnalysisJob
	db       *vbolt.DB
	client   *http.Client

	backlogMu sync.Mutex
	backlog   []PhotoAnalysisJob
	wake      chan struct{}
}

var globalAnalysisWorker *photoAnalysisWorker

type AnalysisWorkerStats struct {
	QueueLength int  `json:"queueLength"`
	IsRunning   bool `json:"isRunning"`
}

func GetAnalysisWorkerStats() AnalysisWorkerStats {
	if globalAnalysisWorker == nil {
		return AnalysisWorkerStats{}
	}
	globalAnalysisWorker.backlogMu.Lock()
	backlog := len(globalAnalysisWorker.backlog)
	globalAnalysisWorker.backlogMu.Unlock()
	return AnalysisWorkerStats{
		QueueLength: len(globalAnalysisWorker.jobQueue) + backlog,
		IsRunning:   globalAnalysisWorker.isRunning(),
	}
}

func InitializeAnalysisWorker(db *vbolt.DB) {
	if !cfg.EnableFaceTagging {
		LogInfo(LogCategoryWorker, "Face tagging disabled, skipping analysis worker initialization")
		return
	}

	if globalAnalysisWorker != nil {
		LogInfo(LogCategoryWorker, "Analysis worker already initialized, skipping")
		return
	}

	conn, err := net.Dial("unix", cfg.FaceAnalysisSocket)
	if err != nil {
		LogInfo(LogCategoryWorker, "Face daemon not reachable", map[string]interface{}{
			"error":  err.Error(),
			"socket": cfg.FaceAnalysisSocket,
		})
		return
	}
	conn.Close()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", cfg.FaceAnalysisSocket)
			},
		},
	}

	globalAnalysisWorker = &photoAnalysisWorker{
		jobQueue: make(chan PhotoAnalysisJob, 100),
		db:       db,
		client:   client,
		wake:     make(chan struct{}, 1),
	}

	quit, done, _ := globalAnalysisWorker.start()
	go globalAnalysisWorker.processJobs(quit, done)
	LogInfo(LogCategoryWorker, "Photo analysis worker started", map[string]interface{}{
		"socket": cfg.FaceAnalysisSocket,
	})
}

func QueuePhotoAnalysis(job PhotoAnalysisJob) {
	if globalAnalysisWorker == nil {
		return
	}
	select {
	case globalAnalysisWorker.jobQueue <- job:
		log.Printf("[FACE_ANALYSIS] Photo %d queued for analysis", job.ImageId)
	default:
		globalAnalysisWorker.addBacklog([]PhotoAnalysisJob{job})
		log.Printf("[FACE_ANALYSIS] Analysis queue full; photo %d added to backlog", job.ImageId)
	}
}

func QueueAnalysisBacklog(jobs []PhotoAnalysisJob) {
	if globalAnalysisWorker == nil {
		return
	}
	globalAnalysisWorker.addBacklog(jobs)
}

func (aw *photoAnalysisWorker) addBacklog(jobs []PhotoAnalysisJob) {
	if len(jobs) == 0 {
		return
	}
	aw.backlogMu.Lock()
	aw.backlog = append(aw.backlog, jobs...)
	aw.backlogMu.Unlock()
	select {
	case aw.wake <- struct{}{}:
	default:
	}
}

func (aw *photoAnalysisWorker) popBacklog() (PhotoAnalysisJob, bool) {
	aw.backlogMu.Lock()
	defer aw.backlogMu.Unlock()
	if len(aw.backlog) == 0 {
		return PhotoAnalysisJob{}, false
	}
	job := aw.backlog[0]
	aw.backlog = aw.backlog[1:]
	return job, true
}

func TriggerPersonFaceUpdate(personId int) {
	if globalAnalysisWorker == nil {
		return
	}
	if err := updatePersonEmbedding(globalAnalysisWorker.db, globalAnalysisWorker.client, personId); err != nil {
		log.Printf("[FACE_ANALYSIS] Failed to update face embedding for person %d: %v", personId, err)
	}
}

func (aw *photoAnalysisWorker) processJobs(quit <-chan struct{}, done chan struct{}) {
	defer close(done)
	aw.backfillPersonEmbeddings(quit)
	aw.addBacklog(photosNeedingAnalysis(aw.db))
	for {
		select {
		case <-quit:
			aw.logStopped()
			return
		case job := <-aw.jobQueue:
			aw.processAnalysisJob(job)
			continue
		default:
		}

		if job, ok := aw.popBacklog(); ok {
			aw.processAnalysisJob(job)
			continue
		}

		select {
		case <-quit:
			aw.logStopped()
			return
		case job := <-aw.jobQueue:
			aw.processAnalysisJob(job)
		case <-aw.wake:
		}
	}
}

func (aw *photoAnalysisWorker) logStopped() {
	aw.backlogMu.Lock()
	backlog := len(aw.backlog)
	aw.backlogMu.Unlock()
	LogInfo(LogCategoryWorker, "Photo analysis worker stopped", map[string]interface{}{
		"abandoned": len(aw.jobQueue) + backlog,
	})
}

func imageNeedsAnalysis(image Image) bool {
	switch image.AnalysisStatus {
	case 0, 1:
		return true
	case 2:
		return image.AnalysisVersion < currentAnalysisVersion
	}
	return false
}

func photosNeedingAnalysis(db *vbolt.DB) (jobs []PhotoAnalysisJob) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
			if image.Status == 0 && imageNeedsAnalysis(image) {
				jobs = append(jobs, PhotoAnalysisJob{ImageId: image.Id, FamilyId: image.FamilyId})
			}
			return true
		})
	})
	if len(jobs) > 0 {
		LogInfo(LogCategoryWorker, "Queued photos for face analysis backfill", map[string]interface{}{
			"count": len(jobs),
		})
	}
	return
}

func (aw *photoAnalysisWorker) backfillPersonEmbeddings(quit <-chan struct{}) {
	var personIds []int
	vbolt.WithReadTx(aw.db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, PeopleBkt, func(_ int, p Person) bool {
			if p.ProfilePhotoId != 0 && len(p.FaceDescriptor) != 128 {
				personIds = append(personIds, p.Id)
			}
			return true
		})
	})
	if len(personIds) == 0 {
		return
	}

	failed := 0
	for _, personId := range personIds {
		select {
		case <-quit:
			return
		default:
		}
		if err := updatePersonEmbedding(aw.db, aw.client, personId); err != nil {
			log.Printf("[FACE_ANALYSIS] Backfill failed for person %d: %v", personId, err)
			failed++
		}
	}
	LogInfo(LogCategoryWorker, "Face embedding backfill finished", map[string]interface{}{
		"candidates": len(personIds),
		"failed":     failed,
	})
}

func StopAnalysisWorker(ctx context.Context) bool {
	if globalAnalysisWorker == nil {
		return true
	}
	return globalAnalysisWorker.stopAndWait(ctx, false)
}

func (aw *photoAnalysisWorker) processAnalysisJob(job PhotoAnalysisJob) {
	log.Printf("[FACE_ANALYSIS] Starting analysis of photo %d", job.ImageId)

	if err := aw.setAnalysisStatus(job.ImageId, 1); err != nil {
		log.Printf("[FACE_ANALYSIS] Failed to set analyzing status for photo %d: %v", job.ImageId, err)
		return
	}

	var img Image
	vbolt.WithReadTx(aw.db, func(tx *vbolt.Tx) {
		img = GetImageById(tx, job.ImageId)
	})
	if img.Id == 0 {
		return
	}

	faces, err := detectFaces(aw.client, img)
	if err != nil {
		log.Printf("[FACE_ANALYSIS] Face detection failed for photo %d: %v", job.ImageId, err)
		aw.setAnalysisStatus(job.ImageId, 3)
		return
	}

	tagged := 0
	vbolt.WithWriteTx(aw.db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, job.ImageId)
		if img.Id == 0 {
			return
		}
		tagged = RecordPhotoFacesTx(tx, img.Id, faces)
		img = GetImageById(tx, job.ImageId)
		img.AnalysisStatus = 2
		img.AnalysisVersion = currentAnalysisVersion
		vbolt.Write(tx, ImagesBkt, img.Id, &img)
		vbolt.TxCommit(tx)
	})
	log.Printf("[FACE_ANALYSIS] Completed analysis of photo %d: %d face(s), %d auto-tagged", job.ImageId, len(faces), tagged)
}

type recognizeRequest struct {
	ImagePath string `json:"image_path"`
	ImageData []byte `json:"image_data,omitempty"`
}

type recognizedFace struct {
	Descriptor []float32 `json:"descriptor"`
	Rect       [4]int    `json:"rect"`
}

type recognizeResponse struct {
	Descriptors [][]float32      `json:"descriptors"`
	Faces       []recognizedFace `json:"faces"`
	Source      string           `json:"source"`
}

func callRecognize(client *http.Client, req recognizeRequest) (recognizeResponse, error) {
	var result recognizeResponse
	body, _ := json.Marshal(req)
	resp, err := client.Post("http://face/recognize", "application/json", bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("face daemon returned status %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func detectFaces(client *http.Client, img Image) ([]DetectedFace, error) {
	path := analysisImagePath(img)
	data, dataSize, dataErr := analysisImageData(img)
	if path == "" && dataErr != nil {
		return nil, fmt.Errorf("no readable image for photo %d: %w", img.Id, dataErr)
	}

	result, err := callRecognize(client, recognizeRequest{ImagePath: path, ImageData: data})
	if err != nil {
		return nil, err
	}
	if result.Source == "data" {
		return toDetectedFaces(result, dataSize), nil
	}
	return toDetectedFaces(result, imageSize(path)), nil
}

// The display variants are too small for the detector to find faces in group
// shots, so analysis works from the original, upright and capped in size.
func analysisImageData(img Image) ([]byte, image.Point, error) {
	decoded, err := imaging.Open(getOriginalPhotoPath(img), imaging.AutoOrientation(true))
	if err != nil {
		return nil, image.Point{}, err
	}
	b := decoded.Bounds()
	if b.Dx() > analysisMaxDimension || b.Dy() > analysisMaxDimension {
		decoded = imaging.Fit(decoded, analysisMaxDimension, analysisMaxDimension, imaging.Lanczos)
	}
	rgb := imaging.Clone(decoded)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgb, &jpeg.Options{Quality: 90}); err != nil {
		return nil, image.Point{}, err
	}
	return buf.Bytes(), image.Point{X: rgb.Bounds().Dx(), Y: rgb.Bounds().Dy()}, nil
}

func imageSize(path string) image.Point {
	f, err := os.Open(path)
	if err != nil {
		return image.Point{}
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return image.Point{}
	}
	return image.Point{X: config.Width, Y: config.Height}
}

func toDetectedFaces(result recognizeResponse, size image.Point) []DetectedFace {
	var faces []DetectedFace
	if result.Faces == nil {
		for _, d := range result.Descriptors {
			faces = append(faces, DetectedFace{Descriptor: d})
		}
		return faces
	}
	for _, f := range result.Faces {
		face := DetectedFace{Descriptor: f.Descriptor}
		if size.X > 0 && size.Y > 0 {
			clamp := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
			face.Box = FaceBox{
				Left:   clamp(float64(f.Rect[0]) / float64(size.X)),
				Top:    clamp(float64(f.Rect[1]) / float64(size.Y)),
				Right:  clamp(float64(f.Rect[2]) / float64(size.X)),
				Bottom: clamp(float64(f.Rect[3]) / float64(size.Y)),
			}
		}
		faces = append(faces, face)
	}
	return faces
}

func analysisImagePath(img Image) string {
	basePath := filepath.Join(cfg.StaticDir, img.FilePath)
	base := strings.TrimSuffix(basePath, filepath.Ext(basePath))
	for _, candidate := range []string{base + "_xlarge.jpg", base + ".jpg", base + "_medium.jpg"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func (aw *photoAnalysisWorker) setAnalysisStatus(imageId int, status int) error {
	var updateErr error
	vbolt.WithWriteTx(aw.db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, imageId)
		if img.Id == 0 {
			updateErr = fmt.Errorf("image not found")
			return
		}
		img.AnalysisStatus = status
		vbolt.Write(tx, ImagesBkt, img.Id, &img)
		vbolt.TxCommit(tx)
	})
	return updateErr
}

// Picks the face a profile photo is about: one already assigned to the person,
// otherwise the detected face closest to the crop's focal point.
func pickProfileFace(person Person, known []PhotoFace, detected []DetectedFace) []float32 {
	for _, face := range known {
		if face.PersonId == person.Id && len(face.Descriptor) == 128 {
			return face.Descriptor
		}
	}
	if len(detected) == 0 {
		return nil
	}
	cx, cy := person.ProfileCropX/100, person.ProfileCropY/100
	if cx == 0 && cy == 0 {
		cx, cy = 0.5, 0.5
	}
	best, bestDist := 0, math.MaxFloat64
	for i, face := range detected {
		fx := (face.Box.Left + face.Box.Right) / 2
		fy := (face.Box.Top + face.Box.Bottom) / 2
		if d := math.Hypot(fx-cx, fy-cy); d < bestDist {
			best, bestDist = i, d
		}
	}
	return detected[best].Descriptor
}

func updatePersonEmbedding(db *vbolt.DB, client *http.Client, personId int) error {
	var person Person
	var img Image
	var known []PhotoFace
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		person = GetPersonById(tx, personId)
		if person.ProfilePhotoId == 0 {
			return
		}
		img = GetImageById(tx, person.ProfilePhotoId)
		if img.Id != 0 {
			known = GetPhotoFacesTx(tx, img.Id)
		}
	})

	descriptor := pickProfileFace(person, known, nil)
	if descriptor == nil {
		if img.Id == 0 {
			return nil
		}
		detected, err := detectFaces(client, img)
		if err != nil {
			return fmt.Errorf("face embedding failed: %w", err)
		}
		descriptor = pickProfileFace(person, nil, detected)
	}
	if len(descriptor) != 128 {
		log.Printf("[FACE_ANALYSIS] No face found in profile photo for person %d", personId)
		return nil
	}

	tagged := 0
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		p := GetPersonById(tx, personId)
		if p.Id == 0 {
			return
		}
		p.FaceDescriptor = descriptor
		vbolt.Write(tx, PeopleBkt, p.Id, &p)
		tagged = RematchFamilyFacesTx(tx, p.FamilyId)
		vbolt.TxCommit(tx)
	})

	log.Printf("[FACE_ANALYSIS] Updated face embedding for person %d (%d photos newly auto-tagged)", personId, tagged)
	return nil
}
