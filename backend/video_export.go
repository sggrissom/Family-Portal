package backend

import (
	"fmt"
	"path"
	"strings"
	"time"

	"go.hasen.dev/vbeam"
)

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
	return fmt.Sprintf("%s_%d_%s", recordFileDate(image.RecordTime), image.Id, name)
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
