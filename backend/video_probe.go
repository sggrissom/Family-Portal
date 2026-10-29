package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type videoProbe struct {
	FormatName   string
	Duration     time.Duration
	VideoCodec   string
	VideoMime    string
	AudioCodec   string
	AudioMime    string
	Width        int
	Height       int
	Rotation     int
	HDR          bool
	CreatedAt    time.Time
	HasCreatedAt bool
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType      string            `json:"codec_type"`
		CodecName      string            `json:"codec_name"`
		CodecTagString string            `json:"codec_tag_string"`
		Profile        string            `json:"profile"`
		Level          int               `json:"level"`
		MimeCodec      string            `json:"mime_codec_string"`
		Width          int               `json:"width"`
		Height         int               `json:"height"`
		ColorTransfer  string            `json:"color_transfer"`
		Duration       string            `json:"duration"`
		Tags           map[string]string `json:"tags"`
		SideData       []struct {
			Rotation *float64 `json:"rotation"`
		} `json:"side_data_list"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		FormatName string            `json:"format_name"`
		Duration   string            `json:"duration"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
}

var errNotAVideo = errors.New("not a readable video")

func parseVideoProbe(data []byte) (videoProbe, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return videoProbe{}, fmt.Errorf("%w: %v", errNotAVideo, err)
	}
	p := videoProbe{FormatName: out.Format.FormatName}
	p.Duration = parseSeconds(out.Format.Duration)

	foundVideo, foundAudio := false, false
	for _, s := range out.Streams {
		switch {
		case s.CodecType == "video" && !foundVideo && s.Disposition.AttachedPic == 0:
			foundVideo = true
			p.VideoCodec = s.CodecName
			p.VideoMime = s.MimeCodec
			if p.VideoMime == "" || strings.HasPrefix(p.VideoMime, "hev1") {
				p.VideoMime = videoMimeCodec(s.CodecName, s.Profile, s.Level)
			}
			p.Width, p.Height = s.Width, s.Height
			for _, sd := range s.SideData {
				if sd.Rotation != nil {
					p.Rotation = normalizeRotation(int(*sd.Rotation))
				}
			}
			if r, err := strconv.Atoi(s.Tags["rotate"]); err == nil && p.Rotation == 0 {
				p.Rotation = normalizeRotation(r)
			}
			p.HDR = s.ColorTransfer == "arib-std-b67" || s.ColorTransfer == "smpte2084"
			if p.Duration == 0 {
				p.Duration = parseSeconds(s.Duration)
			}
		case s.CodecType == "audio" && !foundAudio:
			foundAudio = true
			p.AudioCodec = s.CodecName
			p.AudioMime = s.MimeCodec
			if p.AudioMime == "" {
				p.AudioMime = audioMimeCodec(s.CodecName, s.Profile)
			}
		}
	}
	if !foundVideo {
		return p, fmt.Errorf("%w: no video stream", errNotAVideo)
	}
	if p.Rotation == 90 || p.Rotation == 270 {
		p.Width, p.Height = p.Height, p.Width
	}
	p.CreatedAt, p.HasCreatedAt = videoCreationDate(out.Format.Tags)
	return p, nil
}

func parseSeconds(s string) time.Duration {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

func normalizeRotation(r int) int {
	r %= 360
	if r < 0 {
		r += 360
	}
	return r
}

// Apple writes the capture time with its local offset; that wall clock is
// what a photo date records. creation_time is a UTC instant, so it is only a
// fallback and may land a few hours off the local day.
func videoCreationDate(tags map[string]string) (time.Time, bool) {
	if raw := tags["com.apple.quicktime.creationdate"]; raw != "" {
		for _, layout := range []string{"2006-01-02T15:04:05-0700", time.RFC3339, "2006-01-02T15:04:05Z0700"} {
			if t, err := time.Parse(layout, raw); err == nil {
				return wallClockUTC(t), true
			}
		}
	}
	if raw := tags["creation_time"]; raw != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil && t.Year() > 1970 {
			return wallClockUTC(t.UTC()), true
		}
	}
	return time.Time{}, false
}

func videoMimeCodec(codec, profile string, level int) string {
	switch codec {
	case "h264":
		profileIdc := map[string]string{
			"Constrained Baseline": "42E0", "Baseline": "4200", "Main": "4D00",
			"Extended": "5800", "High": "6400", "High 10": "6E00",
		}[profile]
		if profileIdc == "" {
			profileIdc = "6400"
		}
		return fmt.Sprintf("avc1.%s%02X", profileIdc, level)
	case "hevc":
		profileId := map[string]string{"Main": "1.6", "Main 10": "2.4", "Main Still Picture": "3.E"}[profile]
		if profileId == "" {
			profileId = "1.6"
		}
		return fmt.Sprintf("hvc1.%s.L%d.B0", profileId, level)
	}
	return ""
}

func audioMimeCodec(codec, profile string) string {
	switch codec {
	case "aac":
		if strings.HasPrefix(profile, "HE-AACv2") {
			return "mp4a.40.29"
		}
		if strings.HasPrefix(profile, "HE-AAC") {
			return "mp4a.40.5"
		}
		return "mp4a.40.2"
	case "mp3":
		return "mp4a.6B"
	}
	return ""
}

var acceptedVideoCodecs = map[string]bool{"h264": true, "hevc": true}
var acceptedAudioCodecs = map[string]bool{"": true, "aac": true, "mp3": true}

// rejection is the reason, fit to show the uploader, that a probed file will
// not be kept, or "" when it is acceptable.
func (p videoProbe) rejection() string {
	if !strings.Contains(p.FormatName, "mp4") && !strings.Contains(p.FormatName, "mov") {
		return "This file isn't an MP4 or MOV video."
	}
	if !acceptedVideoCodecs[p.VideoCodec] {
		return fmt.Sprintf("This video uses the %s format, which isn't supported. Export it from your phone as H.264 or HEVC.", strings.ToUpper(p.VideoCodec))
	}
	if !acceptedAudioCodecs[p.AudioCodec] {
		return fmt.Sprintf("This video's sound uses the %s format, which isn't supported.", strings.ToUpper(p.AudioCodec))
	}
	if p.Duration <= 0 {
		return "This video's length couldn't be read."
	}
	if p.Duration > maxVideoDuration+time.Second {
		return fmt.Sprintf("Videos can be at most %d minutes long; this one is %d:%02d.",
			int(maxVideoDuration/time.Minute), int(p.Duration/time.Minute), int(p.Duration/time.Second)%60)
	}
	return ""
}

type mediaTools struct {
	ffmpeg  string
	ffprobe string
	nice    string
}

func findMediaTools() (mediaTools, error) {
	var tools mediaTools
	var err error
	if tools.ffmpeg, err = exec.LookPath("ffmpeg"); err != nil {
		return tools, err
	}
	if tools.ffprobe, err = exec.LookPath("ffprobe"); err != nil {
		return tools, err
	}
	tools.nice, _ = exec.LookPath("nice")
	return tools, nil
}

const maxToolStderr = 2000

// run executes a tool at low priority, bounded by timeout and by ctx.
func (t mediaTools) run(ctx context.Context, timeout time.Duration, tool string, args ...string) ([]byte, error) {
	out, _, err := t.runLogged(ctx, timeout, tool, args...)
	return out, err
}

func (t mediaTools) runLogged(ctx context.Context, timeout time.Duration, tool string, args ...string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	name := tool
	if t.nice != "" {
		args = append([]string{"-n", "10", tool}, args...)
		name = t.nice
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > maxToolStderr {
			msg = msg[len(msg)-maxToolStderr:]
		}
		return nil, "", fmt.Errorf("%s: %w: %s", tool, err, msg)
	}
	return stdout.Bytes(), stderr.String(), nil
}

const (
	probeTimeout  = time.Minute
	remuxTimeout  = 20 * time.Minute
	posterTimeout = 2 * time.Minute
)

func (t mediaTools) probe(ctx context.Context, path string) (videoProbe, error) {
	out, err := t.run(ctx, probeTimeout, t.ffprobe, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	if err != nil {
		if ctx.Err() != nil {
			return videoProbe{}, err
		}
		return videoProbe{}, fmt.Errorf("%w: %v", errNotAVideo, err)
	}
	return parseVideoProbe(out)
}

func (t mediaTools) remux(ctx context.Context, src, dst string, p videoProbe) error {
	args := []string{"-nostdin", "-v", "error", "-y", "-i", src, "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy"}
	if p.VideoCodec == "hevc" {
		args = append(args, "-tag:v", "hvc1")
	}
	args = append(args, "-movflags", "+faststart", "-f", "mp4", dst)
	_, err := t.run(ctx, remuxTimeout, t.ffmpeg, args...)
	return err
}

// preparePlayback copies H.264 losslessly and converts HEVC to an SDR H.264
// derivative. FFmpeg applies the display matrix before the filters, preserving
// portrait orientation. Limit encoder/filter threads on the shared VPS.
func (t mediaTools) preparePlayback(ctx context.Context, src, dst string, p videoProbe) error {
	if p.VideoCodec != "hevc" {
		return t.remux(ctx, src, dst, p)
	}
	filter := ""
	if p.HDR {
		filter = strings.Replace(hdrTonemap, "format=yuvj420p", "format=yuv420p", 1) + ","
	}
	filter += "scale=w='min(1920,iw)':h='min(1920,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2,setsar=1"
	args := []string{"-nostdin", "-v", "error", "-y", "-threads", "2", "-filter_threads", "1", "-i", src,
		"-map", "0:v:0", "-map", "0:a:0?", "-vf", filter,
		"-c:v", "libx264", "-preset", "fast", "-crf", "22", "-pix_fmt", "yuv420p", "-threads", "2",
		"-c:a", "aac", "-b:a", "160k", "-metadata:s:v:0", "rotate=0",
		"-movflags", "+faststart", "-f", "mp4", dst}
	_, err := t.run(ctx, time.Hour, t.ffmpeg, args...)
	return err
}

const hdrTonemap = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuvj420p"

func (t mediaTools) poster(ctx context.Context, src, dst string, p videoProbe) error {
	at := min(time.Second, p.Duration/2)
	grab := func(filter string) error {
		args := []string{"-nostdin", "-v", "error", "-y", "-ss", fmt.Sprintf("%.3f", at.Seconds()), "-i", src, "-frames:v", "1"}
		if filter != "" {
			args = append(args, "-vf", filter)
		}
		args = append(args, "-q:v", "2", dst)
		_, err := t.run(ctx, posterTimeout, t.ffmpeg, args...)
		return err
	}
	if p.HDR {
		if err := grab(hdrTonemap); err == nil || ctx.Err() != nil {
			return err
		}
	}
	return grab("")
}
