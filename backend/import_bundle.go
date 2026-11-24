package backend

import (
	"archive/zip"
	"family/cfg"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

func importPhotos(
	tx *vbolt.Tx,
	familyId int,
	ownerUserId int,
	photos []ExportPhoto,
	l *archiveLedger,
	zipFiles map[string]*zip.File,
	dryRun bool,
) (imported, skipped int, jobs []PhotoProcessingJob, warnings []string) {
	// Going over stops the photo loop rather than failing the import: the people
	// and measurements already written in this transaction are worth keeping.
	used := FamilyStorageUsage(tx, familyId)
	quotaReached := false
	quotaSkipped := 0

	var incoming int64
	for _, zf := range zipFiles {
		incoming += int64(zf.UncompressedSize64)
	}
	if diskErr := CheckDiskHeadroom(cfg.StaticDir, incoming, cfg.MinFreeDiskBytes); diskErr != nil && !dryRun {
		return 0, len(photos), nil, []string{
			"The server is low on storage, so no photos were imported. Everything else in the bundle was.",
		}
	}

	for _, photo := range photos {
		zf, exists := zipFiles[photo.ZipPath]
		if !exists || !photoInScope(photo, l) {
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

		// A fresh name, so importing into the server that exported the bundle
		// does not share files with the photo it came from.
		uniqueFilename, err := generateUniqueFilename(filePathForOriginal(filepath.Base(photo.ZipPath)))
		if err != nil {
			skipped++
			continue
		}
		filePath := "photos/" + uniqueFilename
		mimeType := zipExtToMime(filepath.Ext(filePath))

		if !dryRun {
			if err := writeZipEntryToDisk(zf, photoOriginalPath(filePath)); err != nil {
				LogWarn(LogCategoryPhoto, "Import could not write a photo", map[string]interface{}{"path": photo.ZipPath, "error": err.Error()})
				skipped++
				continue
			}
		}

		var newTagIds []int
		for _, oldTagId := range photo.TagIds {
			if newId, ok := l.tags[oldTagId]; ok {
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
		recordTime, photoDate, err := importRecordTime(photo.RecordTime, wallClockUTC(photo.PhotoDate), l.people, false)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Photo %q was imported without its date: %s", photo.Title, err.Error()))
			recordTime, photoDate = unknownRecordTime(SourceLegacy), time.Time{}
		}
		image.PhotoDate = photoDate
		image.RecordTime = recordTime
		image.Status = PhotoProcessing
		image.CreatedAt = time.Now()

		writeImage(tx, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
		ReindexPhotoDates(tx, image.Id)
		l.photos[photo.Id] = image.Id
		jobs = append(jobs, PhotoProcessingJob{
			ImageId:  image.Id,
			FamilyId: familyId,
			FilePath: filePath,
			MimeType: mimeType,
		})

		for _, tagId := range newTagIds {
			addTagToPhoto(tx, image.Id, tagId, familyId)
		}

		for _, oldPersonId := range photo.PersonIds {
			if newPersonId, ok := l.people[oldPersonId]; ok {
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
