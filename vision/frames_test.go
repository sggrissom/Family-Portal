package vision

import (
	"slices"
	"testing"
	"time"
)

func TestFrameIntervalCapsLongClips(t *testing.T) {
	for _, c := range []struct{ duration, want time.Duration }{
		{3 * time.Second, 5 * time.Second},
		{2 * time.Minute, 5 * time.Second},
		{10 * time.Minute, 12500 * time.Millisecond},
	} {
		if got := FrameInterval(c.duration); got != c.want {
			t.Errorf("FrameInterval(%v) = %v, want %v", c.duration, got, c.want)
		}
	}
}

func TestParseFrameTimesReadsShowinfo(t *testing.T) {
	stderr := `[Parsed_showinfo_1 @ 0x5581] config in time_base: 1/600, frame_rate: 30/1
[Parsed_showinfo_1 @ 0x5581] n:   0 pts:      0 pts_time:0       duration:     20 fmt:yuv420p
[Parsed_showinfo_1 @ 0x5581] color_range:tv color_space:bt709
[Parsed_showinfo_1 @ 0x5581] n:   1 pts:   3000 pts_time:5       duration:     20 fmt:yuv420p
[Parsed_showinfo_1 @ 0x5581] n:   2 pts:   4410 pts_time:7.35    duration:     20 fmt:yuv420p`
	want := []time.Duration{0, 5 * time.Second, 7350 * time.Millisecond}
	if got := ParseFrameTimes(stderr); !slices.Equal(got, want) {
		t.Errorf("ParseFrameTimes() = %v, want %v", got, want)
	}
}

func TestThinFramesSpreadsEvenly(t *testing.T) {
	if got := ThinFrames(3, 48); !slices.Equal(got, []int{0, 1, 2}) {
		t.Errorf("ThinFrames(3, 48) = %v", got)
	}
	if got := ThinFrames(10, 4); !slices.Equal(got, []int{0, 2, 5, 7}) {
		t.Errorf("ThinFrames(10, 4) = %v", got)
	}
}

func TestDistinctFramesDropsRepeatsOfTheLastKept(t *testing.T) {
	a := Normalize([]float32{1, 0, 0})
	nearA := Normalize([]float32{1, 0.1, 0})
	b := Normalize([]float32{0, 1, 0})
	if got := DistinctFrames([][]float32{a, nearA, a, b, b, a}); !slices.Equal(got, []int{0, 3, 5}) {
		t.Errorf("DistinctFrames() = %v, want [0 3 5]", got)
	}
}
