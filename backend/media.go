package backend

type MediaKind int

const (
	MediaPhoto MediaKind = iota
	MediaVideo
)

func (image Image) IsVideo() bool {
	return image.MediaKind == MediaVideo
}
