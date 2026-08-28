package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"family/cfg"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

	var requestedFamilyId int
	if familyIdStr := r.FormValue("familyId"); familyIdStr != "" {
		parsed, convErr := strconv.Atoi(familyIdStr)
		if convErr != nil {
			RespondValidationError(w, r, "Invalid family ID", convErr.Error())
			return
		}
		requestedFamilyId = parsed
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
	var importErr error
	vbolt.WithWriteTx(appDb, func(tx *vbolt.Tx) {
		familyId, resolveErr := ResolveActingFamily(tx, user, requestedFamilyId, AccessContribute)
		if resolveErr != nil {
			importErr = resolveErr
			return
		}

		personIdMapping, importedPeople, mergedPeople, peopleErrors, peopleWarnings := importPeople(tx, importData.People, familyId, "merge_people")
		resp.ImportedPeople = importedPeople
		resp.MergedPeople = mergedPeople
		resp.PersonIdMapping = personIdMapping
		resp.Errors = append(resp.Errors, peopleErrors...)
		resp.Warnings = append(resp.Warnings, peopleWarnings...)

		tagNameToId, importedTags, skippedTags := importTags(tx, importData.Tags, familyId)
		resp.ImportedTags = importedTags
		resp.SkippedTags = skippedTags

		tagIdMapping := make(map[int]int)
		for _, exportTag := range importData.Tags {
			lowerName := strings.ToLower(exportTag.Name)
			if newId, ok := tagNameToId[lowerName]; ok {
				tagIdMapping[exportTag.Id] = newId
			}
		}

		if len(personIdMapping) > 0 {
			filteredHeights, filteredWeights := filterMeasurements(importData.Heights, importData.Weights, personIdMapping)
			importedMeasurements, skippedMeasurements, measurementErrors := importMeasurements(tx, filteredHeights, filteredWeights, personIdMapping, familyId)
			resp.ImportedMeasurements = importedMeasurements
			resp.SkippedMeasurements = skippedMeasurements
			resp.Errors = append(resp.Errors, measurementErrors...)

			if len(importData.Milestones) > 0 {
				filteredMilestones := filterMilestones(importData.Milestones, personIdMapping)
				importedMilestones, skippedMilestones, milestoneErrors := importMilestones(tx, filteredMilestones, personIdMapping, familyId, tagNameToId)
				resp.ImportedMilestones = importedMilestones
				resp.SkippedMilestones = skippedMilestones
				resp.Errors = append(resp.Errors, milestoneErrors...)
			}
		}

		var photoIdMapping map[int]int
		if len(importData.Photos) > 0 {
			imported, skipped, mapping, photoWarnings := importPhotos(tx, familyId, user.Id, importData.Photos, personIdMapping, tagIdMapping, zipReader)
			resp.ImportedPhotos = imported
			resp.SkippedPhotos = skipped
			photoIdMapping = mapping
			resp.Warnings = append(resp.Warnings, photoWarnings...)

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
				person.ProfileCropX = 50
				person.ProfileCropY = 50
				person.ProfileCropScale = 1.0
				vbolt.Write(tx, PeopleBkt, person.Id, &person)
			}
		}

		if len(importData.Activities) > 0 {
			counts, activityWarnings := importActivities(tx, importData.Activities, familyId, personIdMapping, photoIdMapping)
			resp.ImportedActivities = counts
			resp.Warnings = append(resp.Warnings, activityWarnings...)
		}

		resp.SkippedPeople = len(importData.People) - resp.ImportedPeople - resp.MergedPeople

		vbolt.TxCommit(tx)
	})

	if importErr != nil {
		RespondValidationError(w, r, "Failed to import bundle", importErr.Error())
		return
	}

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
) (imported, skipped int, photoIdMapping map[int]int, warnings []string) {
	photoIdMapping = make(map[int]int)
	zipFiles := make(map[string]*zip.File, len(zipReader.File))
	for _, zf := range zipReader.File {
		zipFiles[zf.Name] = zf
	}

	// Going over stops the photo loop rather than failing the import: the people
	// and measurements already written in this transaction are worth keeping.
	used := FamilyStorageUsage(tx, familyId)
	quotaReached := false
	quotaSkipped := 0

	var incoming int64
	for _, zf := range zipReader.File {
		incoming += int64(zf.UncompressedSize64)
	}
	if diskErr := CheckDiskHeadroom(cfg.StaticDir, incoming, cfg.MinFreeDiskBytes); diskErr != nil {
		return 0, len(photos), photoIdMapping, []string{
			"The server is low on storage, so no photos were imported. Everything else in the bundle was.",
		}
	}

	for _, photo := range photos {
		zf, exists := zipFiles[photo.ZipPath]
		if !exists {
			skipped++
			continue
		}

		if quotaReached {
			skipped++
			quotaSkipped++
			continue
		}

		// Tracked locally; re-reading usage per photo would be quadratic.
		size := int64(zf.UncompressedSize64)
		if cfg.FamilyStorageQuotaBytes > 0 && used+size > cfg.FamilyStorageQuotaBytes {
			quotaReached = true
			skipped++
			quotaSkipped++
			continue
		}
		used += size

		fileName := filepath.Base(photo.ZipPath)
		ext := filepath.Ext(fileName)
		base := strings.TrimSuffix(fileName, ext)
		base = strings.TrimSuffix(base, "_original")
		filePath := "photos/" + base + ext

		mimeType := zipExtToMime(ext)

		diskPath := filepath.Join(cfg.StaticDir, photo.ZipPath)
		if err := writeZipEntryToDisk(zf, diskPath); err != nil {
			log.Printf("[IMPORT] Failed to write photo %s: %v", diskPath, err)
			skipped++
			continue
		}

		var newTagIds []int
		for _, oldTagId := range photo.TagIds {
			if newId, ok := tagIdMapping[oldTagId]; ok {
				newTagIds = append(newTagIds, newId)
			}
		}

		var image Image
		image.Id = vbolt.NextIntId(tx, ImagesBkt)
		image.FamilyId = familyId
		image.OwnerUserId = ownerUserId
		image.FilePath = filePath
		image.MimeType = mimeType
		image.FileSize = int(size)
		image.Title = photo.Title
		image.Description = photo.Description
		image.PhotoDate = photo.PhotoDate
		image.Status = 0
		image.CreatedAt = time.Now()

		vbolt.Write(tx, ImagesBkt, image.Id, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
		photoIdMapping[photo.Id] = image.Id

		for _, tagId := range newTagIds {
			addTagToPhoto(tx, image.Id, tagId, familyId)
		}

		for _, oldPersonId := range photo.PersonIds {
			if newPersonId, ok := personIdMapping[oldPersonId]; ok {
				AddPersonToPhoto(tx, image.Id, newPersonId, familyId)
			}
		}

		imported++
	}

	if quotaReached {
		warnings = append(warnings, fmt.Sprintf(
			"Storage quota reached: %d photo(s) were not imported. Everything else in the bundle was.",
			quotaSkipped,
		))
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
