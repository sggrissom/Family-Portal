package backend

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/rwcarlsen/goexif/exif"
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPhotoMethods(app *vbeam.Application) {
	app.HandleFunc("/api/upload-photo", AuthMiddleware(uploadPhotoHandler))
	app.HandleFunc("/api/photo/", AuthMiddleware(servePhotoHandler))
	vbeam.RegisterProc(app, GetPhoto)
	vbeam.RegisterProc(app, UpdatePhoto)
	vbeam.RegisterProc(app, DeletePhoto)
	vbeam.RegisterProc(app, GetPhotoStatus)
	vbeam.RegisterProc(app, ListFamilyPhotos)
	vbeam.RegisterProc(app, AddPeopleToPhoto)
	vbeam.RegisterProc(app, RemovePersonFromPhotoProc)
}

// Request/Response types
type AddPhotoRequest struct {
	PersonIds   []int  `json:"personIds"` // Array of person IDs to tag in the photo
	Title       string `json:"title"`
	Description string `json:"description"`
	InputType   string `json:"inputType"` // 'today' | 'date' | 'age'
	PhotoDate   string `json:"photoDate,omitempty"`
	AgeYears    *int   `json:"ageYears,omitempty"`
	AgeMonths   *int   `json:"ageMonths,omitempty"`
}

type AddPhotoResponse struct {
	Image Image `json:"image"`
}

type GetPhotoRequest struct {
	Id int `json:"id"`
}

type GetPhotoResponse struct {
	Image  Image    `json:"image"`
	People []Person `json:"people"`
}

type UpdatePhotoRequest struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	InputType   string `json:"inputType"`
	PhotoDate   string `json:"photoDate,omitempty"`
	AgeYears    *int   `json:"ageYears,omitempty"`
	AgeMonths   *int   `json:"ageMonths,omitempty"`
}

type UpdatePhotoResponse struct {
	Image Image `json:"image"`
}

type DeletePhotoRequest struct {
	Id int `json:"id"`
}

type DeletePhotoResponse struct {
	Success bool `json:"success"`
}

type GetPhotoStatusRequest struct {
	Id int `json:"id"`
}

type GetPhotoStatusResponse struct {
	Status int `json:"status"` // 0 = active, 1 = processing, 2 = failed
}

type ListFamilyPhotosRequest struct {
	// No parameters needed - uses family from auth
}

type PhotoWithPeople struct {
	Image  Image    `json:"image"`
	People []Person `json:"people"`
}

type ListFamilyPhotosResponse struct {
	Photos []PhotoWithPeople `json:"photos"`
}

type AddPeopleToPhotoRequest struct {
	PhotoId   int   `json:"photoId"`
	PersonIds []int `json:"personIds"`
}

type AddPeopleToPhotoResponse struct {
	Success bool     `json:"success"`
	People  []Person `json:"people"`
}

type RemovePersonFromPhotoRequest struct {
	PhotoId  int `json:"photoId"`
	PersonId int `json:"personId"`
}

type RemovePersonFromPhotoResponse struct {
	Success bool `json:"success"`
}

// Database types
type Image struct {
	Id               int       `json:"id"`
	FamilyId         int       `json:"familyId"`
	OwnerUserId      int       `json:"ownerUserId"`
	OriginalFilename string    `json:"originalFilename"`
	MimeType         string    `json:"mimeType"`
	FileSize         int       `json:"fileSize"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	FilePath         string    `json:"filePath"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	PhotoDate        time.Time `json:"photoDate"`
	CreatedAt        time.Time `json:"createdAt"`
	Status           int       `json:"status"` // 0 = active, 1 = processing, 2 = hidden
}

// PhotoPerson represents the many-to-many relationship between photos and people
type PhotoPerson struct {
	Id        int       `json:"id"`
	PhotoId   int       `json:"photoId"`
	PersonId  int       `json:"personId"`
	FamilyId  int       `json:"familyId"`
	CreatedAt time.Time `json:"createdAt"`
}

// Packing function for vbolt serialization
func PackImage(self *Image, buf *vpack.Buffer) {
	vpack.Version(2, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.OwnerUserId, buf)
	vpack.String(&self.OriginalFilename, buf)
	vpack.String(&self.MimeType, buf)
	vpack.Int(&self.FileSize, buf)
	vpack.Int(&self.Width, buf)
	vpack.Int(&self.Height, buf)
	vpack.String(&self.FilePath, buf)
	vpack.String(&self.Title, buf)
	vpack.String(&self.Description, buf)
	vpack.Time(&self.PhotoDate, buf)
	vpack.Time(&self.CreatedAt, buf)
	vpack.Int(&self.Status, buf)
}

// Packing function for PhotoPerson
func PackPhotoPerson(self *PhotoPerson, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.PhotoId, buf)
	vpack.Int(&self.PersonId, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Time(&self.CreatedAt, buf)
}

// Buckets for vbolt database storage
var ImagesBkt = vbolt.Bucket(&cfg.Info, "images", vpack.FInt, PackImage)
var PhotoPersonBkt = vbolt.Bucket(&cfg.Info, "photo_person", vpack.FInt, PackPhotoPerson)

// ImageByFamilyIndex: term = family_id, target = image_id
var ImageByFamilyIndex = vbolt.Index(&cfg.Info, "image_by_family", vpack.FInt, vpack.FInt)

// PhotoPersonByPhotoIndex: term = photo_id, target = photo_person_id
var PhotoPersonByPhotoIndex = vbolt.Index(&cfg.Info, "photo_person_by_photo", vpack.FInt, vpack.FInt)

// PhotoPersonByPersonIndex: term = person_id, target = photo_person_id
var PhotoPersonByPersonIndex = vbolt.Index(&cfg.Info, "photo_person_by_person", vpack.FInt, vpack.FInt)

// PhotoPersonByFamilyIndex: term = family_id, target = photo_person_id
var PhotoPersonByFamilyIndex = vbolt.Index(&cfg.Info, "photo_person_by_family", vpack.FInt, vpack.FInt)

// Database helper functions
func GetImageById(tx *vbolt.Tx, imageId int) (image Image) {
	vbolt.Read(tx, ImagesBkt, imageId, &image)
	return
}

func GetPhotoPersonById(tx *vbolt.Tx, photoPersonId int) (photoPerson PhotoPerson) {
	vbolt.Read(tx, PhotoPersonBkt, photoPersonId, &photoPerson)
	return
}

// Get all PhotoPerson records for a specific photo
func GetPhotoPersonsByPhoto(tx *vbolt.Tx, photoId int) (photoPersons []PhotoPerson) {
	var photoPersonIds []int
	vbolt.ReadTermTargets(tx, PhotoPersonByPhotoIndex, photoId, &photoPersonIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoPersonBkt, photoPersonIds, &photoPersons)
	return
}

// Get all PhotoPerson records for a specific person
func GetPhotoPersonsByPerson(tx *vbolt.Tx, personId int) (photoPersons []PhotoPerson) {
	var photoPersonIds []int
	vbolt.ReadTermTargets(tx, PhotoPersonByPersonIndex, personId, &photoPersonIds, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoPersonBkt, photoPersonIds, &photoPersons)
	return
}

// Get all people associated with a photo
func GetPhotoPeople(tx *vbolt.Tx, photoId int) (people []Person) {
	photoPersons := GetPhotoPersonsByPhoto(tx, photoId)
	people = make([]Person, 0, len(photoPersons))

	for _, photoPerson := range photoPersons {
		person := GetPersonById(tx, photoPerson.PersonId)
		if person.Id != 0 {
			people = append(people, person)
		}
	}
	return
}

// Get all images for a specific person
func GetPersonImages(tx *vbolt.Tx, personId int) (images []Image) {
	photoPersons := GetPhotoPersonsByPerson(tx, personId)
	images = make([]Image, 0, len(photoPersons))

	for _, photoPerson := range photoPersons {
		image := GetImageById(tx, photoPerson.PhotoId)
		if image.Id != 0 {
			images = append(images, image)
		}
	}
	return
}

// Get all images for a family
func GetFamilyImages(tx *vbolt.Tx, familyId int) (images []Image) {
	var imageIds []int
	vbolt.ReadTermTargets(tx, ImageByFamilyIndex, familyId, &imageIds, vbolt.Window{})
	vbolt.ReadSlice(tx, ImagesBkt, imageIds, &images)
	return
}

// Add a person to a photo
func AddPersonToPhoto(tx *vbolt.Tx, photoId int, personId int, familyId int) (photoPersonId int) {
	photoPerson := PhotoPerson{
		Id:        vbolt.NextIntId(tx, PhotoPersonBkt),
		PhotoId:   photoId,
		PersonId:  personId,
		FamilyId:  familyId,
		CreatedAt: time.Now(),
	}

	vbolt.Write(tx, PhotoPersonBkt, photoPerson.Id, &photoPerson)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, photoId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, personId)
	vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, familyId)

	return photoPerson.Id
}

// Remove a person from a photo
func RemovePersonFromPhoto(tx *vbolt.Tx, photoId int, personId int) {
	photoPersons := GetPhotoPersonsByPhoto(tx, photoId)

	for _, photoPerson := range photoPersons {
		if photoPerson.PersonId == personId {
			vbolt.Delete(tx, PhotoPersonBkt, photoPerson.Id)
			vbolt.SetTargetSingleTerm(tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
			vbolt.SetTargetSingleTerm(tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
			break
		}
	}
}

// Generate unique filename
func generateUniqueFilename(originalFilename string) (string, error) {
	ext := filepath.Ext(originalFilename)
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	filename := hex.EncodeToString(bytes) + ext
	return filename, nil
}

// Validate image file type
func isValidImageType(mimeType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
	}
	for _, validType := range validTypes {
		if mimeType == validType {
			return true
		}
	}
	return false
}

// Get image dimensions
func getImageDimensions(file multipart.File) (int, int, error) {
	// Reset file position
	file.Seek(0, 0)

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}

	// Reset file position for later use
	file.Seek(0, 0)

	return config.Width, config.Height, nil
}

// Extract date from EXIF metadata
func extractExifDate(fileData []byte) (time.Time, error) {
	reader := bytes.NewReader(fileData)

	// Decode EXIF data
	x, err := exif.Decode(reader)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to decode EXIF: %w", err)
	}

	// Try to get DateTime tag (when photo was taken)
	tm, err := x.DateTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("no DateTime found in EXIF: %w", err)
	}

	return tm, nil
}

// Generate default title if none provided
func generateDefaultTitle(originalFilename string, photoDate time.Time) string {
	if !photoDate.IsZero() {
		return fmt.Sprintf("Photo from %s", photoDate.Format("Jan 2, 2006"))
	}
	// Fall back to filename without extension
	return strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
}

// Calculate photo date based on input type
func calculatePhotoDate(inputType string, photoDate string, ageYears *int, ageMonths *int, person Person, fileData []byte) (time.Time, error) {
	switch inputType {
	case "auto":
		// Try to extract from EXIF first
		if exifDate, err := extractExifDate(fileData); err == nil {
			return exifDate, nil
		}
		// Fall back to today if EXIF extraction fails
		return time.Now(), nil
	case "today":
		return time.Now(), nil
	case "date":
		if photoDate == "" {
			return time.Time{}, errors.New("photo date is required")
		}
		return time.Parse("2006-01-02", photoDate)
	case "age":
		if ageYears == nil {
			return time.Time{}, errors.New("age years is required")
		}

		months := 0
		if ageMonths != nil {
			months = *ageMonths
		}

		// Calculate date when person was this age
		targetAge := time.Duration(*ageYears)*365*24*time.Hour + time.Duration(months)*30*24*time.Hour
		photoDateTime := person.Birthday.Add(targetAge)

		return photoDateTime, nil
	default:
		return time.Time{}, errors.New("invalid input type")
	}
}

// Upload photo handler
func uploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max to account for larger images)
	// Validate file size limit (50MB max)
	const maxFileSize = 50 << 20 // 50MB
	if r.ContentLength > maxFileSize {
		RespondFileTooLargeError(w, r, "50MB")
		return
	}

	err := r.ParseMultipartForm(32 << 20) // 32MB in memory, rest on disk
	if err != nil {
		RespondValidationError(w, r, "Failed to parse form", err.Error())
		return
	}

	// Get authenticated user from request context
	user, ok := GetUserFromContext(r)
	if !ok {
		RespondAuthError(w, r, "Authentication required")
		return
	}

	// Log photo upload start
	LogInfoWithRequest(r, LogCategoryPhoto, "Photo upload started", map[string]interface{}{
		"userId": user.Id,
	})

	// Parse form data for person IDs (can be multiple)
	personIdsStr := r.FormValue("personIds")
	var personIds []int

	if personIdsStr != "" {
		// Parse JSON array of person IDs
		if err := json.Unmarshal([]byte(personIdsStr), &personIds); err != nil {
			RespondValidationError(w, r, "Invalid person IDs format", err.Error())
			return
		}
	}

	title := strings.TrimSpace(r.FormValue("title"))

	description := strings.TrimSpace(r.FormValue("description"))
	inputType := r.FormValue("inputType")
	photoDate := r.FormValue("photoDate")

	var ageYears, ageMonths *int
	if ageYearsStr := r.FormValue("ageYears"); ageYearsStr != "" {
		if years, err := strconv.Atoi(ageYearsStr); err == nil {
			ageYears = &years
		}
	}
	if ageMonthsStr := r.FormValue("ageMonths"); ageMonthsStr != "" {
		if months, err := strconv.Atoi(ageMonthsStr); err == nil {
			ageMonths = &months
		}
	}

	// Get the uploaded file
	file, fileHeader, err := r.FormFile("photo")
	if err != nil {
		RespondValidationError(w, r, "Photo file is required", err.Error())
		return
	}
	defer file.Close()

	// Validate file size (32MB max for original upload)
	if fileHeader.Size > 32<<20 { // 32MB
		RespondFileTooLargeError(w, r, "32MB")
		return
	}

	// Validate file type
	mimeType := fileHeader.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		RespondInvalidFileTypeError(w, r, "JPEG, PNG, GIF")
		return
	}

	// Get image dimensions
	width, height, err := getImageDimensions(file)
	if err != nil {
		RespondValidationError(w, r, "Failed to read image dimensions", err.Error())
		return
	}

	// Database operations
	var image Image
	var validPersons []Person

	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		// Validate all person IDs exist and belong to user's family
		if len(personIds) > 0 {
			validPersons = make([]Person, 0, len(personIds))
			for _, personId := range personIds {
				person := GetPersonById(tx, personId)
				if person.Id == 0 || person.FamilyId != user.FamilyId {
					// Don't commit transaction for invalid person
					return
				}
				validPersons = append(validPersons, person)
			}
		}

		// Generate unique filename
		uniqueFilename, err := generateUniqueFilename(fileHeader.Filename)
		if err != nil {
			// Don't commit transaction if filename generation fails
			return
		}

		// Ensure photos directory exists
		photosDir := filepath.Join(cfg.StaticDir, "photos")
		err = os.MkdirAll(photosDir, 0755)
		if err != nil {
			// Don't commit transaction if directory creation fails
			return
		}

		// Read file into memory for processing
		file.Seek(0, 0)
		fileData, err := io.ReadAll(file)
		if err != nil {
			// Don't commit transaction if file read fails
			return
		}

		// Calculate photo date (now that we have fileData for EXIF)
		// Use first person for age-based calculation, or empty person for non-age calculations
		var referencePerson Person
		if len(validPersons) > 0 {
			referencePerson = validPersons[0]
		}
		calculatedPhotoDate, err := calculatePhotoDate(inputType, photoDate, ageYears, ageMonths, referencePerson, fileData)
		if err != nil {
			// Don't commit transaction for invalid date
			return
		}

		// Generate title if not provided
		if title == "" {
			title = generateDefaultTitle(fileHeader.Filename, calculatedPhotoDate)
		}

		// Save original file for background processing
		baseFilename := strings.TrimSuffix(uniqueFilename, filepath.Ext(uniqueFilename))
		originalPath := filepath.Join(photosDir, baseFilename+"_original"+filepath.Ext(uniqueFilename))
		if origFile, err := os.Create(originalPath); err != nil {
			// Don't commit transaction if file save fails
			return
		} else {
			origFile.Write(fileData)
			origFile.Close()
		}

		// Create image record
		image = Image{
			Id:               vbolt.NextIntId(tx, ImagesBkt),
			FamilyId:         user.FamilyId,
			OwnerUserId:      user.Id,
			OriginalFilename: fileHeader.Filename,
			MimeType:         mimeType,
			FileSize:         int(fileHeader.Size), // Original file size for now
			Width:            width,
			Height:           height,
			FilePath:         fmt.Sprintf("photos/%s", uniqueFilename),
			Title:            title,
			Description:      description,
			PhotoDate:        calculatedPhotoDate,
			CreatedAt:        time.Now(),
			Status:           1, // Processing
		}

		// Save image to database
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, user.FamilyId)

		// Create PhotoPerson relationships for each tagged person
		for _, person := range validPersons {
			AddPersonToPhoto(tx, image.Id, person.Id, user.FamilyId)
		}

		vbolt.TxCommit(tx)
	})

	// Check if image was created (transaction succeeded)
	if image.Id == 0 {
		http.Error(w, "Failed to upload photo", http.StatusInternalServerError)
		return
	}

	// Queue photo for background processing
	// Reset file reader to get file data for processing
	file.Seek(0, 0)
	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read file for processing queue: %v", err)
		// Don't fail the upload, just log the error
	} else {
		// Create processing job
		job := PhotoProcessingJob{
			ImageId:        image.Id,
			FilePath:       image.FilePath,
			FileData:       fileData,
			MimeType:       mimeType,
			OriginalWidth:  width,
			OriginalHeight: height,
		}

		// Queue the job for processing
		if err := QueuePhotoProcessing(job); err != nil {
			log.Printf("Failed to queue photo %d for processing: %v", image.Id, err)
			// Could set status to failed here, but let's keep it as processing for now
		}
	}

	// Log successful photo upload
	LogInfoWithRequest(r, LogCategoryPhoto, "Photo upload completed", map[string]interface{}{
		"userId":      user.Id,
		"photoId":     image.Id,
		"personIds":   personIds,
		"peopleCount": len(personIds),
		"fileSize":    fileHeader.Size,
		"mimeType":    mimeType,
		"filename":    fileHeader.Filename,
	})

	// Return success response
	response := AddPhotoResponse{
		Image: image,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Serve photo handler
func servePhotoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract image ID from URL path (e.g., /api/photo/123 -> 123, /api/photo/123/thumb -> 123 + thumb)
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/photo/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	imageId, err := strconv.Atoi(pathParts[0])
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Check for size variant (thumb, medium, large)
	sizeVariant := ""
	if len(pathParts) > 1 {
		sizeVariant = pathParts[1]
	}

	// Get authenticated user from request context
	user, ok := GetUserFromContext(r)
	if !ok {
		RespondNotFoundError(w, r, "Not found") // Return 404 to avoid leaking photo existence
		return
	}

	// Get image from database
	var image Image
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		image = GetImageById(tx, imageId)
	})

	// Check if image exists and user has access
	if image.Id == 0 || image.FamilyId != user.FamilyId {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// If photo is still processing, serve placeholder
	if image.Status == 1 {
		serveProcessingPlaceholder(w, r)
		return
	}

	// If photo failed processing, serve error placeholder or 404
	if image.Status == 2 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Determine optimal format based on browser support
	acceptHeader := r.Header.Get("Accept")
	optimalFormat := GetOptimalImageFormat(acceptHeader)

	// Validate size variant
	validSizes := map[string]bool{
		"small": true, "thumb": true, "medium": true,
		"large": true, "xlarge": true, "xxlarge": true, "original": true,
	}

	if sizeVariant != "" && !validSizes[sizeVariant] {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Default to large if no size specified
	if sizeVariant == "" {
		sizeVariant = "large"
	}

	// Construct file path based on size and optimal format
	basePath := filepath.Join(cfg.StaticDir, image.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	var fullPath string
	var contentType string

	// Handle original size (serve as-is)
	if sizeVariant == "original" {
		fullPath = baseFilename + "_original" + filepath.Ext(basePath)
		contentType = image.MimeType
	} else {
		// Try to find the best format variant
		for _, format := range []string{optimalFormat, "webp", "jpeg"} {
			var ext string
			switch format {
			case "webp":
				ext = ".webp"
				contentType = "image/webp"
			case "avif":
				ext = ".avif"
				contentType = "image/avif"
			default:
				ext = ".jpg"
				contentType = "image/jpeg"
			}

			if sizeVariant == "large" {
				fullPath = baseFilename + ext
			} else {
				fullPath = baseFilename + "_" + sizeVariant + ext
			}

			// Check if this variant exists
			if _, err := os.Stat(fullPath); err == nil {
				break
			}
		}
	}

	// Validate that the file path doesn't contain directory traversal
	cleanPath := filepath.Clean(fullPath)
	staticDir := filepath.Clean(cfg.StaticDir)
	if !strings.HasPrefix(cleanPath, staticDir) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Check if file exists, fall back to JPEG large if variant doesn't exist
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if sizeVariant != "original" {
			// Fall back to JPEG large image
			fallbackPath := baseFilename + ".jpg"
			if _, err := os.Stat(fallbackPath); err == nil {
				fullPath = fallbackPath
				contentType = "image/jpeg"
			} else {
				// Ultimate fallback to original file
				originalPath := baseFilename + "_original" + filepath.Ext(basePath)
				if _, err := os.Stat(originalPath); err == nil {
					fullPath = originalPath
					contentType = image.MimeType
				} else {
					http.Error(w, "Not found", http.StatusNotFound)
					return
				}
			}
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}

	// Set content type based on determined optimal format
	w.Header().Set("Content-Type", contentType)

	// Enhanced cache headers for better performance (1 year for images with versioning)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", fmt.Sprintf("\"%d-%s-%d-%d\"", image.Id, sizeVariant, image.CreatedAt.Unix(), image.Status))

	// Add Vary header for content negotiation
	w.Header().Set("Vary", "Accept")

	// Serve the file
	http.ServeFile(w, r, fullPath)
}

// vbeam procedures for photo operations

// GetPhoto retrieves a photo by ID with family access control
func GetPhoto(ctx *vbeam.Context, req GetPhotoRequest) (resp GetPhotoResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get photo from database
	photo := GetImageById(ctx.Tx, req.Id)

	// Check if photo exists and user has access (same family)
	if photo.Id == 0 || photo.FamilyId != user.FamilyId {
		err = errors.New("Photo not found or access denied")
		return
	}

	// Get all people associated with this photo
	people := GetPhotoPeople(ctx.Tx, photo.Id)

	// Calculate age for all people
	for i := range people {
		people[i].Age = calculateAge(people[i].Birthday)
	}

	resp.Image = photo
	resp.People = people
	return
}

// UpdatePhoto updates photo metadata (title, description, date)
func UpdatePhoto(ctx *vbeam.Context, req UpdatePhotoRequest) (resp UpdatePhotoResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if err = validateUpdatePhotoRequest(req); err != nil {
		return
	}

	vbeam.UseWriteTx(ctx)

	// Get existing photo
	photo := GetImageById(ctx.Tx, req.Id)
	if photo.Id == 0 || photo.FamilyId != user.FamilyId {
		err = errors.New("Photo not found or access denied")
		return
	}

	// Get people for date calculation (use first person if available)
	people := GetPhotoPeople(ctx.Tx, photo.Id)
	var referencePerson Person
	if len(people) > 0 {
		referencePerson = people[0]
	}

	// Calculate new photo date
	calculatedPhotoDate, err := calculatePhotoDate(req.InputType, req.PhotoDate, req.AgeYears, req.AgeMonths, referencePerson, nil)
	if err != nil {
		return
	}

	// Update photo fields
	photo.Title = strings.TrimSpace(req.Title)
	photo.Description = strings.TrimSpace(req.Description)
	photo.PhotoDate = calculatedPhotoDate

	// Generate title if empty
	if photo.Title == "" {
		photo.Title = generateDefaultTitle(photo.OriginalFilename, calculatedPhotoDate)
	}

	// Save updated photo
	vbolt.Write(ctx.Tx, ImagesBkt, photo.Id, &photo)
	vbolt.TxCommit(ctx.Tx)

	resp.Image = photo
	return
}

// DeletePhoto removes a photo and all associated files
func DeletePhoto(ctx *vbeam.Context, req DeletePhotoRequest) (resp DeletePhotoResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	vbeam.UseWriteTx(ctx)

	// Get photo to delete
	photo := GetImageById(ctx.Tx, req.Id)
	if photo.Id == 0 || photo.FamilyId != user.FamilyId {
		err = errors.New("Photo not found or access denied")
		return
	}

	// Delete photo files from disk
	err = deletePhotoFiles(photo)
	if err != nil {
		// Log error but continue with database cleanup
		fmt.Printf("Warning: Failed to delete photo files for ID %d: %v\n", photo.Id, err)
	}

	// Remove PhotoPerson relationships
	photoPersons := GetPhotoPersonsByPhoto(ctx.Tx, photo.Id)
	for _, photoPerson := range photoPersons {
		vbolt.Delete(ctx.Tx, PhotoPersonBkt, photoPerson.Id)
		vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPhotoIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByPersonIndex, photoPerson.Id, -1)
		vbolt.SetTargetSingleTerm(ctx.Tx, PhotoPersonByFamilyIndex, photoPerson.Id, -1)
	}

	// Remove from database
	vbolt.Delete(ctx.Tx, ImagesBkt, photo.Id)

	// Remove from indexes
	vbolt.SetTargetSingleTerm(ctx.Tx, ImageByFamilyIndex, photo.Id, -1)

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}

// Helper function to delete all photo file variants
func deletePhotoFiles(photo Image) error {
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	base := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	// All size variants
	sizes := []string{"small", "thumb", "medium", "large", "xlarge", "xxlarge"}
	// All formats
	formats := []string{"jpg", "webp", "avif", "png"}

	var filesToDelete []string

	// Add all size/format combinations
	for _, size := range sizes {
		for _, format := range formats {
			var fileName string
			if size == "large" {
				fileName = base + "." + format
			} else {
				fileName = base + "_" + size + "." + format
			}
			filesToDelete = append(filesToDelete, fileName)
		}
	}

	// Add original backup file
	filesToDelete = append(filesToDelete, base+"_original"+filepath.Ext(basePath))

	var lastError error
	for _, filePath := range filesToDelete {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			lastError = err // Keep track of last error but continue
		}
	}

	return lastError
}

// Validation for update request
func validateUpdatePhotoRequest(req UpdatePhotoRequest) error {
	if req.Id <= 0 {
		return errors.New("Invalid photo ID")
	}

	if req.InputType == "" {
		return errors.New("Input type is required")
	}

	validInputTypes := []string{"auto", "today", "date", "age"}
	isValid := false
	for _, validType := range validInputTypes {
		if req.InputType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return errors.New("Invalid input type")
	}

	if req.InputType == "date" && req.PhotoDate == "" {
		return errors.New("Photo date is required when input type is 'date'")
	}

	if req.InputType == "age" && req.AgeYears == nil {
		return errors.New("Age years is required when input type is 'age'")
	}

	return nil
}

// serveProcessingPlaceholder serves a placeholder image for photos still being processed
func serveProcessingPlaceholder(w http.ResponseWriter, r *http.Request) {
	// Generate a simple SVG placeholder
	svgContent := `<svg width="400" height="300" xmlns="http://www.w3.org/2000/svg">
		<rect width="100%" height="100%" fill="#f0f0f0"/>
		<circle cx="200" cy="120" r="30" fill="#d0d0d0">
			<animateTransform attributeName="transform" attributeType="XML" type="rotate"
				from="0 200 120" to="360 200 120" dur="1s" repeatCount="indefinite"/>
		</circle>
		<text x="200" y="180" font-family="Arial, sans-serif" font-size="16" text-anchor="middle" fill="#666">
			Processing image...
		</text>
	</svg>`

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	w.Write([]byte(svgContent))
}

// GetPhotoStatus returns the processing status of a photo
func GetPhotoStatus(ctx *vbeam.Context, req GetPhotoStatusRequest) (resp GetPhotoStatusResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get photo from database
	photo := GetImageById(ctx.Tx, req.Id)

	// Check if photo exists and user has access (same family)
	if photo.Id == 0 || photo.FamilyId != user.FamilyId {
		err = ErrAuthFailure
		return
	}

	resp.Status = photo.Status
	return
}

func ListFamilyPhotos(ctx *vbeam.Context, req ListFamilyPhotosRequest) (resp ListFamilyPhotosResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Get all family photos
	images := GetFamilyImages(ctx.Tx, user.FamilyId)

	// Filter out failed photos and create PhotoWithPeople structs
	resp.Photos = make([]PhotoWithPeople, 0, len(images))

	for _, image := range images {
		// Skip failed photos (status=2)
		if image.Status == 2 {
			continue
		}

		// Get all people associated with this photo
		people := GetPhotoPeople(ctx.Tx, image.Id)

		// Calculate age for all people
		for i := range people {
			people[i].Age = calculateAge(people[i].Birthday)
		}

		// Add to response (photos without people are still included)
		resp.Photos = append(resp.Photos, PhotoWithPeople{
			Image:  image,
			People: people,
		})
	}

	return
}

// AddPeopleToPhoto adds multiple people to an existing photo
func AddPeopleToPhoto(ctx *vbeam.Context, req AddPeopleToPhotoRequest) (resp AddPeopleToPhotoResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if req.PhotoId <= 0 {
		err = errors.New("Invalid photo ID")
		return
	}

	if len(req.PersonIds) == 0 {
		err = errors.New("At least one person ID is required")
		return
	}

	vbeam.UseWriteTx(ctx)

	// Get and validate photo
	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || photo.FamilyId != user.FamilyId {
		err = errors.New("Photo not found or access denied")
		return
	}

	// Get currently associated people to avoid duplicates
	existingPeople := GetPhotoPeople(ctx.Tx, req.PhotoId)
	existingPersonIds := make(map[int]bool)
	for _, person := range existingPeople {
		existingPersonIds[person.Id] = true
	}

	// Add new people
	var addedPeople []Person
	for _, personId := range req.PersonIds {
		// Skip if already associated
		if existingPersonIds[personId] {
			continue
		}

		// Validate person belongs to user's family
		person := GetPersonById(ctx.Tx, personId)
		if person.Id == 0 || person.FamilyId != user.FamilyId {
			continue // Skip invalid person but don't fail the whole operation
		}

		// Add the association
		AddPersonToPhoto(ctx.Tx, req.PhotoId, personId, user.FamilyId)
		person.Age = calculateAge(person.Birthday)
		addedPeople = append(addedPeople, person)
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	resp.People = addedPeople
	return
}

// RemovePersonFromPhotoProc removes a person from a photo
func RemovePersonFromPhotoProc(ctx *vbeam.Context, req RemovePersonFromPhotoRequest) (resp RemovePersonFromPhotoResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Validate request
	if req.PhotoId <= 0 {
		err = errors.New("Invalid photo ID")
		return
	}

	if req.PersonId <= 0 {
		err = errors.New("Invalid person ID")
		return
	}

	vbeam.UseWriteTx(ctx)

	// Get and validate photo
	photo := GetImageById(ctx.Tx, req.PhotoId)
	if photo.Id == 0 || photo.FamilyId != user.FamilyId {
		err = errors.New("Photo not found or access denied")
		return
	}

	// Validate person belongs to user's family
	person := GetPersonById(ctx.Tx, req.PersonId)
	if person.Id == 0 || person.FamilyId != user.FamilyId {
		err = errors.New("Person not found or access denied")
		return
	}

	// Remove the association
	RemovePersonFromPhoto(ctx.Tx, req.PhotoId, req.PersonId)

	vbolt.TxCommit(ctx.Tx)

	resp.Success = true
	return
}
