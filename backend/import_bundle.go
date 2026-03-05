package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"family/cfg"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

func importBundleHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r)
	if !ok {
		RespondAuthError(w, r, "Authentication required")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		RespondValidationError(w, r, "Failed to parse multipart form", err.Error())
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		RespondValidationError(w, r, "No file provided", err.Error())
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		RespondInternalError(w, r, "Failed to read file", err.Error())
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		RespondValidationError(w, r, "Invalid ZIP file", err.Error())
		return
	}

	// Find and parse data.json
	var importData ImportDataStructure
	found := false
	for _, zf := range zipReader.File {
		if zf.Name == "data.json" {
			rc, err := zf.Open()
			if err != nil {
				RespondInternalError(w, r, "Failed to open data.json", err.Error())
				return
			}
			jsonBytes, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				RespondInternalError(w, r, "Failed to read data.json", err.Error())
				return
			}
			if err := json.Unmarshal(jsonBytes, &importData); err != nil {
				RespondValidationError(w, r, "Invalid data.json", err.Error())
				return
			}
			found = true
			break
		}
	}
	if !found {
		RespondValidationError(w, r, "ZIP does not contain data.json")
		return
	}

	if err := validateImportData(importData); err != nil {
		RespondValidationError(w, r, err.Error())
		return
	}

	var resp ImportDataResponse
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		personIdMapping, importedPeople, mergedPeople, peopleErrors, peopleWarnings := importPeople(tx, importData.People, user.FamilyId, "merge_people")
		resp.ImportedPeople = importedPeople
		resp.MergedPeople = mergedPeople
		resp.PersonIdMapping = personIdMapping
		resp.Errors = append(resp.Errors, peopleErrors...)
		resp.Warnings = append(resp.Warnings, peopleWarnings...)

		tagNameToId, importedTags, skippedTags := importTags(tx, importData.Tags, user.FamilyId)
		resp.ImportedTags = importedTags
		resp.SkippedTags = skippedTags

		// Build old tag ID → new tag ID mapping
		tagIdMapping := make(map[int]int)
		for _, exportTag := range importData.Tags {
			lowerName := strings.ToLower(exportTag.Name)
			if newId, ok := tagNameToId[lowerName]; ok {
				tagIdMapping[exportTag.Id] = newId
			}
		}

		if len(personIdMapping) > 0 {
			filteredHeights, filteredWeights := filterMeasurements(importData.Heights, importData.Weights, personIdMapping)
			importedMeasurements, skippedMeasurements, measurementErrors := importMeasurements(tx, filteredHeights, filteredWeights, personIdMapping, user.FamilyId)
			resp.ImportedMeasurements = importedMeasurements
			resp.SkippedMeasurements = skippedMeasurements
			resp.Errors = append(resp.Errors, measurementErrors...)

			if len(importData.Milestones) > 0 {
				filteredMilestones := filterMilestones(importData.Milestones, personIdMapping)
				importedMilestones, skippedMilestones, milestoneErrors := importMilestones(tx, filteredMilestones, personIdMapping, user.FamilyId, tagNameToId)
				resp.ImportedMilestones = importedMilestones
				resp.SkippedMilestones = skippedMilestones
				resp.Errors = append(resp.Errors, milestoneErrors...)
			}
		}

		if len(importData.Photos) > 0 {
			imported, skipped, photoIdMapping := importPhotos(tx, user.FamilyId, user.Id, importData.Photos, personIdMapping, tagIdMapping, zipReader)
			resp.ImportedPhotos = imported
			resp.SkippedPhotos = skipped

			// Restore profile photos
			for _, importPerson := range importData.People {
				if importPerson.ImageId == 0 {
					continue
				}
				newPersonId, ok := personIdMapping[importPerson.Id]
				if !ok {
					continue
				}
				newPhotoId, ok := photoIdMapping[importPerson.ImageId]
				if !ok {
					continue
				}
				var person Person
				vbolt.Read(tx, PeopleBkt, newPersonId, &person)
				person.ProfilePhotoId = newPhotoId
				vbolt.Write(tx, PeopleBkt, person.Id, &person)
			}
		}

		resp.SkippedPeople = len(importData.People) - resp.ImportedPeople - resp.MergedPeople

		vbolt.TxCommit(tx)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func importPhotos(
	tx *vbolt.Tx,
	familyId int,
	ownerUserId int,
	photos []ExportPhoto,
	personIdMapping map[int]int,
	tagIdMapping map[int]int,
	zipReader *zip.Reader,
) (imported, skipped int, photoIdMapping map[int]int) {
	photoIdMapping = make(map[int]int)
	// Build a lookup map for ZIP entries
	zipFiles := make(map[string]*zip.File, len(zipReader.File))
	for _, zf := range zipReader.File {
		zipFiles[zf.Name] = zf
	}

	for _, photo := range photos {
		zf, exists := zipFiles[photo.ZipPath]
		if !exists {
			skipped++
			continue
		}

		// Derive FilePath by stripping _original suffix
		fileName := filepath.Base(photo.ZipPath)
		ext := filepath.Ext(fileName)
		base := strings.TrimSuffix(fileName, ext)
		base = strings.TrimSuffix(base, "_original")
		filePath := "photos/" + base + ext

		mimeType := zipExtToMime(ext)

		// Write photo file to disk
		diskPath := filepath.Join(cfg.StaticDir, photo.ZipPath)
		if err := writeZipEntryToDisk(zf, diskPath); err != nil {
			log.Printf("[IMPORT] Failed to write photo %s: %v", diskPath, err)
			skipped++
			continue
		}

		// Remap tag IDs
		var newTagIds []int
		for _, oldTagId := range photo.TagIds {
			if newId, ok := tagIdMapping[oldTagId]; ok {
				newTagIds = append(newTagIds, newId)
			}
		}

		// Create Image record
		var image Image
		image.Id = vbolt.NextIntId(tx, ImagesBkt)
		image.FamilyId = familyId
		image.OwnerUserId = ownerUserId
		image.FilePath = filePath
		image.MimeType = mimeType
		image.Title = photo.Title
		image.Description = photo.Description
		image.PhotoDate = photo.PhotoDate
		image.Status = 0
		image.CreatedAt = time.Now()

		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
		photoIdMapping[photo.Id] = image.Id

		// Apply tags
		for _, tagId := range newTagIds {
			addTagToPhoto(tx, image.Id, tagId, familyId)
		}

		// Link people
		for _, oldPersonId := range photo.PersonIds {
			if newPersonId, ok := personIdMapping[oldPersonId]; ok {
				AddPersonToPhoto(tx, image.Id, newPersonId, familyId)
			}
		}

		imported++
	}
	return
}

func writeZipEntryToDisk(zf *zip.File, diskPath string) error {
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		return err
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(diskPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func zipExtToMime(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
