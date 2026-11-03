package codec

import (
	"fmt"
	"image"
	"image/color"
)

// ProfiledImage associates decoded samples with their color space until the
// processing pipeline separates metadata from the concrete pixel storage.
type ProfiledImage struct {
	image.Image
	ICC      []byte
	Channels int
}

func ValidateICC(channels int, profile []byte) error {
	if len(profile) == 0 {
		return nil
	}

	if len(profile) < 128 || string(profile[36:40]) != "acsp" {
		return fmt.Errorf("invalid ICC profile header")
	}

	space := "RGB "

	if channels == 1 || channels == 2 {
		space = "GRAY"
	}

	if string(profile[16:20]) != space {
		return fmt.Errorf("ICC color space %q does not describe the output %s channels; convert the source profile and pixels to RGB together before processing", profile[16:20], space)
	}

	return nil
}

func ImageChannels(img image.Image) int {
	switch img.(type) {
	case *image.Gray, *image.Gray16:
		return 1
	default:
		return 4
	}
}

// GrayImage restores logical grayscale after operations using RGB storage.
// It checks every channel before discarding any, including transparent samples.
func GrayImage(img image.Image, alpha bool) (image.Image, error) {
	if ImageChannels(img) == 1 {
		return img, nil
	}

	bounds := img.Bounds()
	wide := img.ColorModel() == color.NRGBA64Model || img.ColorModel() == color.RGBA64Model || img.ColorModel() == color.Gray16Model

	var (
		gray   *image.Gray
		gray16 *image.Gray16
	)

	if !alpha {
		if wide {
			gray16 = image.NewGray16(bounds)
		} else {
			gray = image.NewGray(bounds)
		}
	}

	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			pixel := NRGBA64At(img, column, row)
			if pixel.R != pixel.G || pixel.R != pixel.B {
				return nil, fmt.Errorf("processing introduced RGB color into grayscale samples")
			}

			if gray16 != nil {
				gray16.SetGray16(column, row, color.Gray16{Y: pixel.R})
			} else if gray != nil {
				gray.SetGray(column, row, color.Gray{Y: uint8(pixel.R >> 8)})
			}
		}
	}

	if gray16 != nil {
		return gray16, nil
	}

	if gray != nil {
		return gray, nil
	}

	return img, nil
}

// NRGBA64At avoids interface boxing per sample on the decoded pixel layouts.
func NRGBA64At(img image.Image, column, row int) color.NRGBA64 {
	switch pixels := img.(type) {
	case *image.NRGBA64:
		return pixels.NRGBA64At(column, row)
	case *image.NRGBA:
		pixel := pixels.NRGBAAt(column, row)

		return color.NRGBA64{R: uint16(pixel.R) * 257, G: uint16(pixel.G) * 257, B: uint16(pixel.B) * 257, A: uint16(pixel.A) * 257}
	case *image.Gray16:
		gray := pixels.Gray16At(column, row).Y

		return color.NRGBA64{R: gray, G: gray, B: gray, A: 65535}
	case *image.Gray:
		gray := uint16(pixels.GrayAt(column, row).Y) * 257

		return color.NRGBA64{R: gray, G: gray, B: gray, A: 65535}
	default:
		return color.NRGBA64Model.Convert(img.At(column, row)).(color.NRGBA64)
	}
}
