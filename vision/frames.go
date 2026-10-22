package vision

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

const (
	MaxFrames      = 48
	FrameWidth     = 600
	minFrameGap    = 5 * time.Second
	sceneThreshold = 0.3
	sameFrame      = 0.95
)

func FrameInterval(duration time.Duration) time.Duration {
	return max(minFrameGap, duration/MaxFrames)
}

// FrameSelectFilter keeps the first frame, a frame every FrameInterval, and
// every scene cut. showinfo logs each kept frame's time to stderr, in the
// order ffmpeg writes them.
func FrameSelectFilter(duration time.Duration) string {
	return fmt.Sprintf("select='isnan(prev_selected_t)+gte(t-prev_selected_t,%.3f)+gt(scene,%.2f)',showinfo,scale=%d:-2",
		FrameInterval(duration).Seconds(), sceneThreshold, FrameWidth)
}

var showinfoFrame = regexp.MustCompile(`\bn:\s*\d+\s+pts:\s*-?\d+\s+pts_time:(-?[0-9.]+)`)

func ParseFrameTimes(stderr string) []time.Duration {
	var times []time.Duration
	for _, m := range showinfoFrame.FindAllStringSubmatch(stderr, -1) {
		seconds, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		times = append(times, time.Duration(seconds*float64(time.Second)))
	}
	return times
}

// ThinFrames picks at most limit indexes out of n, evenly spread and always
// including the first.
func ThinFrames(n, limit int) []int {
	keep := make([]int, 0, min(n, limit))
	if n <= limit {
		for i := range n {
			keep = append(keep, i)
		}
		return keep
	}
	for i := range limit {
		keep = append(keep, i*n/limit)
	}
	return keep
}

// DistinctFrames drops each frame that looks the same as the last one kept.
func DistinctFrames(vectors [][]float32) []int {
	var keep []int
	for i, v := range vectors {
		if len(keep) > 0 && Dot(v, vectors[keep[len(keep)-1]]) >= sameFrame {
			continue
		}
		keep = append(keep, i)
	}
	return keep
}
