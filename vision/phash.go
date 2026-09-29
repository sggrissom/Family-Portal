package vision

import (
	"image"
	"math"
	"math/bits"

	"github.com/disintegration/imaging"
)

// DHash is a 64-bit difference hash: each bit says whether a pixel of the
// 9x8 grayscale thumbnail is brighter than its right-hand neighbour.
func DHash(img image.Image) uint64 {
	small := imaging.Resize(imaging.Grayscale(img), 9, 8, imaging.Box)
	var hash uint64
	for y := 0; y < 8; y++ {
		row := small.Pix[y*small.Stride:]
		for x := 0; x < 8; x++ {
			hash <<= 1
			if row[x*4] > row[(x+1)*4] {
				hash |= 1
			}
		}
	}
	return hash
}

func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

const qualitySampleSize = 512

// Quality holds scores in 0..1: Sharpness from the variance of the
// Laplacian, Exposure from mean brightness and clipped highlights/shadows.
type Quality struct {
	Sharpness float64
	Exposure  float64
	Score     float64
}

func MeasureQuality(img image.Image) Quality {
	b := img.Bounds()
	if b.Dx() > qualitySampleSize || b.Dy() > qualitySampleSize {
		img = imaging.Fit(img, qualitySampleSize, qualitySampleSize, imaging.Box)
	}
	gray := imaging.Grayscale(img)
	w, h := gray.Bounds().Dx(), gray.Bounds().Dy()
	if w < 3 || h < 3 {
		return Quality{}
	}
	lum := func(x, y int) float64 { return float64(gray.Pix[y*gray.Stride+x*4]) }

	var sum, sumSq float64
	n := 0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			l := lum(x-1, y) + lum(x+1, y) + lum(x, y-1) + lum(x, y+1) - 4*lum(x, y)
			sum += l
			sumSq += l * l
			n++
		}
	}
	mean := sum / float64(n)
	variance := sumSq/float64(n) - mean*mean

	var total float64
	clipped := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := lum(x, y)
			total += v
			if v < 8 || v > 247 {
				clipped++
			}
		}
	}
	pixels := float64(w * h)
	brightness := total / pixels
	clippedFrac := float64(clipped) / pixels

	q := Quality{
		Sharpness: variance / (variance + 150),
		Exposure:  (1 - math.Abs(brightness-128)/128) * (1 - math.Min(1, 2*clippedFrac)),
	}
	q.Score = 0.7*q.Sharpness + 0.3*q.Exposure
	return q
}
