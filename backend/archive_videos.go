package backend

import (
	"family/cfg"
	"fmt"
	"strings"
	"time"

	"go.hasen.dev/vbolt"
)

type ExportVideo struct {
	Id               int                 `json:"id"`
	Title            string              `json:"title"`
	Description      string              `json:"description"`
	PhotoDate        time.Time           `json:"photo_date"`
	RecordTime       *RecordTime         `json:"record_time,omitempty"`
	DateLabel        string              `json:"date_label,omitempty"`
	OriginalFilename string              `json:"original_filename"`
	FileName         string              `json:"file_name"`
	MimeType         string              `json:"mime_type"`
	FileSize         int                 `json:"file_size"`
	DurationMs       int                 `json:"duration_ms"`
	PersonIds        []int               `json:"person_ids"`
	TagIds           []int               `json:"tag_ids"`
	People           []ExportPhotoPerson `json:"people,omitempty"`
	Tags             []ExportPhotoTag    `json:"tags,omitempty"`
	Available        bool                `json:"available"`
	CreatedAt        time.Time           `json:"created_at,omitzero"`
	Transcript       *ExportTranscript   `json:"transcript,omitempty"`
}

type ExportTranscript struct {
	Automatic bool                `json:"automatic"`
	Removed   bool                `json:"removed,omitempty"`
	Language  string              `json:"language"`
	Segments  []TranscriptSegment `json:"segments"`
	Model     string              `json:"model,omitempty"`
	Version   int                 `json:"version,omitempty"`
	EditedAt  time.Time           `json:"edited_at,omitzero"`
	CreatedAt time.Time           `json:"created_at,omitzero"`
}

func exportTranscript(tx *vbolt.Tx, imageId int) *ExportTranscript {
	t, ok := GetVideoTranscriptRecord(tx, imageId)
	if !ok || (t.Status != TranscriptDone && t.Status != TranscriptRemoved) {
		return nil
	}
	return &ExportTranscript{
		Automatic: t.EditedBy == 0, Removed: t.Status == TranscriptRemoved, Language: t.Language, Segments: t.Segments,
		Model: t.Model, Version: t.Version, EditedAt: t.EditedAt, CreatedAt: t.CreatedAt,
	}
}

func buildVideoExport(tx *vbolt.Tx, familyId int) []ExportVideo {
	result := []ExportVideo{}
	for _, image := range GetFamilyImages(tx, familyId) {
		if !image.IsVideo() || image.OriginalKey == "" || image.Status == PhotoProcessing {
			continue
		}
		video := ExportVideo{
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
			PersonIds:        []int{},
			TagIds:           []int{},
			Available:        image.Status == PhotoReady,
			CreatedAt:        image.CreatedAt,
			Transcript:       exportTranscript(tx, image.Id),
		}
		for _, pp := range GetPhotoPersonsByPhoto(tx, image.Id) {
			video.PersonIds = append(video.PersonIds, pp.PersonId)
			video.People = append(video.People, ExportPhotoPerson{PersonId: pp.PersonId, AutoTagged: pp.AutoTagged, CreatedAt: pp.CreatedAt})
		}
		for _, pt := range getPhotoTagRows(tx, image.Id) {
			video.TagIds = append(video.TagIds, pt.TagId)
			video.Tags = append(video.Tags, ExportPhotoTag{TagId: pt.TagId, CreatedAt: pt.CreatedAt})
		}
		result = append(result, video)
	}
	return result
}

func (v ExportVideo) zipPath() string { return "videos/" + v.FileName }

// importVideos adds the archive's videos whose files the zip carries. A video
// the family already has, by content hash, is linked instead. New videos are
// left processing, for the video worker once the import commits.
func importVideos(
	tx *vbolt.Tx,
	familyId int,
	ownerUserId int,
	videos []ExportVideo,
	l *archiveLedger,
	staged map[int]stagedMedia,
	dryRun bool,
) (imported, skipped int, queued []importedVideo, warnings []string) {
	store := activeMediaStore()
	_, localStore := store.(*localMediaStore)
	used := FamilyStorageUsage(tx, familyId)
	existing := familyMediaByHash(tx, familyId)
	var unavailable, overQuota int

	for _, video := range videos {
		if !inScope(video.PersonIds, l) {
			continue
		}
		file, ok := staged[video.Id]
		if !ok {
			skipped++
			continue
		}
		hash := file.hash
		if id, ok := takeMediaByHash(existing, hash); ok {
			l.photos[video.Id] = id
			importTranscript(tx, id, familyId, ownerUserId, video.Transcript)
			skipped++
			continue
		}

		ext, supported := videoExtensions[video.MimeType]
		if store == nil || !supported {
			unavailable++
			skipped++
			continue
		}
		size := file.size
		reservation := int64(videoReservationBytes(int(size)))
		if cfg.FamilyStorageQuotaBytes > 0 && used+reservation > cfg.FamilyStorageQuotaBytes {
			overQuota++
			skipped++
			continue
		}
		if localStore && !dryRun && CheckDiskHeadroom(cfg.MediaDir, reservation, cfg.MinFreeDiskBytes) != nil {
			overQuota++
			skipped++
			continue
		}
		mediaId, err := newMediaId()
		if err != nil {
			skipped++
			continue
		}
		key := "videos/" + mediaId + "/original" + ext
		used += reservation

		image := Image{
			Id:               vbolt.NextIntId(tx, ImagesBkt),
			FamilyId:         familyId,
			OwnerUserId:      ownerUserId,
			OriginalFilename: video.OriginalFilename,
			MimeType:         video.MimeType,
			FileSize:         int(size),
			ContentHash:      hash,
			Title:            video.Title,
			Description:      video.Description,
			DurationMs:       video.DurationMs,
			CreatedAt:        orNow(video.CreatedAt),
			Status:           PhotoProcessing,
			MediaKind:        MediaVideo,
			OriginalKey:      key,
			DerivedBytes:     int(size) + videoPosterAllowance,
		}
		recordTime, photoDate, err := importRecordTime(video.RecordTime, wallClockUTC(video.PhotoDate), l.people, false)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Video %q was imported without its date: %s", video.Title, err.Error()))
			recordTime, photoDate = unknownRecordTime(SourceLegacy), time.Time{}
		}
		image.RecordTime, image.PhotoDate = recordTime, photoDate
		writeImage(tx, &image)
		vbolt.SetTargetSingleTerm(tx, ImageByFamilyIndex, image.Id, familyId)
		ReindexPhotoDates(tx, image.Id)
		l.photos[video.Id] = image.Id

		importPhotoLinks(tx, image, ExportPhoto{
			PersonIds: video.PersonIds, TagIds: video.TagIds, People: video.People, Tags: video.Tags,
		}, l)
		importTranscript(tx, image.Id, familyId, ownerUserId, video.Transcript)
		queued = append(queued, importedVideo{
			id:   image.Id,
			file: stagedUpload{store: store, from: file.path, key: key, size: size, contentType: video.MimeType},
		})
		imported++
	}

	if unavailable > 0 {
		warnings = append(warnings, fmt.Sprintf("%d video(s) were not imported because this site can't store them.", unavailable))
	}
	if overQuota > 0 {
		warnings = append(warnings, fmt.Sprintf("Storage is full: %d video(s) were not imported. Everything else was.", overQuota))
	}
	return
}

// importTranscript carries a transcript as a person left it. One the family
// already has is kept unless the archive's is a person's edit and the
// family's isn't. Edited and removed transcripts come back settled, so the
// video worker leaves them alone.
func importTranscript(tx *vbolt.Tx, imageId, familyId, userId int, exported *ExportTranscript) {
	if exported == nil {
		return
	}
	existing, found := GetVideoTranscriptRecord(tx, imageId)
	if found && (exported.Automatic || transcriptSettled(existing)) {
		return
	}
	t := VideoTranscript{
		ImageId: imageId, FamilyId: familyId, Model: exported.Model, Version: exported.Version,
		Language: exported.Language, Status: TranscriptDone, Segments: exported.Segments,
		EditedAt: exported.EditedAt, CreatedAt: orNow(exported.CreatedAt),
	}
	if exported.Removed {
		t.Status, t.Segments = TranscriptRemoved, nil
	}
	if !exported.Automatic {
		t.EditedBy = userId
	}
	writeVideoTranscriptTx(tx, &t)
}

type ExportFamily struct {
	Name         string `json:"name"`
	VideoEnabled bool   `json:"videoEnabled,omitempty"`
}

func exportFamilySettings(tx *vbolt.Tx, familyId int) *ExportFamily {
	family := GetFamily(tx, familyId)
	return &ExportFamily{Name: family.Name, VideoEnabled: family.VideoEnabled}
}

// importFamilySettings fills in what the family hasn't set: a name still the
// default its creator was given, and video when the archive's family had it.
func importFamilySettings(tx *vbolt.Tx, familyId int, settings *ExportFamily) {
	if settings == nil {
		return
	}
	family := GetFamily(tx, familyId)
	changed := false
	name := strings.TrimSpace(settings.Name)
	if name != "" && family.Name != name && family.Name == GetUser(tx, family.CreatedBy).Name+"'s Family" {
		family.Name, changed = name, true
	}
	if settings.VideoEnabled && !family.VideoEnabled {
		family.VideoEnabled, changed = true, true
	}
	if changed {
		vbolt.Write(tx, FamiliesBkt, family.Id, &family)
	}
}
