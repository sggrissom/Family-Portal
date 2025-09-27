package backend

import (
	"family/cfg"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

// PhotoProcessingJob represents a photo that needs to be processed
type PhotoProcessingJob struct {
	ImageId        int
	FilePath       string
	FileData       []byte
	MimeType       string
	OriginalWidth  int
	OriginalHeight int
}

// PhotoWorker manages background photo processing
type PhotoWorker struct {
	jobQueue    chan PhotoProcessingJob
	stopChannel chan bool
	isRunning   bool
	db          *vbolt.DB
}

var globalPhotoWorker *PhotoWorker

// InitializePhotoWorker starts the background photo processing worker
func InitializePhotoWorker(queueSize int, db *vbolt.DB) {
	if globalPhotoWorker != nil {
		LogInfo(LogCategoryWorker, "Photo worker already initialized, skipping")
		return // Already initialized
	}

	LogInfo(LogCategoryWorker, "Initializing photo processing worker", map[string]interface{}{"queueSize": queueSize})
	globalPhotoWorker = &PhotoWorker{
		jobQueue:    make(chan PhotoProcessingJob, queueSize),
		stopChannel: make(chan bool),
		isRunning:   false,
		db:          db,
	}

	LogInfo(LogCategoryWorker, "Photo worker initialized with database reference")
	globalPhotoWorker.Start()
	LogInfo(LogCategoryWorker, "Photo processing worker started")
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
	if pw.isRunning {
		return
	}

	pw.isRunning = true
	go pw.processJobs()
	LogInfo(LogCategoryWorker, "Photo processing worker started")
}

// Stop gracefully shuts down the worker
func (pw *PhotoWorker) Stop() {
	if !pw.isRunning {
		return
	}

	pw.stopChannel <- true
	pw.isRunning = false
	LogInfo(LogCategoryWorker, "Photo processing worker stopped")
}

// GetQueueLength returns the current number of jobs in the queue
func GetQueueLength() int {
	if globalPhotoWorker == nil {
		return 0
	}
	return len(globalPhotoWorker.jobQueue)
}

// processJobs is the main worker loop that processes jobs from the queue
func (pw *PhotoWorker) processJobs() {
	for {
		select {
		case job := <-pw.jobQueue:
			pw.processPhotoJob(job)
		case <-pw.stopChannel:
			LogInfo(LogCategoryWorker, "Photo worker received stop signal")
			return
		}
	}
}

// processPhotoJob processes a single photo job
func (pw *PhotoWorker) processPhotoJob(job PhotoProcessingJob) {
	startTime := time.Now()
	log.Printf("[PHOTO_PROCESSING] Starting processing of photo ID %d (size: %d bytes)", job.ImageId, len(job.FileData))

	// Update status to processing in database
	log.Printf("[PHOTO_PROCESSING] Setting status to processing for photo %d", job.ImageId)
	err := pw.updatePhotoStatus(job.ImageId, 1) // 1 = processing
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to update photo %d status to processing: %v", job.ImageId, err)
		return
	}

	// Process the image and create multiple sizes/formats
	log.Printf("[PHOTO_PROCESSING] Processing image formats and sizes for photo %d", job.ImageId)
	processedImages, processedWidth, processedHeight, err := ProcessAndSaveMultipleSizes(job.FileData, job.MimeType)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to process photo ID %d: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
		return
	}
	log.Printf("[PHOTO_PROCESSING] Generated %d image variants for photo %d", len(processedImages), job.ImageId)

	// Save all image variants to disk
	log.Printf("[PHOTO_PROCESSING] Saving image variants to disk for photo %d", job.ImageId)
	err = pw.saveImageVariants(job, processedImages)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to save photo variants for ID %d: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
		return
	}
	log.Printf("[PHOTO_PROCESSING] Successfully saved all variants for photo %d", job.ImageId)

	// Update photo dimensions and mark as completed
	log.Printf("[PHOTO_PROCESSING] Marking photo %d as completed", job.ImageId)
	err = pw.updatePhotoComplete(job.ImageId, processedWidth, processedHeight)
	if err != nil {
		log.Printf("[PHOTO_PROCESSING] FAILED to mark photo %d as complete: %v", job.ImageId, err)
		pw.updatePhotoStatus(job.ImageId, 2) // 2 = failed/hidden
		return
	}

	processingTime := time.Since(startTime)
	log.Printf("[PHOTO_PROCESSING] ✅ Successfully completed photo ID %d in %v", job.ImageId, processingTime)
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
			updateError = fmt.Errorf("image not found")
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
			updateError = fmt.Errorf("image not found")
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

// ProcessingStats returns statistics about the photo processing worker
type ProcessingStats struct {
	QueueLength int  `json:"queueLength"`
	IsRunning   bool `json:"isRunning"`
}

// GetProcessingStats returns current processing statistics
func GetProcessingStats() ProcessingStats {
	if globalPhotoWorker == nil {
		return ProcessingStats{
			QueueLength: 0,
			IsRunning:   false,
		}
	}

	return ProcessingStats{
		QueueLength: len(globalPhotoWorker.jobQueue),
		IsRunning:   globalPhotoWorker.isRunning,
	}
}

// StopPhotoWorker gracefully shuts down the global photo worker
func StopPhotoWorker() {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
}
