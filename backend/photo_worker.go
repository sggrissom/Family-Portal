package backend

import (
	"context"
	"errors"
	"family/cfg"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.hasen.dev/vbolt"
)

// PhotoProcessingJob represents a photo that needs to be processed.
//
// An upload carries its bytes in FileData. A reprocess job does not: the
// admin panel can queue every unprocessed photo at once, and holding all of
// their originals in memory to do it would be worse than the long write
// transaction that shape replaced. Reprocess jobs set Reprocess and leave
// FileData nil, and the worker reads the original back off disk when it gets
// to them.
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

// maxRecentPhotoAttempts bounds the in-memory history the admin page reads.
const maxRecentPhotoAttempts = 20

// PhotoAttempt is one photo's trip through the worker. Photo processing failure
// is the most common real problem this site has and was the least visible one:
// the worker's only observable state was a queue length and a boolean, so a
// photo that failed left a Status = 2 row and a line in a log file nobody was
// reading.
type PhotoAttempt struct {
	Time    time.Time `json:"time"`
	ImageId int       `json:"imageId"`
	// Reprocess distinguishes a backlog job from a fresh upload, which fail for
	// different reasons — a missing original versus a bad decode.
	Reprocess bool `json:"reprocess"`
	Success   bool `json:"success"`
	// DurationMs is measured, unlike the "average process time" the analytics
	// page used to derive from file size with an arithmetic expression.
	DurationMs int `json:"durationMs"`
	// Reason is why it failed, empty on success.
	Reason string `json:"reason"`
}

// PhotoWorker manages background photo processing
type PhotoWorker struct {
	workerLifecycle
	jobQueue chan PhotoProcessingJob
	db       *vbolt.DB

	// statsMu guards everything below. Processing happens on the worker
	// goroutine while the admin page reads from request goroutines.
	statsMu       sync.Mutex
	processed     int
	failed        int
	lastProcessed time.Time
	lastError     string
	lastErrorAt   time.Time
	recent        []PhotoAttempt
}

// recordAttempt files one outcome into the counters and the bounded history.
// Same shape as the push worker's, which is the one page in the panel that
// answers the questions you actually arrive with.
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

// errPhotoRecordGone means the photo was deleted while this job was in flight —
// most often because the account that owned it was deleted. It is distinct from
// an ordinary failure because there is nothing to mark failed and nobody to tell:
// the correct response is to clean up whatever the job already wrote and stop.
var errPhotoRecordGone = errors.New("photo record no longer exists")

// InitializePhotoWorker starts the background photo processing worker
func InitializePhotoWorker(queueSize int, db *vbolt.DB) {
	if globalPhotoWorker != nil {
		LogInfo(LogCategoryWorker, "Photo worker already initialized, skipping")
		return // Already initialized
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

// QueuePhotoProcessingBlocking adds a photo to the processing queue, waiting
// for room rather than giving up when it is full. Only the admin reprocess path
// uses it, from its own goroutine: a backlog is routinely larger than the
// queue, and the non-blocking send would drop most of it on the floor.
func QueuePhotoProcessingBlocking(job PhotoProcessingJob) error {
	if globalPhotoWorker == nil {
		return fmt.Errorf("photo worker not initialized")
	}
	globalPhotoWorker.jobQueue <- job
	return nil
}

// QueuePhotoProcessing adds a photo to the processing queue
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
		log.Printf("Cannot queue photo %d: processing queue is full", job.ImageId)
		return fmt.Errorf("processing queue is full")
	}
}

// Start begins the background worker goroutine
func (pw *PhotoWorker) Start() {
	quit, done, ok := pw.start()
	if !ok {
		return
	}

	go pw.processJobs(quit, done)
	LogInfo(LogCategoryWorker, "Photo processing worker started")
}

// Stop signals the worker to exit and abandons whatever is still queued. The
// shutdown path calls StopAndDrain instead; this is for tests and for restarts,
// where finishing the backlog is not the point.
func (pw *PhotoWorker) Stop() {
	pw.stopImmediately()
	LogInfo(LogCategoryWorker, "Photo processing worker stopped")
}

// StopAndDrain stops the worker and finishes the photos already accepted,
// giving up when ctx expires. Draining matters most here of all the workers: a
// queued job holds the upload's bytes in memory and its database row already
// says "processing", so abandoning it strands a photo the user is watching for
// in a state nothing retries.
func (pw *PhotoWorker) StopAndDrain(ctx context.Context) bool {
	return pw.stopAndWait(ctx, true)
}

// GetQueueLength returns the current number of jobs in the queue
func GetQueueLength() int {
	if globalPhotoWorker == nil {
		return 0
	}
	return len(globalPhotoWorker.jobQueue)
}

// processJobs is the main worker loop that processes jobs from the queue
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

// processPhotoJob processes a single photo job
func (pw *PhotoWorker) processPhotoJob(job PhotoProcessingJob) {
	startTime := time.Now()
	log.Printf("[PHOTO_PROCESSING] Starting processing of photo ID %d (size: %d bytes)", job.ImageId, len(job.FileData))

	// Every exit below files an outcome, so the counters cannot drift from what
	// the worker actually did. A photo deleted mid-flight is neither: there is
	// nothing to mark and nobody to tell.
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

	// Update status to processing in database
	log.Printf("[PHOTO_PROCESSING] Setting status to processing for photo %d", job.ImageId)
	err := pw.updatePhotoStatus(job.ImageId, 1) // 1 = processing
	if err != nil {
		if errors.Is(err, errPhotoRecordGone) {
			// Deleted before we started. Nothing has been written yet, but the
			// upload's own original may already be on disk.
			pw.discardJobFiles(job)
			return
		}
		log.Printf("[PHOTO_PROCESSING] FAILED to update photo %d status to processing: %v", job.ImageId, err)
		outcome(false, "could not mark the photo as processing: "+err.Error())
		return
	}

	// A reprocess job arrives without its bytes; read the original back now,
	// one photo at a time, rather than having held it since it was queued.
	sourceData := job.FileData
	if len(sourceData) == 0 {
		sourceData, err = os.ReadFile(getOriginalPhotoPath(Image{FilePath: job.FilePath}))
		if err != nil {
			log.Printf("[PHOTO_PROCESSING] FAILED to read original for photo ID %d: %v", job.ImageId, err)
			pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
			outcome(false, "original file is missing: "+err.Error())
			return
		}
	}

	// Reprocessing writes over an existing set of variants. Formats the current
	// pipeline no longer emits would otherwise survive as stale files that
	// isPhotoProcessed keeps counting as a processed photo.
	if job.Reprocess {
		cleanupOldVariants(strings.TrimSuffix(filepath.Join(cfg.StaticDir, job.FilePath), filepath.Ext(job.FilePath)))
	}

	// Process the image and create multiple sizes/formats
	log.Printf("[PHOTO_PROCESSING] Processing image formats and sizes for photo %d", job.ImageId)
	processedImages, processedWidth, processedHeight, err := ProcessAndSaveMultipleSizes(sourceData, job.MimeType)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to process photo ID %d: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
		outcome(false, "could not decode or re-encode the image: "+err.Error())
		return
	}
	log.Printf("[PHOTO_PROCESSING] Generated %d image variants for photo %d", len(processedImages), job.ImageId)

	// Save all image variants to disk
	log.Printf("[PHOTO_PROCESSING] Saving image variants to disk for photo %d", job.ImageId)
	err = pw.saveImageVariants(job, processedImages)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to save photo variants for ID %d: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
		outcome(false, "could not write the variants to disk: "+err.Error())
		return
	}
	log.Printf("[PHOTO_PROCESSING] Successfully saved all variants for photo %d", job.ImageId)

	// Update photo dimensions and mark as completed
	log.Printf("[PHOTO_PROCESSING] Marking photo %d as completed", job.ImageId)
	err = pw.updatePhotoComplete(job.ImageId, processedWidth, processedHeight)
	if err != nil {
		if errors.Is(err, errPhotoRecordGone) {
			// The photo was deleted while we were decoding it. The variants
			// just written to disk have no record pointing at them, so nothing
			// will ever serve or clean them up but this.
			pw.discardJobFiles(job)
			return
		}
		log.Printf("[PHOTO_PROCESSING] FAILED to mark photo %d as complete: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
		outcome(false, "could not mark the photo complete: "+err.Error())
		return
	}

	// Queue face analysis for the processed photo
	QueuePhotoAnalysis(PhotoAnalysisJob{ImageId: job.ImageId, FamilyId: job.FamilyId})

	outcome(true, "")
	processingTime := time.Since(startTime)
	log.Printf("[PHOTO_PROCESSING] ✅ Successfully completed photo ID %d in %v", job.ImageId, processingTime)
}

// discardJobFiles removes every file this job could have written, for the case
// where the photo record disappeared underneath it. It reuses the deletion path
// a real photo takes, so the two cannot drift apart on which variants exist.
func (pw *PhotoWorker) discardJobFiles(job PhotoProcessingJob) {
	log.Printf("[PHOTO_PROCESSING] Photo %d was deleted mid-processing; discarding its files", job.ImageId)
	if err := deletePhotoFiles(Image{FilePath: job.FilePath}); err != nil {
		LogErrorSimple(LogCategoryWorker, "Failed to discard files for a deleted photo", map[string]interface{}{
			"photoId": job.ImageId,
			"error":   err.Error(),
		})
	}
}

// saveImageVariants saves all processed image variants to disk
func (pw *PhotoWorker) saveImageVariants(job PhotoProcessingJob, processedImages map[string][]byte) error {
	photosDir := filepath.Join(cfg.StaticDir, "photos")

	// Ensure photos directory exists
	err := os.MkdirAll(photosDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create photos directory: %w", err)
	}

	// Extract base filename without extension
	baseFilename := strings.TrimSuffix(filepath.Base(job.FilePath), filepath.Ext(job.FilePath))

	// Save each size and format variant
	for key, data := range processedImages {
		// Extract size and format from key (e.g., "thumb_webp" -> "thumb", "webp")
		parts := strings.Split(key, "_")
		if len(parts) != 2 {
			continue
		}
		sizeName, format := parts[0], parts[1]

		// Determine file extension
		var ext string
		switch format {
		case "webp":
			ext = ".webp"
		case "avif":
			ext = ".avif"
		case "png":
			ext = ".png"
		default:
			ext = ".jpg"
		}

		// Construct filename
		var fileName string
		if sizeName == "large" {
			fileName = baseFilename + ext // Main image without size suffix
		} else {
			fileName = baseFilename + "_" + sizeName + ext
		}

		// Write file
		filePath := filepath.Join(photosDir, fileName)
		if file, err := os.Create(filePath); err == nil {
			_, writeErr := file.Write(data)
			file.Close()
			if writeErr != nil {
				return fmt.Errorf("failed to write file %s: %w", filePath, writeErr)
			}
		} else {
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}
	}

	// Save original for backup (if we have the data)
	if len(job.FileData) > 0 {
		originalPath := filepath.Join(photosDir, baseFilename+"_original"+filepath.Ext(job.FilePath))
		if origFile, err := os.Create(originalPath); err == nil {
			origFile.Write(job.FileData)
			origFile.Close()
		}
	}

	return nil
}

// updatePhotoStatus updates the status of a photo in the database
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
		vbolt.Write(tx, ImagesBkt, image.Id, &image)

		// MUST commit the transaction to persist changes
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

// updatePhotoComplete marks a photo as completed and updates dimensions
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
		image.Status = 0 // 0 = active/completed
		if width > 0 {
			image.Width = width
		}
		if height > 0 {
			image.Height = height
		}

		vbolt.Write(tx, ImagesBkt, image.Id, &image)

		// MUST commit the transaction to persist changes
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

// ProcessingStats is a live snapshot of photo processing. Every counter is
// in-memory: a restart resets them and empties the history.
type ProcessingStats struct {
	QueueLength int  `json:"queueLength"`
	IsRunning   bool `json:"isRunning"`
	Processed   int  `json:"processed"`
	Failed      int  `json:"failed"`
	// LastProcessedAt is how you tell a quiet worker from a stalled one.
	LastProcessedAt time.Time      `json:"lastProcessedAt"`
	LastError       string         `json:"lastError"`
	LastErrorAt     time.Time      `json:"lastErrorAt"`
	RecentAttempts  []PhotoAttempt `json:"recentAttempts"`
}

// GetProcessingStats returns current processing statistics
func GetProcessingStats() ProcessingStats {
	if globalPhotoWorker == nil {
		return ProcessingStats{RecentAttempts: []PhotoAttempt{}}
	}

	pw := globalPhotoWorker
	pw.statsMu.Lock()
	defer pw.statsMu.Unlock()

	// Copy the history so callers cannot observe it being appended to, and
	// reverse it so the most recent attempt reads first.
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

// StopPhotoWorker gracefully shuts down the global photo worker
func StopPhotoWorker() {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
}

// stopPhotoWorkerAndDrain is the shutdown path's entry point. It reports
// whether the worker finished; false means photos were still queued when the
// budget ran out.
func stopPhotoWorkerAndDrain(ctx context.Context) bool {
	if globalPhotoWorker == nil {
		return true
	}
	return globalPhotoWorker.StopAndDrain(ctx)
}
