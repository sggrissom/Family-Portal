package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/disintegration/imaging"
	"go.hasen.dev/vbolt"
)

const currentAnalysisVersion = 1

const analysisMaxDimension = 2048

type PhotoAnalysisJob struct {
	ImageId  int
	FamilyId int
	// PersonId identifies a profile-face update instead of a photo analysis.
	PersonId int
}

type photoAnalysisWorker struct {
	*backlogWorker[PhotoAnalysisJob]
	db     *vbolt.DB
	client *http.Client
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
	return AnalysisWorkerStats{
		QueueLength: globalAnalysisWorker.length(),
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

	initializeAnalysisWorker(db, cfg.FaceAnalysisSocket)
}

func initializeAnalysisWorker(db *vbolt.DB, socket string) {
	aw := &photoAnalysisWorker{db: db, client: newFaceClient(socket)}
	aw.backlogWorker = newBacklogWorker("Photo analysis worker", aw.processAnalysisJob)
	globalAnalysisWorker = aw
	aw.run(func() []PhotoAnalysisJob {
		return append(peopleNeedingFaceEmbeddings(db), photosNeedingAnalysis(db)...)
	})
	LogInfo(LogCategoryWorker, "Photo analysis worker started", map[string]interface{}{
		"socket": socket,
	})
}

func newFaceClient(socket string) *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
		},
	}
}

func QueuePhotoAnalysis(job PhotoAnalysisJob) {
	if globalAnalysisWorker != nil {
		globalAnalysisWorker.add(job)
	}
}

func QueueAnalysisBacklog(jobs []PhotoAnalysisJob) {
	if globalAnalysisWorker != nil {
		globalAnalysisWorker.add(jobs...)
	}
}

func TriggerPersonFaceUpdate(personId int) {
	if globalAnalysisWorker != nil {
		globalAnalysisWorker.add(PhotoAnalysisJob{PersonId: personId})
	}
}

func imageNeedsAnalysis(image Image) bool {
	if image.IsVideo() {
		return false
	}
	switch image.AnalysisStatus {
	// Retry old failures once on startup as well; older workers recorded
	// daemon outages as permanent failures.
	case AnalysisPending, AnalysisRunning, AnalysisFailed:
		return true
	case AnalysisDone:
		return image.AnalysisVersion < currentAnalysisVersion
	}
	return false
}

func photosNeedingAnalysis(db *vbolt.DB) (jobs []PhotoAnalysisJob) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(_ int, image Image) bool {
			if image.Status == PhotoReady && imageNeedsAnalysis(image) {
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

func peopleNeedingFaceEmbeddings(db *vbolt.DB) (jobs []PhotoAnalysisJob) {
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, PeopleBkt, func(_ int, p Person) bool {
			if p.ProfilePhotoId != 0 && len(p.FaceDescriptor) != 128 {
				jobs = append(jobs, PhotoAnalysisJob{PersonId: p.Id})
			}
			return true
		})
	})
	return
}

func StopAnalysisWorker(ctx context.Context) bool {
	if globalAnalysisWorker == nil {
		return true
	}
	return globalAnalysisWorker.stopWait(ctx)
}

func (aw *photoAnalysisWorker) processAnalysisJob(job PhotoAnalysisJob) error {
	if job.PersonId != 0 {
		err := updatePersonEmbedding(aw.db, aw.client, job.PersonId)
		if errors.Is(err, errRetryLater) {
			return errRetryLater
		}
		if err != nil {
			LogWarn(LogCategoryWorker, "Face embedding update failed", map[string]interface{}{"personId": job.PersonId, "error": err.Error()})
		}
		return nil
	}
	var img Image
	vbolt.WithReadTx(aw.db, func(tx *vbolt.Tx) {
		img = GetImageById(tx, job.ImageId)
	})
	if img.Id == 0 || img.IsVideo() {
		return nil
	}

	if err := aw.setAnalysisStatus(job.ImageId, AnalysisRunning); err != nil {
		LogWarn(LogCategoryWorker, "Face analysis could not start", map[string]interface{}{"photoId": job.ImageId, "error": err.Error()})
		return nil
	}

	faces, err := detectFaces(aw.client, img)
	if err != nil {
		LogWarn(LogCategoryWorker, "Face detection failed", map[string]interface{}{"photoId": job.ImageId, "error": err.Error()})
		if errors.Is(err, errRetryLater) {
			aw.setAnalysisStatus(job.ImageId, AnalysisPending)
			return errRetryLater
		}
		aw.setAnalysisStatus(job.ImageId, AnalysisFailed)
		return nil
	}

	tagged := 0
	vbolt.WithWriteTx(aw.db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, job.ImageId)
		if img.Id == 0 {
			return
		}
		tagged = RecordPhotoFacesTx(tx, img.Id, faces)
		img = GetImageById(tx, job.ImageId)
		img.AnalysisStatus = AnalysisDone
		img.AnalysisVersion = currentAnalysisVersion
		writeImage(tx, &img)
		vbolt.TxCommit(tx)
	})
	LogInfo(LogCategoryWorker, "Face analysis finished", map[string]interface{}{
		"photoId": job.ImageId, "faces": len(faces), "autoTagged": tagged,
	})
	return nil
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
		return result, fmt.Errorf("%w: %v", errRetryLater, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return result, fmt.Errorf("%w: face daemon returned status %d", errRetryLater, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("face daemon returned status %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func detectFaces(client *http.Client, img Image) ([]DetectedFace, error) {
	path := photoSourcePath(img.FilePath, XLargeSize.Name, LargeSize.Name, MediumSize.Name)
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
	return analysisFileData(photoOriginalPath(img.FilePath))
}

func analysisFileData(path string) ([]byte, image.Point, error) {
	decoded, err := imaging.Open(path, imaging.AutoOrientation(true))
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

func detectFrameFaces(client *http.Client, file string) ([]DetectedFace, error) {
	data, size, err := analysisFileData(file)
	if err != nil {
		return nil, err
	}
	result, err := callRecognize(client, recognizeRequest{ImageData: data})
	if err != nil {
		return nil, err
	}
	return toDetectedFaces(result, size), nil
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

func (aw *photoAnalysisWorker) setAnalysisStatus(imageId int, status int) error {
	var updateErr error
	vbolt.WithWriteTx(aw.db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, imageId)
		if img.Id == 0 {
			updateErr = fmt.Errorf("image not found")
			return
		}
		img.AnalysisStatus = status
		writeImage(tx, &img)
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
		if img.IsVideo() {
			img = Image{}
		}
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
		LogInfo(LogCategoryWorker, "No face found in a profile photo", map[string]interface{}{"personId": personId})
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
	LogInfo(LogCategoryWorker, "Face embedding updated", map[string]interface{}{
		"personId": personId, "autoTagged": tagged,
	})
	return nil
}
