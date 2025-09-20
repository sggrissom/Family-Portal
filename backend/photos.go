package backend

import (
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
	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

func RegisterPhotoMethods(app *vbeam.Application) {
	app.HandleFunc("/api/upload-photo", uploadPhotoHandler)
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

// Calculate photo date based on input type
func calculatePhotoDate(inputType string, photoDate string, ageYears *int, ageMonths *int, person Person) (time.Time, error) {
	switch inputType {
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

	// Parse multipart form (10MB max)
	err := r.ParseMultipartForm(10 << 20) // 10MB
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
	if title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

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

	// Validate file size (10MB max)
	if fileHeader.Size > 10<<20 { // 10MB
		http.Error(w, "File size too large. Maximum 10MB allowed", http.StatusBadRequest)
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

		// Calculate photo date
		calculatedPhotoDate, err := calculatePhotoDate(inputType, photoDate, ageYears, ageMonths, person)
		if err != nil {
			// Don't commit transaction for invalid date
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

		// Save file to disk
		filePath := filepath.Join(photosDir, uniqueFilename)
		dst, err := os.Create(filePath)
		if err != nil {
			// Don't commit transaction if file creation fails
			return
		}
		defer dst.Close()

		// Reset file position and copy
		file.Seek(0, 0)
		_, err = io.Copy(dst, file)
		if err != nil {
			// Don't commit transaction if file copy fails
			return
		}

		// Create image record
		image = Image{
		Id:               vbolt.NextIntId(tx, ImagesBkt),
		FamilyId:         user.FamilyId,
		PersonId:         personId,
		OwnerUserId:      user.Id,
		OriginalFilename: fileHeader.Filename,
		MimeType:         mimeType,
		FileSize:         int(fileHeader.Size),
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