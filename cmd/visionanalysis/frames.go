//go:build visionanalysis

package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"family/vision"

	"github.com/disintegration/imaging"
)

var videoExts = map[string]bool{".mov": true, ".mp4": true, ".m4v": true}

func probeDuration(path string) (time.Duration, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return time.Duration(seconds * float64(time.Second)), err
}

func sampleFrames(path, dir string, duration time.Duration) ([]string, []time.Duration, error) {
	cmd := exec.Command("ffmpeg", "-nostdin", "-hide_banner", "-v", "info", "-i", path, "-an",
		"-vf", vision.FrameSelectFilter(duration, ""), "-fps_mode", "vfr", "-q:v", "4",
		filepath.Join(dir, "frame-%03d.jpg"))
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("ffmpeg: %v: %s", err, stderr[max(0, len(stderr)-400):])
	}
	times := vision.ParseFrameTimes(string(stderr))
	files := make([]string, len(times))
	for i := range times {
		files[i] = filepath.Join(dir, fmt.Sprintf("frame-%03d.jpg", i+1))
	}
	return files, times, nil
}

func posterFrame(path, dir string, duration time.Duration) (string, error) {
	at := min(time.Second, duration/2)
	dst := filepath.Join(dir, "poster.jpg")
	out, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y", "-ss", fmt.Sprintf("%.3f", at.Seconds()),
		"-i", path, "-frames:v", "1", "-q:v", "2", dst).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("poster: %v: %s", err, out)
	}
	return dst, nil
}

func embedFile(path string) ([]float32, error) {
	img, err := imaging.Open(path, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	return models.EmbedImage(imaging.Fit(img, vision.FrameWidth, vision.FrameWidth, imaging.Lanczos))
}

func runFrameBench(dir, promptsPath string) error {
	prompts, err := readPrompts(promptsPath)
	if err != nil {
		return err
	}
	var textVecs [][]float32
	if len(prompts) > 0 {
		phrases := make([]string, len(prompts))
		for i, p := range prompts {
			phrases[i] = p.Phrase
		}
		if textVecs, err = models.EmbedClipTexts(phrases); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !videoExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		if err := benchVideo(filepath.Join(dir, e.Name()), prompts, textVecs); err != nil {
			fmt.Printf("%s: %v\n", e.Name(), err)
		}
	}
	fmt.Printf("\npeak rss %s\n", procStatus("VmHWM"))
	return nil
}

func benchVideo(path string, prompts []benchPrompt, textVecs [][]float32) error {
	duration, err := probeDuration(path)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "frames-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	start := time.Now()
	files, times, err := sampleFrames(path, work, duration)
	if err != nil {
		return err
	}
	sampled := time.Since(start)
	picked := vision.ThinFrames(len(files), vision.MaxFrames)

	start = time.Now()
	var vectors [][]float32
	var offsets []time.Duration
	for _, i := range picked {
		v, err := embedFile(files[i])
		if err != nil {
			return err
		}
		vectors = append(vectors, v)
		offsets = append(offsets, times[i])
	}
	embedded := time.Since(start)
	kept := vision.DistinctFrames(vectors)

	posterPath, err := posterFrame(path, work, duration)
	if err != nil {
		return err
	}
	poster, err := embedFile(posterPath)
	if err != nil {
		return err
	}

	fmt.Printf("\n%s  %v  sampled %d in %v, embedded %d in %v, kept %d distinct\n",
		filepath.Base(path), duration.Round(time.Second), len(files), sampled.Round(time.Millisecond),
		len(picked), embedded.Round(time.Millisecond), len(kept))

	for p, prompt := range prompts {
		best, bestAt, won := -1.0, time.Duration(0), 0
		for _, k := range kept {
			cos := float64(vision.Dot(vectors[k], textVecs[p]))
			if cos > best {
				best, bestAt = cos, offsets[k]
			}
			if winsFrame(vectors[k], textVecs, p) {
				won++
			}
		}
		fmt.Printf("  %-14s poster %.3f  best frame %.3f at %v  wins %d/%d frames\n",
			prompt.Label, vision.Dot(poster, textVecs[p]), best, bestAt.Round(100*time.Millisecond), won, len(kept))
	}
	return nil
}

func winsFrame(frame []float32, textVecs [][]float32, p int) bool {
	type scored struct {
		i   int
		cos float64
	}
	scores := make([]scored, len(textVecs))
	var denom float64
	for i, t := range textVecs {
		scores[i] = scored{i, float64(vision.Dot(frame, t))}
		denom += math.Exp(100 * scores[i].cos)
	}
	sort.Slice(scores, func(a, b int) bool { return scores[a].cos > scores[b].cos })
	top := scores[0]
	return top.i == p && math.Exp(100*top.cos)/denom > 0.5 && top.cos >= 0.24
}
