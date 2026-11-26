package backend

import (
	"archive/zip"
	"context"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// An Everything import unpacks and hashes its media here before its write
// transaction opens, so the transaction itself only writes records. The root
// sits beside the static directory so a staged photo moves into place with a
// rename.
var archiveStagingRoot = filepath.Join(filepath.Dir(filepath.Clean(cfg.StaticDir)), "import-staging")

// stagedMedia is one file from the archive, unpacked and hashed. A dry run
// hashes without unpacking, so path is empty, and so does a file the disk
// had no room for, which is marked noRoom.
type stagedMedia struct {
	path   string
	hash   string
	size   int64
	noRoom bool
}

// archiveStaging holds an import's staged photos and videos, by their id in
// the archive.
type archiveStaging struct {
	dir      string
	files    map[int]stagedMedia
	scope    map[int]bool
	progress func(staged int) error
}

// stageArchiveMedia unpacks the photos and videos the import will consider.
// A file that can't be read is left out, and the import skips its media.
// progress runs after each file; an error from it stops the staging.
func stageArchiveMedia(archive familyArchive, req ImportDataRequest, keep bool, progress func(staged int) error) (staging *archiveStaging, err error) {
	staging = &archiveStaging{files: map[int]stagedMedia{}, scope: map[int]bool{}, progress: progress}
	if keep {
		if err = os.MkdirAll(archiveStagingRoot, 0755); err != nil {
			return staging, err
		}
		if staging.dir, err = os.MkdirTemp(archiveStagingRoot, "import-*"); err != nil {
			return staging, err
		}
	}
	for _, p := range filterPeople(archive.People, req.FilterFamilyIds, req.FilterPersonIds) {
		staging.scope[p.Id] = true
	}
	for _, photo := range archive.Photos {
		if err = staging.stage(archive.media[photo.ZipPath], photo.Id, photo.PersonIds); err != nil {
			return staging, err
		}
	}
	for _, video := range archive.Videos {
		if err = staging.stage(archive.media[video.zipPath()], video.Id, video.PersonIds); err != nil {
			return staging, err
		}
	}
	return staging, nil
}

func (s *archiveStaging) stage(zf *zip.File, id int, personIds []int) error {
	if zf == nil || !s.inFilter(personIds) {
		return nil
	}
	if _, done := s.files[id]; done {
		return nil
	}
	staged := stagedMedia{size: int64(zf.UncompressedSize64)}
	if s.dir != "" {
		if diskHasRoom(s.dir, staged.size) {
			staged.path = filepath.Join(s.dir, fmt.Sprintf("%d%s", id, filepath.Ext(zf.Name)))
		} else {
			staged.noRoom = true
		}
	}
	hash, err := unpackZipEntry(zf.Open, staged.path)
	if err != nil {
		LogWarn(LogCategoryPhoto, "Import could not read a media file", map[string]interface{}{"path": zf.Name, "error": err.Error()})
		return nil
	}
	staged.hash = hash
	s.files[id] = staged
	if err := s.progress(len(s.files)); err != nil {
		s.cleanup()
		return err
	}
	return nil
}

// diskHasRoom is CheckDiskHeadroom without the log line, since an import
// asks once for every file.
func diskHasRoom(dir string, n int64) bool {
	free, err := freeDiskBytes(dir)
	return err != nil || free-diskBudget.reservedBytes()-n >= cfg.MinFreeDiskBytes
}

func (s *archiveStaging) inFilter(personIds []int) bool {
	for _, id := range personIds {
		if s.scope[id] {
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

// stagedMove puts a staged photo at its path once the import has committed.
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

// stagedUpload puts a staged video in the media store once the import has
// committed.
type stagedUpload struct {
	store       MediaStore
	from, key   string
	size        int64
	contentType string
}

func (u stagedUpload) run() error {
	in, err := os.Open(u.from)
	if err != nil {
		return err
	}
	defer in.Close()
	return u.store.Put(context.Background(), u.key, in, u.size, u.contentType)
}
