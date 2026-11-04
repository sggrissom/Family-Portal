package backend

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"go.hasen.dev/vbolt"
)

type PhotoProcessingJob struct {
	ImageId        int
	FamilyId       int
	FilePath       string
	FileData       []byte
	MimeType       string
	OriginalWidth  int
	OriginalHeight int
	Reprocess      bool
}

const maxRecentPhotoAttempts = 20

type PhotoAttempt struct {
	Time       time.Time `json:"time"`
	ImageId    int       `json:"imageId"`
	Reprocess  bool      `json:"reprocess"`
	Success    bool      `json:"success"`
	DurationMs int       `json:"durationMs"`
	Reason     string    `json:"reason"`
}

type PhotoWorker struct {
	workerLifecycle
	jobQueue chan PhotoProcessingJob
	db       *vbolt.DB

	statsMu       sync.Mutex
	processed     int
	failed        int
	lastProcessed time.Time
	lastError     string
	lastErrorAt   time.Time
	recent        []PhotoAttempt
}

func (pw *PhotoWorker) recordAttempt(attempt PhotoAttempt) {
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()

	if attempt.Success {
		pw.processed++
		pw.lastProcessed = attempt.Time
	} else {
		pw.failed++
		pw.lastError = attempt.Reason
		pw.lastErrorAt = attempt.Time
	}

	pw.recent = append(pw.recent, attempt)
	if len(pw.recent) > maxRecentPhotoAttempts {
		pw.recent = pw.recent[len(pw.recent)-maxRecentPhotoAttempts:]
	}
}

var globalPhotoWorker *PhotoWorker

var errPhotoRecordGone = errors.New("photo record no longer exists")

func InitializePhotoWorker(queueSize int, db *vbolt.DB) {
	if globalPhotoWorker != nil {
		LogInfo(LogCategoryWorker, "Photo worker already initialized, skipping")
		return
	}

	LogInfo(LogCategoryWorker, "Initializing photo processing worker", map[string]interface{}{"queueSize": queueSize})
	globalPhotoWorker = &PhotoWorker{
		jobQueue: make(chan PhotoProcessingJob, queueSize),
		db:       db,
	}

	LogInfo(LogCategoryWorker, "Photo worker initialized with database reference")
	globalPhotoWorker.Start()
	LogInfo(LogCategoryWorker, "Photo processing worker started")
}

var errPhotoWorkerStopped = errors.New("photo worker stopped")

func activePhotoWorker() *PhotoWorker {
	pw := globalPhotoWorker
	if pw == nil || !pw.isRunning() {
		return nil
	}
	return pw
}

func (pw *PhotoWorker) queueBlocking(job PhotoProcessingJob) error {
	select {
	case pw.jobQueue <- job:
		return nil
	case <-pw.stopping():
		return errPhotoWorkerStopped
	}
}

var backlogFeeders sync.WaitGroup

func waitForBacklogFeeders() {
	backlogFeeders.Wait()
}

func queueBacklog(pw *PhotoWorker, jobs []PhotoProcessingJob, failureMsg, doneMsg string) {
	if pw == nil || len(jobs) == 0 {
		return
	}

	backlogFeeders.Add(1)
	go func() {
		defer backlogFeeders.Done()
		for _, job := range jobs {
			if err := pw.queueBlocking(job); err != nil {
				LogErrorSimple(LogCategoryAdmin, failureMsg, map[string]interface{}{
					"photoId": job.ImageId,
					"error":   err.Error(),
				})
				return
			}
		}
		LogInfo(LogCategoryAdmin, doneMsg, map[string]interface{}{
			"count": len(jobs),
		})
	}()
}

func QueuePhotoProcessing(job PhotoProcessingJob) error {
	if globalPhotoWorker == nil {
		log.Printf("Cannot queue photo %d: worker not initialized", job.ImageId)
		return fmt.Errorf("photo worker not initialized")
	}

	select {
	case globalPhotoWorker.jobQueue <- job:
		log.Printf("Photo %d queued for background processing (queue length: %d)", job.ImageId, len(globalPhotoWorker.jobQueue))
		return nil
	default:
	}

	// The original is already on disk, so a burst of uploads waits for room
	// instead of failing photos that were saved.
	pw := activePhotoWorker()
	if pw == nil {
		log.Printf("Cannot queue photo %d: processing queue is full", job.ImageId)
		return fmt.Errorf("processing queue is full")
	}
	job.FileData = nil
	log.Printf("Photo %d deferred until the processing queue has room", job.ImageId)
	backlogFeeders.Add(1)
	go func() {
		defer backlogFeeders.Done()
		if err := pw.queueBlocking(job); err != nil {
			LogWarn(LogCategoryWorker, "Deferred photo was not queued; it will show as stuck", map[string]interface{}{
				"photoId": job.ImageId,
				"error":   err.Error(),
			})
		}
	}()
	return nil
}

func (pw *PhotoWorker) Start() {
	quit, done, ok := pw.start()
	if !ok {
		return
	}

	go pw.processJobs(quit, done)
	LogInfo(LogCategoryWorker, "Photo processing worker started")
}

func (pw *PhotoWorker) Stop() {
	pw.stopImmediately()
	LogInfo(LogCategoryWorker, "Photo processing worker stopped")
}

func (pw *PhotoWorker) StopAndDrain(ctx context.Context) bool {
	return pw.stopAndWait(ctx, true)
}

func GetQueueLength() int {
	if globalPhotoWorker == nil {
		return 0
	}
	return len(globalPhotoWorker.jobQueue)
}

func (pw *PhotoWorker) processJobs(quit <-chan struct{}, done chan struct{}) {
	defer close(done)
	for {
		select {
		case job := <-pw.jobQueue:
			pw.processPhotoJob(job)
		case <-quit:
			drained := drainQueue(pw.drainContext(), pw.jobQueue, pw.processPhotoJob)
			LogInfo(LogCategoryWorker, "Photo worker received stop signal", map[string]interface{}{
				"drained":   drained,
				"abandoned": len(pw.jobQueue),
			})
			return
		}
	}
}

func (pw *PhotoWorker) processPhotoJob(job PhotoProcessingJob) {
	startTime := time.Now()
	log.Printf("[PHOTO_PROCESSING] Starting processing of photo ID %d (size: %d bytes)", job.ImageId, len(job.FileData))

	outcome := func(success bool, reason string) {
		pw.recordAttempt(PhotoAttempt{
			Time:       time.Now(),
			ImageId:    job.ImageId,
			Reprocess:  job.Reprocess,
			Success:    success,
			DurationMs: int(time.Since(startTime).Milliseconds()),
			Reason:     reason,
		})
	}

	log.Printf("[PHOTO_PROCESSING] Setting status to processing for photo %d", job.ImageId)
	err := pw.updatePhotoStatus(job.ImageId, 1)
	if err != nil {
		if errors.Is(err, errPhotoRecordGone) {
			pw.discardJobFiles(job)
			return
		}
		log.Printf("[PHOTO_PROCESSING] FAILED to update photo %d status to processing: %v", job.ImageId, err)
		outcome(false, "could not mark the photo as processing: "+err.Error())
		return
	}

	sourceData := job.FileData
	if len(sourceData) == 0 {
		sourceData, err = os.ReadFile(photoOriginalPath(job.FilePath))
		if err != nil {
			log.Printf("[PHOTO_PROCESSING] FAILED to read original for photo ID %d: %v", job.ImageId, err)
			pw.updatePhotoStatus(job.ImageId, 2)
			outcome(false, "original file is missing: "+err.Error())
			return
		}
	}

	if job.Reprocess {
		removePhotoVariants(job.FilePath)
	}

	log.Printf("[PHOTO_PROCESSING] Processing image formats and sizes for photo %d", job.ImageId)
	processedImages, processedWidth, processedHeight, err := ProcessAndSaveMultipleSizes(sourceData, job.MimeType)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to process photo ID %d: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2)
		outcome(false, "could not decode or re-encode the image: "+err.Error())
		return
	}
	log.Printf("[PHOTO_PROCESSING] Generated %d image variants for photo %d", len(processedImages), job.ImageId)

	log.Printf("[PHOTO_PROCESSING] Saving image variants to disk for photo %d", job.ImageId)
	_, err = writeImageVariants(job.FilePath, processedImages)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to save photo variants for ID %d: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2)
		outcome(false, "could not write the variants to disk: "+err.Error())
		return
	}
	log.Printf("[PHOTO_PROCESSING] Successfully saved all variants for photo %d", job.ImageId)

	log.Printf("[PHOTO_PROCESSING] Marking photo %d as completed", job.ImageId)
	err = pw.updatePhotoComplete(job.ImageId, processedWidth, processedHeight)
	if err != nil {
		if errors.Is(err, errPhotoRecordGone) {
			pw.discardJobFiles(job)
			return
		}
		log.Printf("[PHOTO_PROCESSING] FAILED to mark photo %d as complete: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2)
		outcome(false, "could not mark the photo complete: "+err.Error())
		return
	}

	QueuePhotoAnalysis(PhotoAnalysisJob{ImageId: job.ImageId, FamilyId: job.FamilyId})
	QueuePhotoFeatures(job.ImageId)
	QueuePhotoEmbedding(job.ImageId)

	outcome(true, "")
	processingTime := time.Since(startTime)
	log.Printf("[PHOTO_PROCESSING] ✅ Successfully completed photo ID %d in %v", job.ImageId, processingTime)
}

func (pw *PhotoWorker) discardJobFiles(job PhotoProcessingJob) {
	log.Printf("[PHOTO_PROCESSING] Photo %d was deleted mid-processing; discarding its files", job.ImageId)
	if err := deletePhotoFiles(Image{FilePath: job.FilePath}); err != nil {
		LogErrorSimple(LogCategoryWorker, "Failed to discard files for a deleted photo", map[string]interface{}{
			"photoId": job.ImageId,
			"error":   err.Error(),
		})
	}
}

func (pw *PhotoWorker) updatePhotoStatus(imageId int, status int) error {
	if pw.db == nil {
		log.Printf("ERROR: Photo worker has no database reference")
		return fmt.Errorf("photo worker database not initialized")
	}

	var updateError error
	vbolt.WithWriteTx(pw.db, func(tx *vbolt.Tx) {
		image := GetImageById(tx, imageId)
		if image.Id == 0 {
			updateError = errPhotoRecordGone
			return
		}

		image.Status = status
		writeImage(tx, &image)

		if updateError == nil {
			vbolt.TxCommit(tx)
		}
	})

	if updateError != nil {
		log.Printf("Failed to update photo %d status to %d: %v", imageId, status, updateError)
	} else {
		log.Printf("Photo %d status updated to %d", imageId, status)
	}
	return updateError
}

func (pw *PhotoWorker) updatePhotoComplete(imageId int, width, height int) error {
	if pw.db == nil {
		log.Printf("ERROR: Photo worker has no database reference")
		return fmt.Errorf("photo worker database not initialized")
	}

	var updateError error
	vbolt.WithWriteTx(pw.db, func(tx *vbolt.Tx) {
		image := GetImageById(tx, imageId)
		if image.Id == 0 {
			updateError = errPhotoRecordGone
			return
		}

		log.Printf("Marking photo %d as complete (status: %d -> 0)", imageId, image.Status)
		image.Status = 0
		if width > 0 {
			image.Width = width
		}
		if height > 0 {
			image.Height = height
		}

		writeImage(tx, &image)

		if updateError == nil {
			vbolt.TxCommit(tx)
			log.Printf("Transaction committed for photo %d", imageId)
		}
	})

	if updateError != nil {
		log.Printf("Failed to mark photo %d as complete: %v", imageId, updateError)
	} else {
		log.Printf("Photo %d processing completed (dimensions: %dx%d)", imageId, width, height)
	}
	return updateError
}

type ProcessingStats struct {
	QueueLength     int            `json:"queueLength"`
	IsRunning       bool           `json:"isRunning"`
	Processed       int            `json:"processed"`
	Failed          int            `json:"failed"`
	LastProcessedAt time.Time      `json:"lastProcessedAt"`
	LastError       string         `json:"lastError"`
	LastErrorAt     time.Time      `json:"lastErrorAt"`
	RecentAttempts  []PhotoAttempt `json:"recentAttempts"`
}

func GetProcessingStats() ProcessingStats {
	if globalPhotoWorker == nil {
		return ProcessingStats{RecentAttempts: []PhotoAttempt{}}
	}

	pw := globalPhotoWorker
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()

	recent := make([]PhotoAttempt, 0, len(pw.recent))
	for i := len(pw.recent) - 1; i >= 0; i-- {
		recent = append(recent, pw.recent[i])
	}

	return ProcessingStats{
		QueueLength:     len(pw.jobQueue),
		IsRunning:       pw.isRunning(),
		Processed:       pw.processed,
		Failed:          pw.failed,
		LastProcessedAt: pw.lastProcessed,
		LastError:       pw.lastError,
		LastErrorAt:     pw.lastErrorAt,
		RecentAttempts:  recent,
	}
}

func StopPhotoWorker() {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
}

func stopPhotoWorkerAndDrain(ctx context.Context) bool {
	if globalPhotoWorker == nil {
		return true
	}
	return globalPhotoWorker.StopAndDrain(ctx)
}
