package main

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/ericpauley/go-quantize/quantize"
)

func Quantize(img image.Image) image.Image {
	if opts.NumColors == 0 {
		return img
	}

	bounds := img.Bounds()

	q := quantize.MedianCutQuantizer{}
	p := q.Quantize(make([]color.Color, 0, opts.NumColors), img)

	paletted := image.NewPaletted(bounds, p)

	draw.Draw(paletted, bounds, img, image.Point{}, draw.Src)

	return paletted
}
