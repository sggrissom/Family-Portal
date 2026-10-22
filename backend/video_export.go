package backend

import (
	"fmt"
	"path"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type ExportVideo struct {
	Id               int               `json:"id"`
	Title            string            `json:"title"`
	Description      string            `json:"description"`
	PhotoDate        time.Time         `json:"photo_date"`
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

// videoExportFileName is unique within a family, sorts by date, and keeps the
// camera's own name recognisable.
func videoExportFileName(image Image) string {
	name := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 32 {
			return '_'
		}
		return r
	}, image.OriginalFilename)
	if name == "" {
		name = "video" + path.Ext(image.OriginalKey)
	}
	return fmt.Sprintf("%s_%d_%s", image.PhotoDate.Format("2006-01-02"), image.Id, name)
}

func buildVideoExport(tx *vbolt.Tx, familyId int) []ExportVideo {
	result := []ExportVideo{}
	for _, image := range GetFamilyImages(tx, familyId) {
		if !image.IsVideo() || image.OriginalKey == "" || image.Status == 1 {
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
			OriginalFilename: image.OriginalFilename,
			FileName:         videoExportFileName(image),
			MimeType:         image.MimeType,
			FileSize:         image.FileSize,
			DurationMs:       image.DurationMs,
			PersonIds:        personIds,
			TagIds:           GetPhotoTagIds(tx, image.Id),
			Available:        image.Status == 0,
			Transcript:       exportTranscript(tx, image.Id),
		})
	}
	return result
}

type ListVideoOriginalsRequest struct {
	FamilyId int `json:"familyId,omitempty"`
}

type VideoOriginal struct {
	Video       ExportVideo `json:"video"`
	DownloadUrl string      `json:"downloadUrl"`
}

type ListVideoOriginalsResponse struct {
	Videos     []VideoOriginal `json:"videos"`
	TotalBytes int             `json:"totalBytes"`
	ExpiresAt  time.Time       `json:"expiresAt"`
}

// ListVideoOriginals is the manifest for saving every original. Its links
// expire; a client saving many files asks again, or asks GetVideoPlayback for
// one file, whenever a link has lapsed.
func ListVideoOriginals(ctx *vbeam.Context, req ListVideoOriginalsRequest) (resp ListVideoOriginalsResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	familyId, err := ResolveActingFamily(ctx.Tx, user, req.FamilyId, AccessView)
	if err != nil {
		return
	}
	resp.Videos = []VideoOriginal{}
	store := activeMediaStore()
	for _, video := range buildVideoExport(ctx.Tx, familyId) {
		item := VideoOriginal{Video: video}
		resp.TotalBytes += video.FileSize
		if store != nil {
			image := GetImageById(ctx.Tx, video.Id)
			var expires time.Time
			item.DownloadUrl, expires, err = store.PlaybackURL(image.OriginalKey, MediaURLOptions{
				ContentType: image.MimeType, DownloadName: video.FileName, TTL: mediaPlaybackTTL,
			})
			if err != nil {
				return
			}
			resp.ExpiresAt = expires
		}
		resp.Videos = append(resp.Videos, item)
	}
	return
}

type GetVideoOriginalLinkRequest struct {
	Id int `json:"id"`
}

type GetVideoOriginalLinkResponse struct {
	DownloadUrl string    `json:"downloadUrl"`
	FileName    string    `json:"fileName"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

func GetVideoOriginalLink(ctx *vbeam.Context, req GetVideoOriginalLinkRequest) (resp GetVideoOriginalLinkResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	image := GetImageById(ctx.Tx, req.Id)
	if image.Id == 0 || !image.IsVideo() || image.OriginalKey == "" || !CanAccessPhoto(ctx.Tx, user, image, AccessView) {
		err = ErrVideoNotFound
		return
	}
	store := activeMediaStore()
	if store == nil {
		err = ErrVideoUnavailable
		return
	}
	resp.FileName = videoExportFileName(image)
	resp.DownloadUrl, resp.ExpiresAt, err = store.PlaybackURL(image.OriginalKey, MediaURLOptions{
		ContentType: image.MimeType, DownloadName: resp.FileName, TTL: mediaPlaybackTTL,
	})
	return
}
