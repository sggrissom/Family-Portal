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

	"github.com/golang-jwt/jwt/v5"
	"github.com/rwcarlsen/goexif/exif"
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPhotoMethods(app *vbeam.Application) {
	app.HandleFunc("/api/upload-photo", uploadPhotoHandler)
	app.HandleFunc("/api/photo/", servePhotoHandler)
}

// Request/Response types
type AddPhotoRequest struct {
	PersonId    int    `json:"personId"`
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

// Database types
type Image struct {
	Id               int       `json:"id"`
	FamilyId         int       `json:"familyId"`
	PersonId         int       `json:"personId"`
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

// Packing function for vbolt serialization
func PackImage(self *Image, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.Int(&self.PersonId, buf)
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

// Buckets for vbolt database storage
var ImagesBkt = vbolt.Bucket(&cfg.Info, "images", vpack.FInt, PackImage)

// ImageByPersonIndex: term = person_id, target = image_id
var ImageByPersonIndex = vbolt.Index(&cfg.Info, "image_by_person", vpack.FInt, vpack.FInt)

// ImageByFamilyIndex: term = family_id, target = image_id
var ImageByFamilyIndex = vbolt.Index(&cfg.Info, "image_by_family", vpack.FInt, vpack.FInt)

// Database helper functions
func GetImageById(tx *vbolt.Tx, imageId int) (image Image) {
	vbolt.Read(tx, ImagesBkt, imageId, &image)
	return
}

func GetPersonImages(tx *vbolt.Tx, personId int) (images []Image) {
	var imageIds []int
	vbolt.ReadTermTargets(tx, ImageByPersonIndex, personId, &imageIds, vbolt.Window{})
	vbolt.ReadSlice(tx, ImagesBkt, imageIds, &images)
	return
}

func GetFamilyImages(tx *vbolt.Tx, familyId int) (images []Image) {
	var imageIds []int
	vbolt.ReadTermTargets(tx, ImageByFamilyIndex, familyId, &imageIds, vbolt.Window{})
	vbolt.ReadSlice(tx, ImagesBkt, imageIds, &images)
	return
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
	err := r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get auth token from cookie
	cookie, err := r.Cookie("authToken")
	if err != nil || cookie.Value == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse JWT token and get user
	var user User
	token, err := jwt.ParseWithClaims(cookie.Value, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get user from database
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		if claims, ok := token.Claims.(*Claims); ok {
			userId := GetUserId(tx, claims.Username)
			if userId != 0 {
				user = GetUser(tx, userId)
			}
		}
	})

	if user.Id == 0 {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse form data
	personIdStr := r.FormValue("personId")
	if personIdStr == "" {
		http.Error(w, "Person ID is required", http.StatusBadRequest)
		return
	}

	personId, err := strconv.Atoi(personIdStr)
	if err != nil {
		http.Error(w, "Invalid person ID", http.StatusBadRequest)
		return
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
		http.Error(w, "Photo file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file size (32MB max for original upload)
	if fileHeader.Size > 32<<20 { // 32MB
		http.Error(w, "File size too large. Maximum 32MB allowed", http.StatusBadRequest)
		return
	}

	// Validate file type
	mimeType := fileHeader.Header.Get("Content-Type")
	if !isValidImageType(mimeType) {
		http.Error(w, "Invalid file type. Only JPEG, PNG, and GIF images are allowed", http.StatusBadRequest)
		return
	}

	// Get image dimensions
	width, height, err := getImageDimensions(file)
	if err != nil {
		http.Error(w, "Failed to read image dimensions", http.StatusBadRequest)
		return
	}

	// Database operations
	var person Person
	var image Image

	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		// Validate person exists and belongs to user's family
		person = GetPersonById(tx, personId)
		if person.Id == 0 || person.FamilyId != user.FamilyId {
			// Don't commit transaction for invalid person
			return
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
		calculatedPhotoDate, err := calculatePhotoDate(inputType, photoDate, ageYears, ageMonths, person, fileData)
		if err != nil {
			// Don't commit transaction for invalid date
			return
		}

		// Generate title if not provided
		if title == "" {
			title = generateDefaultTitle(fileHeader.Filename, calculatedPhotoDate)
		}

		// Process image and create multiple sizes
		processedImages, processedWidth, processedHeight, err := ProcessAndSaveMultipleSizes(fileData, mimeType)
		if err != nil {
			// Fall back to saving original if processing fails
			processedImages = map[string][]byte{
				"large": fileData,
			}
			processedWidth = width
			processedHeight = height
		}

		// Update dimensions with processed values
		if processedWidth > 0 {
			width = processedWidth
		}
		if processedHeight > 0 {
			height = processedHeight
		}

		// Save all image variants to disk
		baseFilename := strings.TrimSuffix(uniqueFilename, filepath.Ext(uniqueFilename))
		extension := filepath.Ext(uniqueFilename)

		// Save large/main image
		filePath := filepath.Join(photosDir, uniqueFilename)
		if largeData, ok := processedImages["large"]; ok {
			dst, err := os.Create(filePath)
			if err != nil {
				return
			}
			defer dst.Close()
			_, err = dst.Write(largeData)
			if err != nil {
				return
			}
		}

		// Save medium image
		if mediumData, ok := processedImages["medium"]; ok {
			mediumPath := filepath.Join(photosDir, baseFilename+"_medium"+extension)
			mediumFile, err := os.Create(mediumPath)
			if err == nil {
				mediumFile.Write(mediumData)
				mediumFile.Close()
			}
		}

		// Save thumbnail
		if thumbData, ok := processedImages["thumb"]; ok {
			thumbnailPath := filepath.Join(photosDir, baseFilename+"_thumb"+extension)
			thumbFile, err := os.Create(thumbnailPath)
			if err == nil {
				thumbFile.Write(thumbData)
				thumbFile.Close()
			}
		}

		// Save original for backup (optional)
		originalPath := filepath.Join(photosDir, baseFilename+"_original"+extension)
		origFile, err := os.Create(originalPath)
		if err == nil {
			origFile.Write(fileData)
			origFile.Close()
		}

		// Create image record
		image = Image{
		Id:               vbolt.NextIntId(tx, ImagesBkt),
		FamilyId:         user.FamilyId,
		PersonId:         personId,
		OwnerUserId:      user.Id,
		OriginalFilename: fileHeader.Filename,
		MimeType:         mimeType,
		FileSize:         len(processedImages["large"]),
		Width:            width,
		Height:           height,
		FilePath:         fmt.Sprintf("photos/%s", uniqueFilename),
		Title:            title,
		Description:      description,
		PhotoDate:        calculatedPhotoDate,
		CreatedAt:        time.Now(),
		Status:           0, // Active
	}

		// Save to database
		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByPersonIndex, image.Id, personId)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, user.FamilyId)
		vbolt.TxCommit(tx)
	})

	// Check if image was created (transaction succeeded)
	if image.Id == 0 {
		http.Error(w, "Failed to upload photo", http.StatusInternalServerError)
		return
	}

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

	// Get auth token from cookie
	cookie, err := r.Cookie("authToken")
	if err != nil || cookie.Value == "" {
		http.Error(w, "Not found", http.StatusNotFound) // Return 404 to avoid leaking photo existence
		return
	}

	// Parse JWT token and get user
	var user User
	token, err := jwt.ParseWithClaims(cookie.Value, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtKey, nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "Not found", http.StatusNotFound) // Return 404 to avoid leaking photo existence
		return
	}

	// Get user from database
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		if claims, ok := token.Claims.(*Claims); ok {
			userId := GetUserId(tx, claims.Username)
			if userId != 0 {
				user = GetUser(tx, userId)
			}
		}
	})

	if user.Id == 0 {
		http.Error(w, "Not found", http.StatusNotFound) // Return 404 to avoid leaking photo existence
		return
	}

	// Get image from database
	var image Image
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		image = GetImageById(tx, imageId)
	})

	// Check if image exists and user has access
	if image.Id == 0 || image.FamilyId != user.FamilyId || image.Status != 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Construct full file path based on size variant
	basePath := filepath.Join(cfg.StaticDir, image.FilePath)
	fullPath := basePath

	// Handle size variants
	if sizeVariant != "" {
		ext := filepath.Ext(basePath)
		base := strings.TrimSuffix(basePath, ext)

		switch sizeVariant {
		case "thumb":
			fullPath = base + "_thumb" + ext
		case "medium":
			fullPath = base + "_medium" + ext
		case "original":
			fullPath = base + "_original" + ext
		case "large":
			// large is the default main image
			fullPath = basePath
		default:
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}

	// Validate that the file path doesn't contain directory traversal
	cleanPath := filepath.Clean(fullPath)
	staticDir := filepath.Clean(cfg.StaticDir)
	if !strings.HasPrefix(cleanPath, staticDir) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Check if file exists, fall back to main image if variant doesn't exist
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if sizeVariant != "" {
			// Fall back to main image
			fullPath = basePath
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}

	// Set content type based on stored mime type
	w.Header().Set("Content-Type", image.MimeType)

	// Set cache headers for browser caching (24 hours)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("ETag", fmt.Sprintf("\"%d-%d\"", image.Id, image.CreatedAt.Unix()))

	// Serve the file
	http.ServeFile(w, r, fullPath)
}