package backend

import "time"

const (
	maxVideoBytes    = 1 << 30
	maxVideoDuration = 10 * time.Minute
)

type MediaKind int

const (
	MediaPhoto MediaKind = iota
	MediaVideo
)

func (image Image) IsVideo() bool {
	return image.MediaKind == MediaVideo
}
