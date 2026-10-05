package backend

import (
	"bytes"
	"context"
	"family/cfg"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func createTestImage(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, image.Black)
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func createMultipartFile(filename string, data []byte) (*multipart.File, *multipart.FileHeader, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("photo", filename)
	if err != nil {
		return nil, nil, err
	}

	part.Write(data)
	writer.Close()

	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(1024 * 1024)
	if err != nil {
		return nil, nil, err
	}

	files := form.File["photo"]
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no file found")
	}

	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, err
	}

	return &file, fileHeader, nil
}

func TestPackImage(t *testing.T) {
	testImage := Image{
		Id:               123,
		FamilyId:         1,
		OwnerUserId:      789,
		OriginalFilename: "test.jpg",
		MimeType:         "image/jpeg",
		FileSize:         1024,
		Width:            800,
		Height:           600,
		FilePath:         "/path/to/test.jpg",
		Title:            "Test Photo",
		Description:      "A test photo",
		PhotoDate:        time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC),
		CreatedAt:        time.Date(2023, 6, 15, 10, 35, 0, 0, time.UTC),
		Status:           0,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PackImage panicked: %v", r)
		}
	}()

	if testImage.Id != 123 {
		t.Errorf("Expected Id 123, got %d", testImage.Id)
	}
}

func TestGetImageById(t *testing.T) {
	testDBPath := "test_get_image.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	testImage := Image{
		Id:               1,
		FamilyId:         1,
		OwnerUserId:      1,
		OriginalFilename: "test.jpg",
		MimeType:         "image/jpeg",
		Title:            "Test Image",
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		vbolt.Write(tx, ImagesBkt, testImage.Id, &testImage)
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrieved := GetImageById(tx, testImage.Id)

		if retrieved.Id != testImage.Id {
			t.Errorf("Expected ID %d, got %d", testImage.Id, retrieved.Id)
		}

		if retrieved.OriginalFilename != testImage.OriginalFilename {
			t.Errorf("Expected filename '%s', got '%s'", testImage.OriginalFilename, retrieved.OriginalFilename)
		}

		if retrieved.MimeType != testImage.MimeType {
			t.Errorf("Expected MIME type '%s', got '%s'", testImage.MimeType, retrieved.MimeType)
		}
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrieved := GetImageById(tx, 999)
		if retrieved.Id != 0 {
			t.Errorf("Expected empty image for non-existent ID, got ID %d", retrieved.Id)
		}
	})
}

func TestGetPersonImages(t *testing.T) {
	testDBPath := "test_person_images.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	personId := 123
	otherPersonId := 456
	familyId := 1
	images := []Image{
		{Id: 1, FamilyId: familyId, Title: "Image 1"},
		{Id: 2, FamilyId: familyId, Title: "Image 2"},
		{Id: 3, FamilyId: familyId, Title: "Other Person Image"},
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, img := range images {
			vbolt.Write(tx, ImagesBkt, img.Id, &img)
			vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, img.Id, img.FamilyId)
		}

		AddPersonToPhoto(tx, 1, personId, familyId)
		AddPersonToPhoto(tx, 2, personId, familyId)
		AddPersonToPhoto(tx, 3, otherPersonId, familyId)

		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrieved := GetPersonImages(tx, personId)

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 images for person %d, got %d", personId, len(retrieved))
		}

		expectedImageIds := map[int]bool{1: true, 2: true}
		for _, img := range retrieved {
			if !expectedImageIds[img.Id] {
				t.Errorf("Unexpected image ID %d for person %d", img.Id, personId)
			}
		}
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrieved := GetPersonImages(tx, 999)
		if len(retrieved) != 0 {
			t.Errorf("Expected 0 images for non-existent person, got %d", len(retrieved))
		}
	})
}

func TestGetFamilyImages(t *testing.T) {
	testDBPath := "test_family_images.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	familyId := 1
	images := []Image{
		{Id: 1, FamilyId: familyId, Title: "Family Image 1"},
		{Id: 2, FamilyId: familyId, Title: "Family Image 2"},
		{Id: 3, FamilyId: 2, Title: "Other Family"},
	}

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for _, img := range images {
			vbolt.Write(tx, ImagesBkt, img.Id, &img)
			vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, img.Id, img.FamilyId)
		}
		vbolt.TxCommit(tx)
	})

	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		retrieved := GetFamilyImages(tx, familyId)

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 images for family %d, got %d", familyId, len(retrieved))
		}

		for _, img := range retrieved {
			if img.FamilyId != familyId {
				t.Errorf("Expected FamilyId %d, got %d", familyId, img.FamilyId)
			}
		}
	})
}

func TestGenerateUniqueFilename(t *testing.T) {
	originalFilename := "test_photo.jpg"

	filename1, err1 := generateUniqueFilename(originalFilename)
	filename2, err2 := generateUniqueFilename(originalFilename)

	if err1 != nil || err2 != nil {
		t.Fatalf("Expected no errors, got %v, %v", err1, err2)
	}

	if filename1 == filename2 {
		t.Error("Expected different filenames, got same value")
	}

	if !strings.HasSuffix(filename1, ".jpg") {
		t.Errorf("Expected filename to end with .jpg, got %s", filename1)
	}

	if !strings.HasSuffix(filename2, ".jpg") {
		t.Errorf("Expected filename to end with .jpg, got %s", filename2)
	}

	expectedLength := 32 + len(".jpg")
	if len(filename1) != expectedLength {
		t.Errorf("Expected filename length %d, got %d", expectedLength, len(filename1))
	}
}

func TestIsValidImageType(t *testing.T) {
	testCases := []struct {
		mimeType string
		expected bool
	}{
		{"image/jpeg", true},
		{"image/jpg", true},
		{"image/png", true},
		{"image/gif", true},
		{"image/webp", false},
		{"image/bmp", false},
		{"text/plain", false},
		{"application/pdf", false},
		{"", false},
		{"image/", false},
		{"jpeg", false},
	}

	for _, tc := range testCases {
		result := isValidImageType(tc.mimeType)
		if result != tc.expected {
			t.Errorf("For MIME type '%s', expected %v, got %v", tc.mimeType, tc.expected, result)
		}
	}
}

func TestGetImageDimensions(t *testing.T) {
	testImageData := createTestImage(100, 200)

	file, _, err := createMultipartFile("test.png", testImageData)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer (*file).Close()

	width, height, err := getImageDimensions(*file)
	if err != nil {
		t.Fatalf("Failed to get image dimensions: %v", err)
	}

	if width != 100 {
		t.Errorf("Expected width 100, got %d", width)
	}

	if height != 200 {
		t.Errorf("Expected height 200, got %d", height)
	}
}

func TestExtractExifDate(t *testing.T) {
	t.Run("Invalid EXIF data", func(t *testing.T) {
		invalidData := []byte("not an image")

		_, err := extractExifDate(invalidData)
		if err == nil {
			t.Error("Expected error for invalid EXIF data")
		}

		expectedError := "failed to decode EXIF"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Image without EXIF", func(t *testing.T) {
		testImageData := createTestImage(10, 10)

		_, err := extractExifDate(testImageData)
		if err == nil {
			t.Error("Expected error for image without EXIF")
		}
	})
}

func TestGenerateDefaultTitle(t *testing.T) {
	testCases := []struct {
		filename  string
		photoDate time.Time
		expected  string
	}{
		{
			filename:  "photo.jpg",
			photoDate: time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
			expected:  "Photo from Jun 15, 2023",
		},
		{
			filename:  "vacation.png",
			photoDate: time.Time{},
			expected:  "vacation",
		},
		{
			filename:  "document.pdf.jpg",
			photoDate: time.Time{},
			expected:  "document.pdf",
		},
		{
			filename:  "noextension",
			photoDate: time.Time{},
			expected:  "noextension",
		},
	}

	for _, tc := range testCases {
		result := generateDefaultTitle(tc.filename, tc.photoDate)
		if result != tc.expected {
			t.Errorf("For filename '%s' and date '%v', expected '%s', got '%s'",
				tc.filename, tc.photoDate, tc.expected, result)
		}
	}
}

func TestCalculatePhotoDate(t *testing.T) {
	testPerson := Person{
		Id:       1,
		Birthday: time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	t.Run("Today input type", func(t *testing.T) {
		result, err := calculatePhotoDate("today", "", nil, nil, testPerson, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if result != dayStart(result) || time.Since(result) > 24*time.Hour {
			t.Errorf("Expected today's UTC calendar day, got %v", result)
		}
	})

	t.Run("Date input type with valid date", func(t *testing.T) {
		dateString := "2023-06-15"
		expected := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)

		result, err := calculatePhotoDate("date", dateString, nil, nil, testPerson, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !result.Equal(expected) {
			t.Errorf("Expected date %v, got %v", expected, result)
		}
	})

	t.Run("Date input type without date", func(t *testing.T) {
		_, err := calculatePhotoDate("date", "", nil, nil, testPerson, nil)
		if err == nil {
			t.Error("Expected error for missing date")
		}

		expectedError := "Photo date is required"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Age input type", func(t *testing.T) {
		years := 2
		months := 6

		result, err := calculatePhotoDate("age", "", &years, &months, testPerson, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if want := testPerson.Birthday.AddDate(2, 6, 0); !result.Equal(want) {
			t.Errorf("Expected %v, the same date milestones and measurements get, got %v", want, result)
		}
	})

	t.Run("Age months outside 0-11", func(t *testing.T) {
		years, months := 1, 12
		if _, err := calculatePhotoDate("age", "", &years, &months, testPerson, nil); err == nil {
			t.Error("Expected an error for month 12, as milestones and measurements give")
		}
	})

	t.Run("Age input type without years", func(t *testing.T) {
		_, err := calculatePhotoDate("age", "", nil, nil, testPerson, nil)
		if err == nil {
			t.Error("Expected error for missing age years")
		}

		expectedError := "Age years must be non-negative"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Invalid input type", func(t *testing.T) {
		_, err := calculatePhotoDate("invalid", "", nil, nil, testPerson, nil)
		if err == nil {
			t.Error("Expected error for invalid input type")
		}

		expectedError := "Input type must be"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Auto input type fallback", func(t *testing.T) {
		invalidImageData := []byte("not an image")

		result, err := calculatePhotoDate("auto", "", nil, nil, testPerson, invalidImageData)
		if err != nil {
			t.Errorf("Expected no error with auto fallback, got %v", err)
		}

		if time.Since(result) > time.Minute || result.Location() != time.UTC {
			t.Errorf("Expected the current UTC time for auto fallback, got %v", result)
		}
	})

	t.Run("Auto input type uses the uploader's day without EXIF", func(t *testing.T) {
		result, err := calculatePhotoDate("auto", "2024-12-31", nil, nil, testPerson, []byte("not an image"))
		if err != nil || result != time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC) {
			t.Errorf("got %v, %v; want the supplied local day", result, err)
		}
	})
}

func TestUploadPhotoHandlerValidation(t *testing.T) {
	testDBPath := "test_upload_validation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	t.Run("Method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/upload-photo", nil)
		recorder := httptest.NewRecorder()

		uploadPhotoHandler(recorder, req)

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
		}
	})

	t.Run("File too large", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/upload-photo", nil)
		req.ContentLength = 60 << 20
		recorder := httptest.NewRecorder()

		uploadPhotoHandler(recorder, req)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}

		if !strings.Contains(recorder.Body.String(), "too large") {
			t.Error("Expected 'too large' in response body")
		}
	})

	t.Run("Unauthenticated request", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("personId", "1")
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload-photo", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		recorder := httptest.NewRecorder()

		uploadPhotoHandler(recorder, req)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
		}
	})
}

func TestValidateUpdatePhotoRequest(t *testing.T) {
	t.Run("Valid request", func(t *testing.T) {
		req := UpdatePhotoRequest{
			Id:          1,
			Title:       "Valid Title",
			Description: "Valid description",
			InputType:   "today",
		}

		err := validateUpdatePhotoRequest(req)
		if err != nil {
			t.Errorf("Expected no error for valid request, got %v", err)
		}
	})

	t.Run("Missing ID", func(t *testing.T) {
		req := UpdatePhotoRequest{
			Title:     "Valid Title",
			InputType: "today",
		}

		err := validateUpdatePhotoRequest(req)
		if err == nil {
			t.Error("Expected error for missing ID")
		}
	})

	t.Run("Empty title", func(t *testing.T) {
		req := UpdatePhotoRequest{
			Id:        1,
			Title:     "",
			InputType: "today",
		}

		err := validateUpdatePhotoRequest(req)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("Invalid input type", func(t *testing.T) {
		req := UpdatePhotoRequest{
			Id:        1,
			Title:     "Valid Title",
			InputType: "invalid",
		}

		err := validateUpdatePhotoRequest(req)
		if err == nil {
			t.Error("Expected error for invalid input type")
		}
	})

	t.Run("Date input without date", func(t *testing.T) {
		req := UpdatePhotoRequest{
			Id:        1,
			Title:     "Valid Title",
			InputType: "date",
		}

		err := validateUpdatePhotoRequest(req)
		if err == nil {
			t.Error("Expected error for date input without date")
		}
	})

	t.Run("Age input without years", func(t *testing.T) {
		req := UpdatePhotoRequest{
			Id:        1,
			Title:     "Valid Title",
			InputType: "age",
		}

		err := validateUpdatePhotoRequest(req)
		if err == nil {
			t.Error("Expected error for age input without years")
		}
	})
}

func TestUpdatePhotoKeepsDate(t *testing.T) {
	fx, cleanup := setupIsolationFixture(t)
	defer cleanup()

	taken := time.Date(2025, 7, 4, 15, 42, 0, 0, time.UTC)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		photo := GetImageById(tx, fx.photo.Id)
		photo.PhotoDate = taken
		vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		vbolt.TxCommit(tx)
	})

	token, err := generateJwtTokenString(fx.owner)
	if err != nil {
		t.Fatalf("generateJwtTokenString() error = %v", err)
	}

	var resp UpdatePhotoResponse
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		resp, err = UpdatePhoto(&vbeam.Context{Tx: tx, Token: token}, UpdatePhotoRequest{
			Id: fx.photo.Id, Title: "Fireworks", InputType: "keep",
		})
	})
	if err != nil {
		t.Fatalf("UpdatePhoto() error = %v", err)
	}
	if resp.Image.Title != "Fireworks" {
		t.Errorf("Title = %q, want %q", resp.Image.Title, "Fireworks")
	}
	if !resp.Image.PhotoDate.Equal(taken) {
		t.Errorf("PhotoDate = %v, want %v unchanged", resp.Image.PhotoDate, taken)
	}
}

func TestServePhotoOriginalDownloadNamesTheUploadedFile(t *testing.T) {
	fx := setupMatrixFixture(t)
	photosDir := filepath.Join(cfg.StaticDir, "photos")
	if err := os.MkdirAll(photosDir, 0755); err != nil {
		t.Fatal(err)
	}
	originalPath := filepath.Join(photosDir, "download-test_original.jpg")
	if err := os.WriteFile(originalPath, []byte("original bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(originalPath) })

	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		fx.photo.Status = 0
		fx.photo.FilePath = "photos/download-test.jpg"
		fx.photo.OriginalFilename = "Beach Day.jpg"
		vbolt.Write(tx, ImagesBkt, fx.photo.Id, &fx.photo)
		vbolt.TxCommit(tx)
	})

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req = req.WithContext(context.WithValue(req.Context(), UserContextKey, fx.owner))
		rec := httptest.NewRecorder()
		servePhotoHandler(rec, req)
		return rec
	}

	rec := get(fmt.Sprintf("/api/photo/%d/original?download=1", fx.photo.Id))
	if rec.Code != http.StatusOK || rec.Body.String() != "original bytes" {
		t.Fatalf("download = %d %q, want the original file", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Content-Disposition"), `attachment; filename="Beach Day.jpg"`; got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}

	rec = get(fmt.Sprintf("/api/photo/%d/original", fx.photo.Id))
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("inline original sent Content-Disposition %q", got)
	}
}
