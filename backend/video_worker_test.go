package backend

import (
	"bytes"
	"context"
	"errors"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func TestParseVideoProbe(t *testing.T) {
	modern := `{"streams":[
		{"codec_type":"video","codec_name":"h264","profile":"High","level":40,"mime_codec_string":"avc1.640028",
		 "width":1920,"height":1080,"side_data_list":[{"side_data_type":"Display Matrix","rotation":-90}]},
		{"codec_type":"audio","codec_name":"aac","profile":"LC","mime_codec_string":"mp4a.40.2"},
		{"codec_type":"data","codec_name":"none"}],
	 "format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"12.345",
		"tags":{"creation_time":"2024-07-04T22:30:00.000000Z","com.apple.quicktime.creationdate":"2024-07-04T15:30:00-0700"}}}`
	p, err := parseVideoProbe([]byte(modern))
	if err != nil {
		t.Fatal(err)
	}
	if p.VideoMime != "avc1.640028" || p.AudioMime != "mp4a.40.2" || p.Rotation != 270 || p.Width != 1080 || p.Height != 1920 {
		t.Fatalf("modern probe = %+v", p)
	}
	if p.Duration != 12345*time.Millisecond {
		t.Fatalf("duration %v", p.Duration)
	}
	if want := time.Date(2024, 7, 4, 15, 30, 0, 0, time.UTC); !p.HasCreatedAt || !p.CreatedAt.Equal(want) {
		t.Fatalf("created %v, want the local wall clock %v", p.CreatedAt, want)
	}
	if p.rejection() != "" {
		t.Fatalf("rejected: %s", p.rejection())
	}

	older := `{"streams":[
		{"codec_type":"video","codec_name":"hevc","profile":"Main 10","level":153,"codec_tag_string":"hev1",
		 "width":3840,"height":2160,"color_transfer":"arib-std-b67","tags":{"rotate":"90"}}],
	 "format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"3.0","tags":{"creation_time":"2023-01-02T03:04:05.000000Z"}}}`
	p, err = parseVideoProbe([]byte(older))
	if err != nil {
		t.Fatal(err)
	}
	if p.VideoMime != "hvc1.2.4.L153.B0" || p.AudioMime != "" || !p.HDR || p.Rotation != 90 || p.Width != 2160 {
		t.Fatalf("older probe = %+v", p)
	}
	if want := time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC); !p.CreatedAt.Equal(want) {
		t.Fatalf("creation_time fallback = %v", p.CreatedAt)
	}

	if _, err := parseVideoProbe([]byte(`{"streams":[{"codec_type":"audio","codec_name":"aac"}],"format":{}}`)); !errors.Is(err, errNotAVideo) {
		t.Fatalf("audio-only file: %v", err)
	}
	if got := videoMimeCodec("h264", "Main", 31); got != "avc1.4D001F" {
		t.Fatalf("derived h264 codec %q", got)
	}
}

func TestVideoProbeRejections(t *testing.T) {
	ok := videoProbe{FormatName: "mov,mp4,m4a,3gp,3g2,mj2", VideoCodec: "h264", AudioCodec: "aac", Duration: time.Minute}
	cases := map[string]func(*videoProbe){
		"container": func(p *videoProbe) { p.FormatName = "matroska,webm" },
		"codec":     func(p *videoProbe) { p.VideoCodec = "vp9" },
		"audio":     func(p *videoProbe) { p.AudioCodec = "pcm_s16le" },
		"too long":  func(p *videoProbe) { p.Duration = maxVideoDuration + 2*time.Second },
		"no length": func(p *videoProbe) { p.Duration = 0 },
	}
	for name, mutate := range cases {
		p := ok
		mutate(&p)
		if p.rejection() == "" {
			t.Errorf("%s: accepted", name)
		}
	}
	silent := ok
	silent.AudioCodec = ""
	if silent.rejection() != "" {
		t.Errorf("rejected a silent video: %s", silent.rejection())
	}
}

func requireFFmpeg(t *testing.T) mediaTools {
	t.Helper()
	tools, err := findMediaTools()
	if err != nil {
		t.Skip("ffmpeg/ffprobe not installed")
	}
	return tools
}

func makeClip(t *testing.T, tools mediaTools, name string, args ...string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command(tools.ffmpeg, append(append([]string{"-v", "error", "-y"}, args...), out)...)
	if msg, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(msg), "Unknown encoder") {
			t.Skipf("encoder not available: %s", msg)
		}
		t.Fatalf("making %s: %v: %s", name, err, msg)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type videoWorkerFixture struct {
	videoFixture
	worker *videoWorker
}

func setupVideoWorker(t *testing.T) videoWorkerFixture {
	t.Helper()
	tools := requireFFmpeg(t)
	fx := videoWorkerFixture{videoFixture: setupVideoFixture(t)}
	fx.worker = newVideoWorker(fx.db, tools, t.TempDir())
	t.Cleanup(fx.worker.cancel)
	return fx
}

func (fx videoWorkerFixture) upload(t *testing.T, filename, contentType string, data []byte) Image {
	t.Helper()
	begun, err := callAs(t, fx.resultsFixture, BeginVideoUpload, BeginVideoUploadRequest{
		PersonIds: []int{fx.alice.Id}, Filename: filename, ContentType: contentType,
		Size: len(data), FallbackDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	if code := fx.put(t, begun.Upload, data); code != http.StatusOK {
		t.Fatalf("PUT status %d", code)
	}
	done, err := callAs(t, fx.resultsFixture, CompleteVideoUpload, CompleteVideoUploadRequest{UploadId: begun.UploadId})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deletePhotoFiles(imageForTest(t, fx.db, done.Image.Id)) })
	return done.Image
}

func atomOffset(data []byte, name string) int {
	return bytes.Index(data, []byte(name))
}

func TestVideoWorkerRemuxesAndPublishes(t *testing.T) {
	fx := setupVideoWorker(t)
	plain := makeClip(t, fx.worker.tools, "plain.mov",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30", "-f", "lavfi", "-i", "sine=frequency=440",
		"-t", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-f", "mov")
	plainPath := filepath.Join(t.TempDir(), "plain.mov")
	if err := os.WriteFile(plainPath, plain, 0644); err != nil {
		t.Fatal(err)
	}
	clip := makeClip(t, fx.worker.tools, "IMG_0001.MOV",
		"-display_rotation", "90", "-i", plainPath, "-c", "copy", "-movflags", "use_metadata_tags",
		"-metadata", "com.apple.quicktime.creationdate=2024-07-04T15:30:00-0700", "-f", "mov")
	if atomOffset(clip, "moov") < atomOffset(clip, "mdat") {
		t.Fatal("test clip already has its moov first; it would not prove faststart")
	}

	video := fx.upload(t, "IMG_0001.MOV", "video/quicktime", clip)
	sourceKey := imageForTest(t, fx.db, video.Id).OriginalKey

	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	got := imageForTest(t, fx.db, video.Id)
	if got.Status != 0 || got.ProcessingError != "" {
		t.Fatalf("status %d error %q", got.Status, got.ProcessingError)
	}
	if !strings.HasPrefix(got.OriginalKey, "videos/") || !strings.HasSuffix(got.PlaybackKey, "/playback.mp4") {
		t.Fatalf("keys %q %q", got.OriginalKey, got.PlaybackKey)
	}
	if _, err := fx.store.Stat(context.Background(), sourceKey); !errors.Is(err, ErrMediaNotFound) {
		t.Fatal("upload source key still holds the original")
	}
	if !strings.HasPrefix(got.VideoCodec, "avc1.") || got.AudioCodec != "mp4a.40.2" {
		t.Fatalf("codecs %q %q", got.VideoCodec, got.AudioCodec)
	}
	if got.Width != 240 || got.Height != 320 {
		t.Fatalf("poster is %dx%d, want the rotated 240x320", got.Width, got.Height)
	}
	if got.DurationMs < 1900 || got.DurationMs > 2100 {
		t.Fatalf("duration %dms", got.DurationMs)
	}
	if want := time.Date(2024, 7, 4, 15, 30, 0, 0, time.UTC); !got.PhotoDate.Equal(want) || got.Title != "Video from Jul 4, 2024" {
		t.Fatalf("date %v title %q", got.PhotoDate, got.Title)
	}
	if got.DerivedBytes <= 0 || imageStoredBytes(got) != int64(got.FileSize+got.DerivedBytes) {
		t.Fatalf("accounting: derived %d", got.DerivedBytes)
	}
	if fx.usage(t) != imageStoredBytes(got) {
		t.Fatalf("family usage %d, want %d", fx.usage(t), imageStoredBytes(got))
	}
	if _, err := os.Stat(filepath.Join(cfg.StaticDir, "photos", mediaIdOf(got.OriginalKey)+"_thumb.jpg")); err != nil {
		t.Fatalf("poster thumbnail missing: %v", err)
	}

	r, err := fx.store.Open(context.Background(), got.PlaybackKey)
	if err != nil {
		t.Fatal(err)
	}
	var remux bytes.Buffer
	remux.ReadFrom(r)
	r.Close()
	if atomOffset(remux.Bytes(), "moov") > atomOffset(remux.Bytes(), "mdat") {
		t.Fatal("playback file does not start with its moov atom")
	}

	playback, err := callAs(t, fx.resultsFixture, GetVideoPlayback, GetVideoPlaybackRequest{Id: video.Id})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(playback.MimeType, `video/mp4; codecs="avc1.`) || playback.PlaybackUrl == "" || playback.DownloadUrl == "" {
		t.Fatalf("playback = %+v", playback)
	}
	req := httptest.NewRequest(http.MethodGet, playback.PlaybackUrl, nil)
	req.Header.Set("Range", "bytes=0-99")
	rec := httptest.NewRecorder()
	fx.store.serveObject(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 100 {
		t.Fatalf("ranged playback: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	rec = httptest.NewRecorder()
	fx.store.serveObject(rec, httptest.NewRequest(http.MethodGet, playback.DownloadUrl, nil))
	if !bytes.Equal(rec.Body.Bytes(), clip) || !strings.Contains(rec.Header().Get("Content-Disposition"), "IMG_0001.MOV") {
		t.Fatal("download did not return the untouched original")
	}

	if imageNeedsAnalysis(got) {
		t.Fatal("published video needs face analysis")
	}
}

func TestVideoWorkerConvertsHEVCForBrowsers(t *testing.T) {
	fx := setupVideoWorker(t)
	clip := makeClip(t, fx.worker.tools, "hevc.mp4",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30", "-t", "1",
		"-c:v", "libx265", "-pix_fmt", "yuv420p", "-tag:v", "hev1", "-x265-params", "log-level=error:pools=1")
	video := fx.upload(t, "hevc.mp4", "video/mp4", clip)
	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	got := imageForTest(t, fx.db, video.Id)
	if got.Status != 0 || !strings.HasPrefix(got.VideoCodec, "avc1.") || got.AudioCodec != "" {
		t.Fatalf("status %d codecs %q %q (%s)", got.Status, got.VideoCodec, got.AudioCodec, got.ProcessingError)
	}
	r, _ := fx.store.Open(context.Background(), got.PlaybackKey)
	var remux bytes.Buffer
	remux.ReadFrom(r)
	r.Close()
	original, err := fx.store.Open(context.Background(), got.OriginalKey)
	if err != nil {
		t.Fatal(err)
	}
	var originalBytes bytes.Buffer
	originalBytes.ReadFrom(original)
	original.Close()
	if !bytes.Equal(originalBytes.Bytes(), clip) {
		t.Fatal("original was changed")
	}
	if videoNeedsRetry(got) {
		t.Fatal("converted video should not be queued again")
	}
	got.VideoCodec = "hvc1.1.6.L90.B0"
	if !videoNeedsRetry(got) {
		t.Fatal("legacy HEVC video should be eligible for reprocessing")
	}
	if !bytes.Contains(remux.Bytes(), []byte("avc1")) || bytes.Contains(remux.Bytes(), []byte("hev1")) {
		t.Fatal("HEVC playback file was not converted to H.264")
	}
}

func TestVideoWorkerRejectsUnusableFiles(t *testing.T) {
	fx := setupVideoWorker(t)
	tools := fx.worker.tools
	cases := []struct {
		name, filename, contentType string
		data                        []byte
		reason                      string
	}{
		{"not a video", "notes.mov", "video/quicktime", []byte("this is plainly not a video file"), "couldn't be read"},
		{"too long", "long.mp4", "video/mp4", makeClip(t, tools, "long.mp4",
			"-f", "lavfi", "-i", "testsrc=size=32x32:rate=1", "-t", "605", "-c:v", "libx264", "-preset", "ultrafast"), "at most 10 minutes"},
		{"old codec", "old.mp4", "video/mp4", makeClip(t, tools, "old.mp4",
			"-f", "lavfi", "-i", "testsrc=size=64x48:rate=10", "-t", "1", "-c:v", "mpeg4"), "MPEG4"},
	}
	for _, c := range cases {
		video := fx.upload(t, c.filename, c.contentType, c.data)
		if err := fx.worker.process(video.Id); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := imageForTest(t, fx.db, video.Id)
		if got.Status != 2 || !strings.Contains(got.ProcessingError, c.reason) || got.OriginalKey != "" || imageStoredBytes(got) != 0 {
			t.Fatalf("%s: status %d, error %q, key %q, stored %d", c.name, got.Status, got.ProcessingError, got.OriginalKey, imageStoredBytes(got))
		}
	}
	sweepMedia(fx.db, fx.store, time.Now())
	var leftovers []string
	filepath.WalkDir(fx.store.root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) > 0 {
		t.Fatalf("rejected files still stored: %v", leftovers)
	}
	if fx.usage(t) != 0 {
		t.Fatalf("rejected videos still count %d bytes", fx.usage(t))
	}
}

func TestVideoDeletedDuringProcessingIsNotPublished(t *testing.T) {
	fx := setupVideoWorker(t)
	clip := makeClip(t, fx.worker.tools, "clip.mp4",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	video := fx.upload(t, "clip.mp4", "video/mp4", clip)
	mediaId := mediaIdOf(imageForTest(t, fx.db, video.Id).OriginalKey)

	videoBeforePublish = func(id int) {
		if _, err := callAs(t, fx.resultsFixture, DeletePhoto, DeletePhotoRequest{Id: id}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { videoBeforePublish = nil })

	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	sweepMedia(fx.db, fx.store, time.Now())

	if imageForTest(t, fx.db, video.Id).Id != 0 {
		t.Fatal("deleted video came back")
	}
	var leftovers []string
	filepath.WalkDir(fx.store.root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) > 0 {
		t.Fatalf("outputs of a deleted video still stored: %v", leftovers)
	}
	if matches, _ := filepath.Glob(filepath.Join(cfg.StaticDir, "photos", mediaId+"*")); len(matches) > 0 {
		t.Fatalf("poster files left: %v", matches)
	}
}

func TestVideoWorkerShutdownLeavesWorkForRestart(t *testing.T) {
	fx := setupVideoWorker(t)
	clip := makeClip(t, fx.worker.tools, "clip.mp4",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=10", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	video := fx.upload(t, "clip.mp4", "video/mp4", clip)
	photo := fx.addPhotoAt(t, "still.jpg", featuresEpoch)

	fx.worker.cancel()
	if err := fx.worker.process(video.Id); err != nil {
		t.Fatal(err)
	}
	got := imageForTest(t, fx.db, video.Id)
	if got.Status != 1 || got.ProcessingError != "" {
		t.Fatalf("shutdown marked the video %d %q", got.Status, got.ProcessingError)
	}

	pending := videosNeedingProcessing(fx.db)
	if len(pending) != 1 || pending[0] != video.Id {
		t.Fatalf("restart sweep = %v, want only video %d (not photo %d)", pending, video.Id, photo.Id)
	}

	restarted := newVideoWorker(fx.db, fx.worker.tools, t.TempDir())
	t.Cleanup(restarted.cancel)
	if err := restarted.process(video.Id); err != nil {
		t.Fatal(err)
	}
	if got := imageForTest(t, fx.db, video.Id); got.Status != 0 {
		t.Fatalf("after restart: status %d %q", got.Status, got.ProcessingError)
	}
}

func TestVideoWorkerWaitsForDisk(t *testing.T) {
	fx := setupVideoWorker(t)
	video := fx.upload(t, "clip.mov", "video/quicktime", []byte("bytes"))
	freeDiskBytes = func(string) (int64, error) { return 1 << 20, nil }

	if err := fx.worker.process(video.Id); !errors.Is(err, errRetryLater) {
		t.Fatalf("process with a full disk = %v, want a retry", err)
	}
	if got := imageForTest(t, fx.db, video.Id); got.Status != 1 {
		t.Fatalf("status %d", got.Status)
	}
	if diskBudget.reservedBytes() != 0 {
		t.Fatalf("leaked %d reserved bytes", diskBudget.reservedBytes())
	}
}

func TestVideoPlaybackAccess(t *testing.T) {
	fx := setupVideoFixture(t)
	video := fx.addVideoAt(t, "clip.mov", featuresEpoch)
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		video.Status = 1
		vbolt.Write(tx, ImagesBkt, video.Id, &video)
		vbolt.TxCommit(tx)
	})

	var stranger User
	vbolt.WithWriteTx(fx.db, func(tx *vbolt.Tx) {
		stranger = AddUserTx(tx, CreateAccountRequest{Name: "Stranger", Email: "stranger-playback@example.com"}, nil)
		vbolt.TxCommit(tx)
	})
	var err error
	fx.asUser(t, stranger, func(ctx *vbeam.Context) {
		_, err = GetVideoPlayback(ctx, GetVideoPlaybackRequest{Id: video.Id})
	})
	if !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("stranger got playback: %v", err)
	}

	photo := fx.addPhotoAt(t, "still.jpg", featuresEpoch)
	if _, err := callAs(t, fx.resultsFixture, GetVideoPlayback, GetVideoPlaybackRequest{Id: photo.Id}); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("playback for a photo: %v", err)
	}

	resp, err := callAs(t, fx.resultsFixture, GetVideoPlayback, GetVideoPlaybackRequest{Id: video.Id})
	if err != nil || resp.PlaybackUrl != "" || resp.DownloadUrl != "" {
		t.Fatalf("processing video handed out URLs: %+v %v", resp, err)
	}
}

func TestHEVCPlaybackHDRAndRotation(t *testing.T) {
	tools := requireFFmpeg(t)
	for _, hdr := range []bool{false, true} {
		name := "SDR"
		if hdr {
			name = "HDR"
		}
		t.Run(name, func(t *testing.T) {
			args := []string{"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=10",
				"-f", "lavfi", "-i", "sine=frequency=440", "-t", "0.5",
				"-c:v", "libx265", "-x265-params", "pools=1:log-level=error",
				"-pix_fmt", "yuv420p10le", "-c:a", "aac"}
			if hdr {
				args = append(args, "-color_primaries", "bt2020", "-color_trc", "arib-std-b67", "-colorspace", "bt2020nc")
			}
			data := makeClip(t, tools, "source.mp4", args...)
			src := filepath.Join(t.TempDir(), "source.mp4")
			if err := os.WriteFile(src, data, 0600); err != nil {
				t.Fatal(err)
			}
			// Set a portrait display matrix using stream copy.
			rotated := makeClip(t, tools, "portrait.mp4", "-display_rotation", "90", "-i", src, "-c", "copy")
			if err := os.WriteFile(src, rotated, 0600); err != nil {
				t.Fatal(err)
			}
			p, err := tools.probe(context.Background(), src)
			if err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(t.TempDir(), "playback.mp4")
			if err := tools.preparePlayback(context.Background(), src, dst, p); err != nil {
				t.Fatal(err)
			}
			got, err := tools.probe(context.Background(), dst)
			if err != nil {
				t.Fatal(err)
			}
			if got.VideoCodec != "h264" || got.AudioCodec != "aac" || got.HDR || got.Width != 240 || got.Height != 320 || got.Rotation != 0 {
				t.Fatalf("incompatible playback: %+v (source %+v)", got, p)
			}
		})
	}
}
