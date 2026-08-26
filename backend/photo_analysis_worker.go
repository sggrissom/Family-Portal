//go:build release

package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"family/cfg"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

type PhotoAnalysisJob struct {
	ImageId  int
	FamilyId int
}

type photoAnalysisWorker struct {
	workerLifecycle
	jobQueue chan PhotoAnalysisJob
	db       *vbolt.DB
	client   *http.Client
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
		QueueLength: len(globalAnalysisWorker.jobQueue),
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
		log.Printf("[FACE_ANALYSIS] Cannot queue photo %d: analysis queue full", job.ImageId)
	}
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
	for {
		select {
		case job := <-aw.jobQueue:
			aw.processAnalysisJob(job)
		case <-quit:
			LogInfo(LogCategoryWorker, "Photo analysis worker stopped", map[string]interface{}{
				"abandoned": len(aw.jobQueue),
			})
			return
		}
	}
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

	imagePath := aw.resolveImagePath(job.ImageId)
	if imagePath == "" {
		log.Printf("[FACE_ANALYSIS] Image file not found for photo %d", job.ImageId)
		aw.setAnalysisStatus(job.ImageId, 3)
		return
	}

	descriptors, err := callRecognize(aw.client, imagePath)
	if err != nil {
		log.Printf("[FACE_ANALYSIS] Face detection failed for photo %d: %v", job.ImageId, err)
		aw.setAnalysisStatus(job.ImageId, 3)
		return
	}
	log.Printf("[FACE_ANALYSIS] Detected %d face(s) in photo %d", len(descriptors), job.ImageId)

	if len(descriptors) > 0 {
		aw.matchAndTagFaces(job, descriptors)
	}

	aw.setAnalysisStatus(job.ImageId, 2)
	log.Printf("[FACE_ANALYSIS] Completed analysis of photo %d", job.ImageId)
}

type recognizeRequest struct {
	ImagePath string `json:"image_path"`
}

type recognizeResponse struct {
	Descriptors [][]float32 `json:"descriptors"`
}

type embedRequest struct {
	ImagePath string `json:"image_path"`
}

type embedResponse struct {
	Descriptor []float32 `json:"descriptor"`
}

func callRecognize(client *http.Client, imagePath string) ([][]float32, error) {
	body, _ := json.Marshal(recognizeRequest{ImagePath: imagePath})
	resp, err := client.Post("http://face/recognize", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("face daemon returned status %d", resp.StatusCode)
	}
	var result recognizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Descriptors, nil
}

func callEmbed(client *http.Client, imagePath string) ([]float32, error) {
	body, _ := json.Marshal(embedRequest{ImagePath: imagePath})
	resp, err := client.Post("http://face/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("face daemon returned status %d", resp.StatusCode)
	}
	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Descriptor, nil
}

func (aw *photoAnalysisWorker) resolveImagePath(imageId int) string {
	var imagePath string
	vbolt.WithReadTx(aw.db, func(tx *vbolt.Tx) {
		img := GetImageById(tx, imageId)
		if img.Id == 0 {
			return
		}
		basePath := filepath.Join(cfg.StaticDir, img.FilePath)
		base := strings.TrimSuffix(basePath, filepath.Ext(basePath))
		mediumPath := base + "_medium.jpg"
		if _, err := os.Stat(mediumPath); err == nil {
			imagePath = mediumPath
			return
		}
		largePath := base + ".jpg"
		if _, err := os.Stat(largePath); err == nil {
			imagePath = largePath
		}
	})
	return imagePath
}

func (aw *photoAnalysisWorker) matchAndTagFaces(job PhotoAnalysisJob, descriptors [][]float32) {
	var knownPersons []Person
	vbolt.WithReadTx(aw.db, func(tx *vbolt.Tx) {
		all := GetFamilyOwnPeople(tx, job.FamilyId)
		for _, p := range all {
			if len(p.FaceDescriptor) == 128 {
				knownPersons = append(knownPersons, p)
			}
		}
	})

	if len(knownPersons) == 0 {
		return
	}

	existingPersonIds := make(map[int]bool)
	vbolt.WithReadTx(aw.db, func(tx *vbolt.Tx) {
		for _, pp := range GetPhotoPersonsByPhoto(tx, job.ImageId) {
			existingPersonIds[pp.PersonId] = true
		}
	})

	const matchThreshold = 0.6
	matched := make(map[int]bool)

	for _, detected := range descriptors {
		bestDist := math.MaxFloat64
		bestPersonId := 0

		for _, known := range knownPersons {
			dist := faceEuclideanDistance(detected, known.FaceDescriptor)
			if dist < bestDist {
				bestDist = dist
				bestPersonId = known.Id
			}
		}

		if bestPersonId == 0 || bestDist >= matchThreshold {
			continue
		}
		if existingPersonIds[bestPersonId] || matched[bestPersonId] {
			continue
		}

		matched[bestPersonId] = true
		tagged := false
		vbolt.WithWriteTx(aw.db, func(tx *vbolt.Tx) {
			if GetImageById(tx, job.ImageId).Id == 0 || GetPersonById(tx, bestPersonId).Id == 0 {
				return
			}
			addAutoTaggedPersonToPhoto(tx, job.ImageId, bestPersonId, job.FamilyId)
			vbolt.TxCommit(tx)
			tagged = true
		})
		if !tagged {
			log.Printf("[FACE_ANALYSIS] Photo %d or person %d disappeared mid-analysis; not tagging", job.ImageId, bestPersonId)
			continue
		}
		log.Printf("[FACE_ANALYSIS] Auto-tagged person %d in photo %d (dist: %.3f)", bestPersonId, job.ImageId, bestDist)
	}
}

func addAutoTaggedPersonToPhoto(tx *vbolt.Tx, photoId int, personId int, familyId int) {
	pp := PhotoPerson{
		Id:         vbolt.NextIntId(tx, PhotoPersonBkt),
		PhotoId:    photoId,
		PersonId:   personId,
		FamilyId:   familyId,
		CreatedAt:  time.Now(),
		AutoTagged: true,
	}
	vbolt.Write(tx, PhotoPersonBkt, pp.Id, &pp)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, pp.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, pp.Id, personId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, pp.Id, familyId)
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

func updatePersonEmbedding(db *vbolt.DB, client *http.Client, personId int) error {
	var imagePath string
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		person := GetPersonById(tx, personId)
		if person.ProfilePhotoId == 0 {
			return
		}
		img := GetImageById(tx, person.ProfilePhotoId)
		if img.Id == 0 {
			return
		}
		basePath := filepath.Join(cfg.StaticDir, img.FilePath)
		base := strings.TrimSuffix(basePath, filepath.Ext(basePath))
		mediumPath := base + "_medium.jpg"
		if _, err := os.Stat(mediumPath); err == nil {
			imagePath = mediumPath
			return
		}
		largePath := base + ".jpg"
		if _, err := os.Stat(largePath); err == nil {
			imagePath = largePath
		}
	})

	if imagePath == "" {
		return nil
	}

	descriptor, err := callEmbed(client, imagePath)
	if err != nil {
		return fmt.Errorf("face embedding failed: %w", err)
	}
	if len(descriptor) == 0 {
		log.Printf("[FACE_ANALYSIS] No face found in profile photo for person %d", personId)
		return nil
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		p := GetPersonById(tx, personId)
		if p.Id == 0 {
			return
		}
		p.FaceDescriptor = descriptor
		vbolt.Write(tx, PeopleBkt, p.Id, &p)
		vbolt.TxCommit(tx)
	})

	log.Printf("[FACE_ANALYSIS] Updated face embedding for person %d", personId)
	return nil
}

func faceEuclideanDistance(a, b []float32) float64 {
	var sum float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}
