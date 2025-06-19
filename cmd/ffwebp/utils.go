package main

import (
	"fmt"
	"image"
	"image/color"
)

func colorModel(img image.Image) string {
	model := img.ColorModel()

	switch model {
	case color.RGBAModel:
		return "RGBA"
	case color.RGBA64Model:
		return "RGBA64"
	case color.NRGBAModel:
		return "NRGBA"
	case color.NRGBA64Model:
		return "NRGBA64"
	case color.AlphaModel:
		return "Alpha"
	case color.Alpha16Model:
		return "Alpha16"
	case color.GrayModel:
		return "Gray"
	case color.Gray16Model:
		return "Gray16"
	}

	return fmt.Sprintf("%T", model)
}
