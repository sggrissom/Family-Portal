package backend

import (
	"time"

	"go.hasen.dev/vbolt"
)

type ExportVideo struct {
	Id               int               `json:"id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	PhotoDate        time.Time         `json:"photo_date"`
	RecordTime       *RecordTime       `json:"record_time,omitempty"`
	DateLabel        string            `json:"date_label,omitempty"`
	OriginalFilename string            `json:"original_filename"`
	FileName         string            `json:"file_name"`
	MimeType         string            `json:"mime_type"`
	FileSize         int               `json:"file_size"`
	DurationMs       int               `json:"duration_ms"`
	PersonIds        []int             `json:"person_ids"`
	TagIds           []int             `json:"tag_ids"`
	Available        bool              `json:"available"`
	Transcript       *ExportTranscript `json:"transcript,omitempty"`
}

type ExportTranscript struct {
	Automatic bool                `json:"automatic"`
	Language  string              `json:"language"`
	Segments  []TranscriptSegment `json:"segments"`
}

func exportTranscript(tx *vbolt.Tx, imageId int) *ExportTranscript {
	t, ok := GetVideoTranscriptRecord(tx, imageId)
	if !ok || t.Status != TranscriptDone {
		return nil
	}
	return &ExportTranscript{Automatic: t.EditedBy == 0, Language: t.Language, Segments: t.Segments}
}

func buildVideoExport(tx *vbolt.Tx, familyId int) []ExportVideo {
	result := []ExportVideo{}
	for _, image := range GetFamilyImages(tx, familyId) {
		if !image.IsVideo() || image.OriginalKey == "" || image.Status == PhotoProcessing {
			continue
		}
		personIds := []int{}
		for _, pp := range GetPhotoPersonsByPhoto(tx, image.Id) {
			personIds = append(personIds, pp.PersonId)
		}
		result = append(result, ExportVideo{
			Id:               image.Id,
			Title:            image.Title,
			Description:      image.Description,
			PhotoDate:        image.PhotoDate,
			RecordTime:       &image.RecordTime,
			DateLabel:        image.RecordTime.Label(),
			OriginalFilename: image.OriginalFilename,
			FileName:         videoExportFileName(image),
			MimeType:         image.MimeType,
			FileSize:         image.FileSize,
			DurationMs:       image.DurationMs,
			PersonIds:        personIds,
			TagIds:           GetPhotoTagIds(tx, image.Id),
			Available:        image.Status == PhotoReady,
			Transcript:       exportTranscript(tx, image.Id),
		})
	}
	return result
}
