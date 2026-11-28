package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"go.hasen.dev/vbolt"
)

// mediaContentHash is the SHA-256 of a photo's or video's original file.
func mediaContentHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func bytesContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// familyMediaByHash lists a family's photos and videos by content hash. Each
// hash can name several, since a family can hold byte-identical copies.
func familyMediaByHash(tx *vbolt.Tx, familyId int) map[string][]int {
	byHash := map[string][]int{}
	for _, img := range GetFamilyImages(tx, familyId) {
		if img.ContentHash != "" {
			byHash[img.ContentHash] = append(byHash[img.ContentHash], img.Id)
		}
	}
	return byHash
}

// takeMediaByHash claims one of the family's media with this hash, so two
// identical files in one archive link to two existing copies, not one.
func takeMediaByHash(byHash map[string][]int, hash string) (int, bool) {
	ids := byHash[hash]
	if len(ids) == 0 {
		return 0, false
	}
	byHash[hash] = ids[1:]
	return ids[0], true
}

func fileContentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return mediaContentHash(f)
}

func openMediaOriginal(img Image) (io.ReadCloser, error) {
	if img.IsVideo() {
		store := activeMediaStore()
		if store == nil {
			return nil, ErrVideoUnavailable
		}
		return store.Open(context.Background(), img.OriginalKey)
	}
	return os.Open(photoOriginalPath(img.FilePath))
}

// BackfillMediaContentHashes hashes the photos and videos stored before
// images carried a hash. Files are read outside any transaction; media whose
// original can't be read is left without a hash and tried again next start.
func BackfillMediaContentHashes(db *vbolt.DB) (hashed int) {
	var pending []Image
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		vbolt.IterateAll(tx, ImagesBkt, func(id int, img Image) bool {
			if img.ContentHash == "" {
				pending = append(pending, img)
			}
			return true
		})
	})
	return hashMedia(db, pending)
}

// hashFamilyMedia hashes the family's media that has no hash yet, so an
// import can recognise it before the startup backfill reaches it.
func hashFamilyMedia(db *vbolt.DB, familyId int) (hashed int) {
	var pending []Image
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		for _, img := range GetFamilyImages(tx, familyId) {
			if img.ContentHash == "" {
				pending = append(pending, img)
			}
		}
	})
	return hashMedia(db, pending)
}

func hashMedia(db *vbolt.DB, pending []Image) (hashed int) {
	hashes := map[int]string{}
	flush := func() {
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			for id, hash := range hashes {
				current := GetImageById(tx, id)
				if current.Id != 0 && current.ContentHash == "" {
					current.ContentHash = hash
					saveImage(tx, &current)
					hashed++
				}
			}
			vbolt.TxCommit(tx)
		})
		clear(hashes)
	}
	for _, img := range pending {
		r, err := openMediaOriginal(img)
		if err != nil {
			continue
		}
		hash, err := mediaContentHash(r)
		r.Close()
		if err != nil {
			continue
		}
		if hashes[img.Id] = hash; len(hashes) >= 200 {
			flush()
		}
	}
	if len(hashes) > 0 {
		flush()
	}
	return
}
