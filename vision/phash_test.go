package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/disintegration/imaging"
)

func checkerboard(w, h, cell int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(40)
			if (x/cell+y/cell)%2 == 0 {
				v = 215
			}
			img.Set(x, y, color.NRGBA{v, v, v, 255})
		}
	}
	return img
}

func gradient(w, h int, flip bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / w)
			if flip {
				v = 255 - v
			}
			img.Set(x, y, color.NRGBA{v, v / 2, 255 - v, 255})
		}
	}
	return img
}

func TestDHashSurvivesResizeAndRecompressionButNotContent(t *testing.T) {
	original := gradient(640, 480, false)
	for x := 100; x < 300; x++ {
		for y := 200; y < 260; y++ {
			original.Set(x, y, color.NRGBA{250, 250, 250, 255})
		}
	}
	resized := imaging.Resize(original, 320, 240, imaging.Lanczos)
	brighter := imaging.AdjustBrightness(original, 8)
	different := gradient(640, 480, true)

	h := DHash(original)
	if d := HammingDistance(h, DHash(resized)); d > 3 {
		t.Errorf("resized copy is %d bits away", d)
	}
	if d := HammingDistance(h, DHash(brighter)); d > 3 {
		t.Errorf("brightened copy is %d bits away", d)
	}
	if d := HammingDistance(h, DHash(different)); d < 20 {
		t.Errorf("different image is only %d bits away", d)
	}
}

func TestQualityPrefersSharpWellExposed(t *testing.T) {
	sharp := checkerboard(400, 300, 4)
	blurred := imaging.Blur(sharp, 4)
	dark := imaging.AdjustBrightness(sharp, -70)

	qs, qb, qd := MeasureQuality(sharp), MeasureQuality(blurred), MeasureQuality(dark)
	if qs.Sharpness <= qb.Sharpness || qs.Score <= qb.Score {
		t.Errorf("sharp %+v should beat blurred %+v", qs, qb)
	}
	if qs.Exposure <= qd.Exposure {
		t.Errorf("mid exposure %+v should beat dark %+v", qs, qd)
	}
	for _, q := range []Quality{qs, qb, qd} {
		if q.Score < 0 || q.Score > 1 || q.Sharpness < 0 || q.Sharpness > 1 || q.Exposure < 0 || q.Exposure > 1 {
			t.Errorf("score out of range: %+v", q)
		}
	}
	if got := MeasureQuality(image.NewNRGBA(image.Rect(0, 0, 2, 2))); got != (Quality{}) {
		t.Errorf("tiny image = %+v, want zero", got)
	}
}
