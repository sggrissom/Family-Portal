package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
)

type GetVideoPlaybackRequest struct {
	Id int `json:"id"`
}

type GetVideoPlaybackResponse struct {
	Status int `json:"status"`
	// PlaybackUrl is set once the video is ready. Like an upload URL it carries
	// its own authorization: request it without the app's credentials, and ask
	// for a new one when it expires.
	PlaybackUrl string `json:"playbackUrl"`
	// MimeType includes the codecs, for canPlayType.
	MimeType         string    `json:"mimeType"`
	DownloadUrl      string    `json:"downloadUrl"`
	OriginalFilename string    `json:"originalFilename"`
	ExpiresAt        time.Time `json:"expiresAt"`
	ProcessingError  string    `json:"processingError"`
}

var ErrVideoNotFound = errors.New("Video not found or access denied")

func playbackMimeType(image Image) string {
	codecs := image.VideoCodec
	if image.AudioCodec != "" {
		codecs += ", " + image.AudioCodec
	}
	if codecs == "" {
		return playbackContentType
	}
	return playbackContentType + `; codecs="` + codecs + `"`
}

func GetVideoPlayback(ctx *vbeam.Context, req GetVideoPlaybackRequest) (resp GetVideoPlaybackResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	image := GetImageById(ctx.Tx, req.Id)
	if image.Id == 0 || !image.IsVideo() || !CanAccessPhoto(ctx.Tx, user, image, AccessView) {
		err = ErrVideoNotFound
		return
	}

	resp.Status = image.Status
	resp.ProcessingError = image.ProcessingError
	resp.OriginalFilename = image.OriginalFilename
	resp.MimeType = playbackMimeType(image)

	store := activeMediaStore()
	if store == nil {
		return
	}
	if image.Status == 0 && image.PlaybackKey != "" {
		resp.PlaybackUrl, resp.ExpiresAt, err = store.PlaybackURL(image.PlaybackKey, MediaURLOptions{
			ContentType: playbackContentType, TTL: mediaPlaybackTTL,
		})
		if err != nil {
			return
		}
	}
	if image.OriginalKey != "" && image.Status != 1 {
		resp.DownloadUrl, resp.ExpiresAt, err = store.PlaybackURL(image.OriginalKey, MediaURLOptions{
			ContentType: image.MimeType, DownloadName: image.OriginalFilename, TTL: mediaPlaybackTTL,
		})
	}
	return
}
