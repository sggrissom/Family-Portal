package backend

import (
	"family/cfg"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

type ExportPhoto struct {
	Id               int                 `json:"id"`
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	OriginalFilename string              `json:"original_filename,omitempty"`
	PhotoDate        time.Time           `json:"photo_date"`
	RecordTime       *RecordTime         `json:"record_time,omitempty"`
	DateLabel        string              `json:"date_label,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	ZipPath          string              `json:"zip_path"`
	PersonIds        []int               `json:"person_ids"`
	TagIds           []int               `json:"tag_ids"`
	People           []ExportPhotoPerson `json:"people,omitempty"`
	Tags             []ExportPhotoTag    `json:"tags,omitempty"`
	AnalysisVersion  int                 `json:"analysis_version,omitempty"`
	Faces            []ExportFace        `json:"faces,omitempty"`
}

// ExportPhotoPerson and ExportPhotoTag carry what PersonIds and TagIds can't.
type ExportPhotoPerson struct {
	PersonId   int       `json:"person_id"`
	AutoTagged bool      `json:"auto_tagged,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type ExportPhotoTag struct {
	TagId     int       `json:"tag_id"`
	CreatedAt time.Time `json:"created_at"`
}

type ExportFace struct {
	PersonId          int       `json:"person_id,omitempty"`
	Status            int       `json:"status"`
	Distance          float64   `json:"distance,omitempty"`
	Box               FaceBox   `json:"box"`
	RejectedPersonIds []int     `json:"rejected_person_ids,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	FrameMs           int       `json:"frame_ms,omitempty"`
	FrameCount        int       `json:"frame_count,omitempty"`
}

type ExportPlace struct {
	Name         string    `json:"name"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	RadiusMeters int       `json:"radius_meters"`
	CreatedAt    time.Time `json:"created_at"`
}

func buildPhotoExportMetadata(tx *vbolt.Tx, familyId int) []ExportPhoto {
	images := GetFamilyImages(tx, familyId)
	result := make([]ExportPhoto, 0, len(images))
	for _, img := range images {
		if img.Status != PhotoReady || img.IsVideo() {
			continue
		}
		photo := ExportPhoto{
			Id:               img.Id,
			Title:            img.Title,
			Description:      img.Description,
			OriginalFilename: img.OriginalFilename,
			PhotoDate:        img.PhotoDate,
			RecordTime:       &img.RecordTime,
			DateLabel:        img.RecordTime.Label(),
			CreatedAt:        img.CreatedAt,
			ZipPath:          "photos/" + originalFileName(img.FilePath),
			PersonIds:        []int{},
			TagIds:           []int{},
			AnalysisVersion:  img.AnalysisVersion,
		}
		for _, pp := range GetPhotoPersonsByPhoto(tx, img.Id) {
			photo.PersonIds = append(photo.PersonIds, pp.PersonId)
			photo.People = append(photo.People, ExportPhotoPerson{PersonId: pp.PersonId, AutoTagged: pp.AutoTagged, CreatedAt: pp.CreatedAt})
		}
		for _, pt := range getPhotoTagRows(tx, img.Id) {
			photo.TagIds = append(photo.TagIds, pt.TagId)
			photo.Tags = append(photo.Tags, ExportPhotoTag{TagId: pt.TagId, CreatedAt: pt.CreatedAt})
		}
		for _, face := range GetPhotoFacesTx(tx, img.Id) {
			photo.Faces = append(photo.Faces, ExportFace{
				PersonId: face.PersonId, Status: face.Status, Distance: face.Distance, Box: face.Box,
				RejectedPersonIds: face.RejectedPersonIds, CreatedAt: face.CreatedAt,
				FrameMs: face.FrameMs, FrameCount: face.FrameCount,
			})
		}
		result = append(result, photo)
	}
	return result
}

func getPhotoTagRows(tx *vbolt.Tx, photoId int) (rows []PhotoTag) {
	var ids []int
	vbolt.ReadTermTargets(tx, PhotoTagByPhotoIndex, photoId, &ids, vbolt.Window{})
	vbolt.ReadSlice(tx, PhotoTagBkt, ids, &rows)
	return
}

func buildPlaceExport(tx *vbolt.Tx, familyId int) (places []ExportPlace) {
	for _, place := range GetFamilyPlaces(tx, familyId) {
		places = append(places, ExportPlace{
			Name: place.Name, Latitude: place.Latitude, Longitude: place.Longitude,
			RadiusMeters: place.RadiusMeters, CreatedAt: place.CreatedAt,
		})
	}
	return
}

// importPlaces adds the places the family doesn't already have by name.
func importPlaces(tx *vbolt.Tx, places []ExportPlace, familyId int) {
	have := map[string]bool{}
	for _, place := range GetFamilyPlaces(tx, familyId) {
		have[strings.ToLower(place.Name)] = true
	}
	for _, p := range places {
		if have[strings.ToLower(p.Name)] {
			continue
		}
		have[strings.ToLower(p.Name)] = true
		place := FamilyPlace{
			Id: vbolt.NextIntId(tx, FamilyPlaceBkt), FamilyId: familyId, Name: p.Name,
			Latitude: p.Latitude, Longitude: p.Longitude, RadiusMeters: p.RadiusMeters,
			CreatedAt: orNow(p.CreatedAt),
		}
		vbolt.Write(tx, FamilyPlaceBkt, place.Id, &place)
		vbolt.SetTargetSingleTerm(tx, FamilyPlaceByFamilyIndex, place.Id, familyId)
	}
}

func orNow(at time.Time) time.Time {
	if at.IsZero() {
		return time.Now()
	}
	return at
}

// importPhotoLinks writes a new photo's people, tags and faces.
func importPhotoLinks(tx *vbolt.Tx, image Image, photo ExportPhoto, l *archiveLedger) {
	people := map[int]ExportPhotoPerson{}
	for _, pp := range photo.People {
		people[pp.PersonId] = pp
	}
	for _, oldPersonId := range photo.PersonIds {
		newPersonId, ok := l.people[oldPersonId]
		if !ok {
			continue
		}
		detail := people[oldPersonId]
		insertPhotoPerson(tx, &PhotoPerson{
			PhotoId: image.Id, PersonId: newPersonId, FamilyId: image.FamilyId,
			CreatedAt: orNow(detail.CreatedAt), AutoTagged: detail.AutoTagged,
		})
	}

	tags := map[int]time.Time{}
	for _, pt := range photo.Tags {
		tags[pt.TagId] = pt.CreatedAt
	}
	for _, oldTagId := range photo.TagIds {
		newTagId, ok := l.tags[oldTagId]
		if !ok {
			continue
		}
		pt := PhotoTag{
			Id: vbolt.NextIntId(tx, PhotoTagBkt), PhotoId: image.Id, TagId: newTagId,
			FamilyId: image.FamilyId, CreatedAt: orNow(tags[oldTagId]),
		}
		vbolt.Write(tx, PhotoTagBkt, pt.Id, &pt)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByPhotoIndex, pt.Id, image.Id)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByTagIndex, pt.Id, newTagId)
		vbolt.SetTargetSingleTerm(tx, PhotoTagByFamilyIndex, pt.Id, image.FamilyId)
	}

	for _, f := range photo.Faces {
		face := PhotoFace{
			Id: vbolt.NextIntId(tx, PhotoFaceBkt), PhotoId: image.Id, FamilyId: image.FamilyId,
			Status: f.Status, Distance: f.Distance, Box: f.Box, CreatedAt: orNow(f.CreatedAt),
			FrameMs: f.FrameMs, FrameCount: f.FrameCount,
		}
		if f.PersonId != 0 {
			if face.PersonId = l.people[f.PersonId]; face.PersonId == 0 {
				face.Status, face.Distance = FaceUnknown, 0
			}
		}
		for _, rejected := range f.RejectedPersonIds {
			if id, ok := l.people[rejected]; ok {
				face.RejectedPersonIds = append(face.RejectedPersonIds, id)
			}
		}
		writeFace(tx, &face)
	}
}

func importPhotos(
	tx *vbolt.Tx,
	familyId int,
	ownerUserId int,
	photos []ExportPhoto,
	l *archiveLedger,
	staged map[int]stagedMedia,
) (imported, skipped int, jobs []importedPhoto, warnings []string) {
	// Going over stops the photo loop rather than failing the import: the people
	// and measurements already written in this transaction are worth keeping.
	used := FamilyStorageUsage(tx, familyId)
	quotaReached := false
	quotaSkipped, noRoom := 0, 0

	existing := familyMediaByHash(tx, familyId)
	for _, photo := range photos {
		if !inScope(photo.PersonIds, l) {
			continue
		}
		file, exists := staged[photo.Id]
		if !exists {
			skipped++
			continue
		}
		hash := file.hash
		if id, ok := takeMediaByHash(existing, hash); ok {
			l.photos[photo.Id] = id
			skipped++
			continue
		}
		if file.noRoom {
			skipped++
			noRoom++
			continue
		}

		if quotaReached {
			skipped++
			quotaSkipped++
			continue
		}

		// Tracked locally; re-reading usage per photo would be quadratic.
		size := file.size
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

		var image Image
		image.FamilyId = familyId
		image.OwnerUserId = ownerUserId
		image.FilePath = filePath
		image.MimeType = mimeType
		image.FileSize = int(size)
		image.ContentHash = hash
		image.OriginalFilename = photo.OriginalFilename
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
		image.CreatedAt = orNow(photo.CreatedAt)
		if photo.AnalysisVersion > 0 {
			image.AnalysisStatus = AnalysisDone
			image.AnalysisVersion = photo.AnalysisVersion
		}

		createImage(tx, &image)
		l.photos[photo.Id] = image.Id
		jobs = append(jobs, importedPhoto{
			ImageId:  image.Id,
			FamilyId: familyId,
			FilePath: filePath,
			MimeType: mimeType,
			File:     stagedMove{From: file.path, To: photoOriginalPath(filePath)},
		})

		importPhotoLinks(tx, image, photo, l)

		imported++
	}

	if noRoom > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"The server is low on storage, so %d photo(s) were not imported. Everything else in the bundle was.",
			noRoom,
		))
	}
	if quotaReached {
		warnings = append(warnings, fmt.Sprintf(
			"Storage quota reached: %d photo(s) were not imported. Everything else in the bundle was.",
			quotaSkipped,
		))
	}
	return
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
