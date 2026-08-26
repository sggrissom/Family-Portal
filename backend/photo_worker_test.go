package backend

import (
	"bytes"
	"errors"
	"family/cfg"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"go.hasen.dev/vbolt"
)

func createTestImageData(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, image.Black)
			} else {
				img.Set(x, y, image.White)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestInitializePhotoWorker(t *testing.T) {
	globalPhotoWorker = nil

	testDBPath := "test_photo_worker_init.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	t.Run("Initialize new worker", func(t *testing.T) {
		InitializePhotoWorker(10, db)

		if globalPhotoWorker == nil {
			t.Error("Expected global photo worker to be initialized")
		}

		if !globalPhotoWorker.isRunning() {
			t.Error("Expected worker to be running after initialization")
		}

		if globalPhotoWorker.db != db {
			t.Error("Expected worker to have correct database reference")
		}

		if cap(globalPhotoWorker.jobQueue) != 10 {
			t.Errorf("Expected job queue capacity 10, got %d", cap(globalPhotoWorker.jobQueue))
		}
	})

	t.Run("Multiple initialization attempts", func(t *testing.T) {
		firstWorker := globalPhotoWorker

		InitializePhotoWorker(20, db)

		if globalPhotoWorker != firstWorker {
			t.Error("Expected worker not to be recreated on second initialization")
		}
	})

	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
		globalPhotoWorker = nil
	}
}

func TestQueuePhotoProcessing(t *testing.T) {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil

	testDBPath := "test_photo_worker_queue.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	t.Run("Queue without initialized worker", func(t *testing.T) {
		job := PhotoProcessingJob{
			ImageId:  1,
			FilePath: "test.jpg",
			FileData: []byte("test data"),
		}

		err := QueuePhotoProcessing(job)
		if err == nil {
			t.Error("Expected error when worker not initialized")
		}

		expectedError := "photo worker not initialized"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Queue with initialized worker", func(t *testing.T) {
		InitializePhotoWorker(5, db)
		defer func() {
			if globalPhotoWorker != nil {
				globalPhotoWorker.Stop()
				globalPhotoWorker = nil
			}
		}()

		job := PhotoProcessingJob{
			ImageId:  1,
			FilePath: "test.jpg",
			FileData: []byte("test data"),
		}

		err := QueuePhotoProcessing(job)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Queue full", func(t *testing.T) {
		InitializePhotoWorker(2, db)
		defer func() {
			if globalPhotoWorker != nil {
				globalPhotoWorker.Stop()
				globalPhotoWorker = nil
			}
		}()

		if globalPhotoWorker != nil {
			globalPhotoWorker.Stop()
		}

		for i := 1; i <= 2; i++ {
			job := PhotoProcessingJob{
				ImageId:  i,
				FilePath: "test.jpg",
				FileData: []byte("test data"),
			}
			err := QueuePhotoProcessing(job)
			if err != nil {
				t.Fatalf("Failed to queue job %d: %v", i, err)
			}
		}

		job := PhotoProcessingJob{
			ImageId:  3,
			FilePath: "test.jpg",
			FileData: []byte("test data"),
		}

		err := QueuePhotoProcessing(job)
		if err == nil {
			t.Error("Expected error when queue is full")
			return
		}

		expectedError := "processing queue is full"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
		globalPhotoWorker = nil
	}
}

func TestGetQueueLength(t *testing.T) {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil

	t.Run("No worker initialized", func(t *testing.T) {
		length := GetQueueLength()
		if length != 0 {
			t.Errorf("Expected queue length 0 when no worker, got %d", length)
		}
	})

	testDBPath := "test_queue_length.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	t.Run("Empty queue", func(t *testing.T) {
		InitializePhotoWorker(10, db)

		length := GetQueueLength()
		if length != 0 {
			t.Errorf("Expected queue length 0 for empty queue, got %d", length)
		}
	})

	t.Run("Queue with jobs", func(t *testing.T) {
		if globalPhotoWorker != nil {
			globalPhotoWorker.Stop()
		}

		globalPhotoWorker = &PhotoWorker{
			jobQueue: make(chan PhotoProcessingJob, 10),
			db:       db,
		}

		for i := 1; i <= 3; i++ {
			job := PhotoProcessingJob{
				ImageId:  i,
				FilePath: "test.jpg",
				FileData: []byte("test data"),
			}
			QueuePhotoProcessing(job)
		}

		length := GetQueueLength()
		if length != 3 {
			t.Errorf("Expected queue length 3, got %d", length)
		}
	})

	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
		globalPhotoWorker = nil
	}
}

func TestPhotoWorkerStartStop(t *testing.T) {
	testDBPath := "test_worker_start_stop.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	worker := &PhotoWorker{
		jobQueue: make(chan PhotoProcessingJob, 5),
		db:       db,
	}

	t.Run("Start worker", func(t *testing.T) {
		if worker.isRunning() {
			t.Error("Expected worker to not be running initially")
		}

		worker.Start()

		if !worker.isRunning() {
			t.Error("Expected worker to be running after start")
		}

		worker.Start()
		if !worker.isRunning() {
			t.Error("Expected worker to still be running after second start")
		}
	})

	t.Run("Stop worker", func(t *testing.T) {
		worker.Stop()

		if worker.isRunning() {
			t.Error("Expected worker to not be running after stop")
		}

		worker.Stop()
		if worker.isRunning() {
			t.Error("Expected worker to still be stopped after second stop")
		}
	})
}

func TestUpdatePhotoStatus(t *testing.T) {
	testDBPath := "test_update_status.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	worker := &PhotoWorker{db: db}

	testImage := Image{
		Id:       1,
		FamilyId: 1,
		Status:   0,
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, ImagesBkt, testImage.Id, &testImage)
		vbolt.TxCommit(tx)
	})

	t.Run("Update status successfully", func(t *testing.T) {
		err := worker.updatePhotoStatus(testImage.Id, 1)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			updated := GetImageById(tx, testImage.Id)
			if updated.Status != 1 {
				t.Errorf("Expected status 1, got %d", updated.Status)
			}
		})
	})

	t.Run("Update non-existent image", func(t *testing.T) {
		err := worker.updatePhotoStatus(999, 1)
		if err == nil {
			t.Fatal("Expected error for non-existent image")
		}

		if !errors.Is(err, errPhotoRecordGone) {
			t.Errorf("Expected errPhotoRecordGone, got '%v'", err)
		}
	})

	t.Run("Worker without database", func(t *testing.T) {
		workerNoDB := &PhotoWorker{db: nil}

		err := workerNoDB.updatePhotoStatus(1, 1)
		if err == nil {
			t.Error("Expected error when worker has no database")
		}

		expectedError := "photo worker database not initialized"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})
}

func TestUpdatePhotoComplete(t *testing.T) {
	testDBPath := "test_update_complete.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	worker := &PhotoWorker{db: db}

	testImage := Image{
		Id:       1,
		FamilyId: 1,
		Status:   1,
		Width:    0,
		Height:   0,
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, ImagesBkt, testImage.Id, &testImage)
		vbolt.TxCommit(tx)
	})

	t.Run("Mark as complete with dimensions", func(t *testing.T) {
		err := worker.updatePhotoComplete(testImage.Id, 800, 600)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			updated := GetImageById(tx, testImage.Id)
			if updated.Status != 0 {
				t.Errorf("Expected status 0, got %d", updated.Status)
			}
			if updated.Width != 800 {
				t.Errorf("Expected width 800, got %d", updated.Width)
			}
			if updated.Height != 600 {
				t.Errorf("Expected height 600, got %d", updated.Height)
			}
		})
	})

	t.Run("Mark as complete with zero dimensions", func(t *testing.T) {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			testImage.Status = 1
			testImage.Width = 100
			testImage.Height = 100
			vbolt.Write(tx, ImagesBkt, testImage.Id, &testImage)
			vbolt.TxCommit(tx)
		})

		err := worker.updatePhotoComplete(testImage.Id, 0, 0)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			updated := GetImageById(tx, testImage.Id)
			if updated.Status != 0 {
				t.Errorf("Expected status 0, got %d", updated.Status)
			}
			if updated.Width != 100 {
				t.Errorf("Expected width to remain 100, got %d", updated.Width)
			}
			if updated.Height != 100 {
				t.Errorf("Expected height to remain 100, got %d", updated.Height)
			}
		})
	})
}

func TestGetProcessingStats(t *testing.T) {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil

	t.Run("No worker initialized", func(t *testing.T) {
		stats := GetProcessingStats()

		if stats.QueueLength != 0 {
			t.Errorf("Expected queue length 0, got %d", stats.QueueLength)
		}
		if stats.IsRunning {
			t.Error("Expected IsRunning to be false")
		}
	})

	testDBPath := "test_processing_stats.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	t.Run("Worker initialized and running", func(t *testing.T) {
		InitializePhotoWorker(10, db)
		defer func() {
			if globalPhotoWorker != nil {
				globalPhotoWorker.Stop()
				globalPhotoWorker = nil
			}
		}()

		if globalPhotoWorker != nil {
			globalPhotoWorker.Stop()
		}

		for i := 1; i <= 3; i++ {
			job := PhotoProcessingJob{
				ImageId:  i,
				FilePath: "test.jpg",
				FileData: []byte("test data"),
			}
			QueuePhotoProcessing(job)
		}

		stats := GetProcessingStats()

		if stats.QueueLength != 3 {
			t.Errorf("Expected queue length 3, got %d", stats.QueueLength)
		}
		if stats.IsRunning {
			t.Error("Expected IsRunning to be false after stopping worker")
		}
	})

	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
		globalPhotoWorker = nil
	}
}

func TestStopPhotoWorker(t *testing.T) {
	if globalPhotoWorker != nil {
		globalPhotoWorker.Stop()
	}
	globalPhotoWorker = nil

	t.Run("Stop when no worker", func(t *testing.T) {
		StopPhotoWorker()
	})

	testDBPath := "test_stop_worker.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	t.Run("Stop running worker", func(t *testing.T) {
		InitializePhotoWorker(5, db)

		if !globalPhotoWorker.isRunning() {
			t.Error("Expected worker to be running before stop")
		}

		StopPhotoWorker()

		if globalPhotoWorker.isRunning() {
			t.Error("Expected worker to be stopped after StopPhotoWorker")
		}
	})
}

func TestSaveImageVariants(t *testing.T) {
	testDBPath := "test_save_variants.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	tempDir := filepath.Join(os.TempDir(), "test_photos")
	defer os.RemoveAll(tempDir)

	worker := &PhotoWorker{db: db}

	job := PhotoProcessingJob{
		ImageId:  1,
		FilePath: "test_image.jpg",
		FileData: createTestImageData(50, 50),
	}

	processedImages := map[string][]byte{
		"thumb_webp": createTestImageData(100, 100),
		"medium_jpg": createTestImageData(300, 300),
		"large_png":  createTestImageData(800, 800),
	}

	t.Run("Save variants successfully", func(t *testing.T) {
		err := worker.saveImageVariants(job, processedImages)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Invalid variant key format", func(t *testing.T) {
		invalidProcessedImages := map[string][]byte{
			"invalid_key_format_here": createTestImageData(100, 100),
			"thumb_webp":              createTestImageData(100, 100),
		}

		err := worker.saveImageVariants(job, invalidProcessedImages)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestPhotoProcessingJobStruct(t *testing.T) {
	job := PhotoProcessingJob{
		ImageId:        123,
		FilePath:       "/path/to/image.jpg",
		FileData:       []byte("test image data"),
		MimeType:       "image/jpeg",
		OriginalWidth:  800,
		OriginalHeight: 600,
	}

	if job.ImageId != 123 {
		t.Errorf("Expected ImageId 123, got %d", job.ImageId)
	}
	if job.FilePath != "/path/to/image.jpg" {
		t.Errorf("Expected FilePath '/path/to/image.jpg', got '%s'", job.FilePath)
	}
	if string(job.FileData) != "test image data" {
		t.Errorf("Expected FileData 'test image data', got '%s'", string(job.FileData))
	}
	if job.MimeType != "image/jpeg" {
		t.Errorf("Expected MimeType 'image/jpeg', got '%s'", job.MimeType)
	}
	if job.OriginalWidth != 800 {
		t.Errorf("Expected OriginalWidth 800, got %d", job.OriginalWidth)
	}
	if job.OriginalHeight != 600 {
		t.Errorf("Expected OriginalHeight 600, got %d", job.OriginalHeight)
	}
}

func TestProcessPhotoJobDiscardsWorkForADeletedPhoto(t *testing.T) {
	db := vbolt.Open(t.TempDir() + "/worker_deleted.db")
	vbolt.InitBuckets(db, &cfg.Info)
	t.Cleanup(func() { _ = db.Close() })

	worker := &PhotoWorker{db: db}

	photosDir := filepath.Join(cfg.StaticDir, "photos")
	if err := os.MkdirAll(photosDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	orphan := filepath.Join(photosDir, "gone_original.jpg")
	if err := os.WriteFile(orphan, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(orphan) })

	worker.processPhotoJob(PhotoProcessingJob{
		ImageId:  4242,
		FamilyId: 1,
		FilePath: "photos/gone.jpg",
		FileData: createTestImageData(64, 64),
		MimeType: "image/png",
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		if GetImageById(tx, 4242).Id != 0 {
			t.Error("the worker recreated a deleted photo record")
		}
	})
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("the worker left files on disk for a deleted photo")
	}
}
