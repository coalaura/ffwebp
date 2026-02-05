package resize

import (
	"image"

	"golang.org/x/image/draw"
)

func Thumbnail(maxWidth, maxHeight uint, img image.Image) image.Image {
	if maxWidth == 0 && maxHeight == 0 {
		return img
	}

	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()

	if srcW <= int(maxWidth) && srcH <= int(maxHeight) {
		return img
	}

	var dstW, dstH int

	if maxWidth == 0 {
		dstH = int(maxHeight)
		dstW = srcW * dstH / srcH
	} else if maxHeight == 0 {
		dstW = int(maxWidth)
		dstH = srcH * dstW / srcW
	} else {
		scaleX := float64(maxWidth) / float64(srcW)
		scaleY := float64(maxHeight) / float64(srcH)

		scale := scaleX

		if scaleY < scaleX {
			scale = scaleY
		}

		dstW = int(float64(srcW) * scale)
		dstH = int(float64(srcH) * scale)
	}

	if dstW < 1 {
		dstW = 1
	}

	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return dst
}
