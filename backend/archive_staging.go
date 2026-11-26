package backend

import (
	"archive/zip"
	"context"
	"errors"
	"family/cfg"
	"fmt"
	"io"
	"io/fs"
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

func newArchiveStagingDir() (string, error) {
	if err := os.MkdirAll(archiveStagingRoot, 0755); err != nil {
		return "", err
	}
	return os.MkdirTemp(archiveStagingRoot, "import-*")
}

// removeStrayStaging deletes the staging directories a crash left behind,
// except those in keep.
func removeStrayStaging(keep map[string]bool) {
	entries, err := os.ReadDir(archiveStagingRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		dir := filepath.Join(archiveStagingRoot, entry.Name())
		if !keep[filepath.Clean(dir)] {
			os.RemoveAll(dir)
		}
	}
}

// stageArchiveMedia unpacks the photos and videos the import will consider
// into dir, or only hashes them when dir is empty. A file that can't be read
// is left out, and the import skips its media. progress runs after each
// file; an error from it stops the staging.
func stageArchiveMedia(archive familyArchive, req ImportDataRequest, dir string, progress func(staged int) error) (staging *archiveStaging, err error) {
	staging = &archiveStaging{dir: dir, files: map[int]stagedMedia{}, scope: map[int]bool{}, progress: progress}
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
// Run again after it finished, it finds the photo in place and does nothing.
type stagedMove struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (m stagedMove) run() error {
	if _, err := os.Stat(m.From); errors.Is(err, fs.ErrNotExist) {
		if _, err := os.Stat(m.To); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(m.To), 0755); err != nil {
		return err
	}
	if os.Rename(m.From, m.To) == nil {
		return nil
	}
	in, err := os.Open(m.From)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(m.To)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(m.To)
		return errors.Join(errors.New("could not move a staged photo"), err)
	}
	return nil
}

// stagedUpload puts a staged video in the media store once the import has
// committed.
type stagedUpload struct {
	From        string `json:"from"`
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

func (u stagedUpload) run() error {
	store := activeMediaStore()
	if store == nil {
		return ErrArchiveImportUnavailable
	}
	in, err := os.Open(u.From)
	if err != nil {
		if _, statErr := store.Stat(context.Background(), u.Key); statErr == nil {
			return nil
		}
		return err
	}
	defer in.Close()
	return store.Put(context.Background(), u.Key, in, u.Size, u.ContentType)
}
