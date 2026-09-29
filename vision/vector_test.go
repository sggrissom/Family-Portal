package vision

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestNormalizeAndDot(t *testing.T) {
	v := Normalize([]float32{3, 4})
	if math.Abs(float64(Dot(v, v))-1) > 1e-6 {
		t.Errorf("normalized self-dot = %v", Dot(v, v))
	}
	if got := Normalize([]float32{0, 0}); got[0] != 0 || got[1] != 0 {
		t.Errorf("zero vector changed: %v", got)
	}
	if Dot([]float32{1}, []float32{1, 2}) != 0 {
		t.Error("mismatched lengths should score 0")
	}
}

func TestMeanPoolIgnoresPadding(t *testing.T) {
	hidden := []float32{1, 2, 3, 4, 100, 100}
	got := MeanPool(hidden, []int64{1, 1, 0}, 2)
	if got[0] != 2 || got[1] != 3 {
		t.Errorf("got %v, want [2 3]", got)
	}
}

func TestClipPixelsShapeAndNormalization(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	px := ClipPixels(img)
	const plane = ClipImageSize * ClipImageSize
	if len(px) != 3*plane {
		t.Fatalf("len = %d", len(px))
	}
	wantR := (1 - clipMean[0]) / clipStd[0]
	wantG := (0 - clipMean[1]) / clipStd[1]
	if math.Abs(float64(px[plane/2]-wantR)) > 1e-4 || math.Abs(float64(px[plane+plane/2]-wantG)) > 1e-4 {
		t.Errorf("center pixel = %v,%v want %v,%v", px[plane/2], px[plane+plane/2], wantR, wantG)
	}
}
