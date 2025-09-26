package backend

import (
	"errors"
	"family/cfg"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAdminMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListAllUsers)
	vbeam.RegisterProc(app, GetPhotoStats)
	vbeam.RegisterProc(app, ReprocessAllPhotos)
	vbeam.RegisterProc(app, GetPhotoProcessingStats)
}

type AdminUserInfo struct {
	Id         int       `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Creation   time.Time `json:"creation"`
	LastLogin  time.Time `json:"lastLogin"`
	FamilyId   int       `json:"familyId"`
	FamilyName string    `json:"familyName"`
	IsAdmin    bool      `json:"isAdmin"`
}

type ListAllUsersResponse struct {
	Users []AdminUserInfo `json:"users"`
}

// Admin-only procedure to list all registered users
func ListAllUsers(ctx *vbeam.Context, req Empty) (resp ListAllUsersResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Get all users using IterateAll
	var users []User
	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		users = append(users, user)
		return true // Continue iteration
	})

	// Convert to AdminUserInfo with family names
	resp.Users = make([]AdminUserInfo, 0, len(users))
	for _, u := range users {
		familyName := ""
		if u.FamilyId != 0 {
			family := GetFamily(ctx.Tx, u.FamilyId)
			familyName = family.Name
		}

		adminUser := AdminUserInfo{
			Id:         u.Id,
			Name:       u.Name,
			Email:      u.Email,
			Creation:   u.Creation,
			LastLogin:  u.LastLogin,
			FamilyId:   u.FamilyId,
			FamilyName: familyName,
			IsAdmin:    u.Id == 1, // Admin check
		}
		resp.Users = append(resp.Users, adminUser)
	}

	return
}

// Photo Management Types and Procedures

type GetPhotoStatsRequest struct{}

type GetPhotoStatsResponse struct {
	TotalPhotos     int `json:"totalPhotos"`
	ProcessedPhotos int `json:"processedPhotos"`
	PendingPhotos   int `json:"pendingPhotos"`
}

type ReprocessAllPhotosRequest struct{}

type ReprocessAllPhotosResponse struct {
	Processed    int      `json:"processed"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors"`
	TotalTime    string   `json:"totalTime"`
}



// Get photo statistics for admin dashboard
func GetPhotoStats(ctx *vbeam.Context, req GetPhotoStatsRequest) (resp GetPhotoStatsResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Count all photos
	var allPhotos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		allPhotos = append(allPhotos, image)
		return true
	})

	resp.TotalPhotos = len(allPhotos)

	// Count processed photos (those with modern format variants)
	processedCount := 0
	for _, photo := range allPhotos {
		if isPhotoProcessed(photo) {
			processedCount++
		}
	}

	resp.ProcessedPhotos = processedCount
	resp.PendingPhotos = resp.TotalPhotos - resp.ProcessedPhotos

	return
}

// Reprocess all photos with modern formats and responsive sizes
func ReprocessAllPhotos(ctx *vbeam.Context, req ReprocessAllPhotosRequest) (resp ReprocessAllPhotosResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	vbeam.UseWriteTx(ctx)
	startTime := time.Now()

	// Get all photos that need reprocessing
	var photosToProcess []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if !isPhotoProcessed(image) {
			photosToProcess = append(photosToProcess, image)
		}
		return true
	})

	var errors []string
	processed := 0
	failed := 0

	for _, photo := range photosToProcess {
		// Update status to processing
		photo.Status = 1 // Processing
		vbolt.Write(ctx.Tx, ImagesBkt, photo.Id, &photo)

		// Attempt to reprocess
		if reprocessErr := reprocessSinglePhoto(photo); reprocessErr != nil {
			failed++
			errors = append(errors, fmt.Sprintf("Photo %d: %v", photo.Id, reprocessErr))
			// Mark as failed/needs reprocessing
			photo.Status = 2
		} else {
			processed++
			// Mark as successfully processed
			photo.Status = 0
		}

		// Update final status
		vbolt.Write(ctx.Tx, ImagesBkt, photo.Id, &photo)
	}

	vbolt.TxCommit(ctx.Tx)

	resp.Processed = processed
	resp.Failed = failed
	resp.Errors = errors
	resp.TotalTime = time.Since(startTime).String()

	return
}

// Helper function to check if a photo has been processed with modern formats
func isPhotoProcessed(photo Image) bool {
	// Check if modern format files exist
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	// Check for at least one AVIF or WebP variant
	modernFormats := []string{".avif", ".webp"}
	sizes := []string{"", "_small", "_thumb", "_medium", "_large", "_xlarge", "_xxlarge"}

	for _, format := range modernFormats {
		for _, size := range sizes {
			var fileName string
			if size == "" || size == "_large" {
				fileName = baseFilename + format
			} else {
				fileName = baseFilename + size + format
			}

			if _, err := os.Stat(fileName); err == nil {
				return true // Found at least one modern format variant
			}
		}
	}

	return false
}

// Helper function to reprocess a single photo
func reprocessSinglePhoto(photo Image) error {
	// Read the original file
	originalPath := getOriginalPhotoPath(photo)
	originalData, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	// Reprocess with modern formats and sizes
	processedImages, _, _, err := ProcessAndSaveMultipleSizes(originalData, photo.MimeType)
	if err != nil {
		return fmt.Errorf("failed to process image: %w", err)
	}

	// Save all variants to disk
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	baseFilename := strings.TrimSuffix(basePath, filepath.Ext(basePath))

	// Clean up old variants first (except original)
	cleanupOldVariants(baseFilename)

	// Save new variants
	for key, data := range processedImages {
		parts := strings.Split(key, "_")
		if len(parts) != 2 {
			continue
		}
		sizeName, format := parts[0], parts[1]

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

		var fileName string
		if sizeName == "large" {
			fileName = baseFilename + ext
		} else {
			fileName = baseFilename + "_" + sizeName + ext
		}

		if err := os.WriteFile(fileName, data, 0644); err != nil {
			return fmt.Errorf("failed to save variant %s: %w", fileName, err)
		}
	}

	return nil
}

// Helper function to get the original photo path
func getOriginalPhotoPath(photo Image) string {
	basePath := filepath.Join(cfg.StaticDir, photo.FilePath)
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(basePath, ext)
	return base + "_original" + ext
}

// Helper function to clean up old format variants
func cleanupOldVariants(baseFilename string) {
	// Remove old JPEG variants (except original)
	oldVariants := []string{
		baseFilename + ".jpg",
		baseFilename + "_thumb.jpg",
		baseFilename + "_medium.jpg",
		baseFilename + "_small.jpg",
		baseFilename + "_xlarge.jpg",
		baseFilename + "_xxlarge.jpg",
	}

	for _, variant := range oldVariants {
		os.Remove(variant) // Ignore errors - files may not exist
	}
}

// GetPhotoProcessingStats returns statistics about photo processing queue
func GetPhotoProcessingStats(ctx *vbeam.Context, req Empty) (resp ProcessingStats, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Get processing statistics from photo worker
	resp = GetProcessingStats()
	return
}

