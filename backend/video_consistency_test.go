package backend

import (
	"context"
	"strings"
	"testing"

	"go.hasen.dev/vbolt"
)

func (fx videoFixture) storedVideo(t *testing.T, filename string, status int) Image {
	t.Helper()
	video := fx.addVideoAt(t, filename, featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		video.Status = status
		video.FileSize = len(filename)
		id := strings.ToLower(strings.TrimSuffix(filename, ".MOV"))
		video.OriginalKey = "videos/" + strings.ReplaceAll(id, "_", "") + "/original.mov"
		video.PlaybackKey = "videos/" + strings.ReplaceAll(id, "_", "") + "/playback.mp4"
		vbolt.Write(tx, ImagesBkt, video.Id, &video)
		vbolt.TxCommit(tx)
	})
	if err := fx.store.Put(context.Background(), video.OriginalKey, strings.NewReader(filename), int64(len(filename)), "video/quicktime"); err != nil {
		t.Fatal(err)
	}
	return video
}

func TestConsistencyCheckFindsMissingVideoFiles(t *testing.T) {
	fx := setupVideoFixture(t)
	present := fx.storedVideo(t, "IMG_0001.MOV", 0)
	gone := fx.storedVideo(t, "IMG_0002.MOV", 0)
	fx.store.Delete(context.Background(), gone.OriginalKey)
	fx.store.Put(context.Background(), present.PlaybackKey, strings.NewReader("x"), 1, "video/mp4")
	fx.store.Put(context.Background(), gone.PlaybackKey, strings.NewReader("x"), 1, "video/mp4")

	var checked int
	var missing []MissingOriginal
	vbolt.WithReadTx(fx.db, func(tx *vbolt.Tx) { checked, missing = scanVideoObjects(tx, fx.store) })
	if checked != 2 || len(missing) != 1 || missing[0].ImageId != gone.Id {
		t.Fatalf("checked %d, missing %+v", checked, missing)
	}
}
