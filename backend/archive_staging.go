package backend

import (
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// An Everything import unpacks and hashes its media here before its write
// transaction opens, so the transaction itself only writes records. The root
// sits beside the static directory so a staged file moves into place with a
// rename.
var archiveStagingRoot = filepath.Join(filepath.Dir(filepath.Clean(cfg.StaticDir)), "import-staging")

// stagedMedia is one file from the archive, unpacked and hashed. A dry run
// hashes without unpacking, so path is empty.
type stagedMedia struct {
	path string
	hash string
	size int64
}

type archiveStaging struct {
	dir   string
	files map[int]stagedMedia
}

// stageArchiveMedia unpacks the photos the import will consider, keyed by
// their id in the archive. A file that can't be read is left out, and the
// import skips its photo. progress runs after each file; an error from it
// stops the staging.
func stageArchiveMedia(archive familyArchive, req ImportDataRequest, keep bool, progress func(staged int) error) (staging *archiveStaging, err error) {
	staging = &archiveStaging{files: map[int]stagedMedia{}}
	if keep {
		if err = os.MkdirAll(archiveStagingRoot, 0755); err != nil {
			return staging, err
		}
		if staging.dir, err = os.MkdirTemp(archiveStagingRoot, "import-*"); err != nil {
			return staging, err
		}
	}
	scope := map[int]bool{}
	for _, p := range filterPeople(archive.People, req.FilterFamilyIds, req.FilterPersonIds) {
		scope[p.Id] = true
	}
	for _, photo := range archive.Photos {
		zf := archive.media[photo.ZipPath]
		if zf == nil || !photoInFilter(photo.PersonIds, scope) {
			continue
		}
		if _, done := staging.files[photo.Id]; done {
			continue
		}
		staged := stagedMedia{size: int64(zf.UncompressedSize64)}
		if keep {
			staged.path = filepath.Join(staging.dir, fmt.Sprintf("%d%s", photo.Id, filepath.Ext(photo.ZipPath)))
		}
		if staged.hash, err = unpackZipEntry(zf.Open, staged.path); err != nil {
			LogWarn(LogCategoryPhoto, "Import could not read a photo", map[string]interface{}{"path": photo.ZipPath, "error": err.Error()})
			err = nil
			continue
		}
		staging.files[photo.Id] = staged
		if err = progress(len(staging.files)); err != nil {
			staging.cleanup()
			return staging, err
		}
	}
	return staging, nil
}

func photoInFilter(personIds []int, scope map[int]bool) bool {
	for _, id := range personIds {
		if scope[id] {
			return true
		}
	}
	return len(personIds) == 0
}

// unpackZipEntry hashes an entry and, when path is set, writes it there.
func unpackZipEntry(open func() (io.ReadCloser, error), path string) (string, error) {
	rc, err := open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	if path == "" {
		return mediaContentHash(rc)
	}
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	hash, err := mediaContentHash(io.TeeReader(rc, out))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return hash, err
}

func (s *archiveStaging) cleanup() {
	if s != nil && s.dir != "" {
		os.RemoveAll(s.dir)
	}
}

// stagedMove puts one staged file at its photo's path once the import has
// committed.
type stagedMove struct {
	from, to string
}

func (m stagedMove) run() error {
	if err := os.MkdirAll(filepath.Dir(m.to), 0755); err != nil {
		return err
	}
	if os.Rename(m.from, m.to) == nil {
		return nil
	}
	in, err := os.Open(m.from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(m.to)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(m.to)
		return errors.Join(errors.New("could not move a staged photo"), err)
	}
	return nil
}
