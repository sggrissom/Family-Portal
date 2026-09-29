package vision

import (
	"image"

	"github.com/disintegration/imaging"
)

const ClipImageSize = 224

var (
	clipMean = [3]float32{0.48145466, 0.4578275, 0.40821073}
	clipStd  = [3]float32{0.26862954, 0.26130258, 0.27577711}
)

// ClipPixels resizes the shortest edge to 224 (bicubic), center-crops to
// 224x224, and returns normalized CHW float32 pixel values.
func ClipPixels(img image.Image) []float32 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < h {
		img = imaging.Resize(img, ClipImageSize, 0, imaging.CatmullRom)
	} else {
		img = imaging.Resize(img, 0, ClipImageSize, imaging.CatmullRom)
	}
	cropped := imaging.CropCenter(img, ClipImageSize, ClipImageSize)

	const plane = ClipImageSize * ClipImageSize
	out := make([]float32, 3*plane)
	for y := 0; y < ClipImageSize; y++ {
		row := cropped.Pix[y*cropped.Stride:]
		for x := 0; x < ClipImageSize; x++ {
			for c := 0; c < 3; c++ {
				v := float32(row[x*4+c]) / 255
				out[c*plane+y*ClipImageSize+x] = (v - clipMean[c]) / clipStd[c]
			}
		}
	}
	return out
}
