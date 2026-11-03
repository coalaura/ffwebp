package jpeg2000

import (
	"encoding/binary"
	"fmt"
	"image"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	jp2 "github.com/coalaura/jpeg/jpeg2000"
)

func grayAlphaImage(img image.Image, settings *jp2.Options) *jp2.NativeImage {
	bounds := img.Bounds()

	precision := settings.BitsPerSample
	if precision == 0 {
		precision = jpegutil.Precision(img)
	}

	native := &jp2.NativeImage{
		Bounds:     image.Rect(0, 0, bounds.Dx(), bounds.Dy()),
		ColorSpace: jp2.Gray,
		Components: make([]jp2.Component, 2),
		ICC:        settings.ICC,
	}
	maximum := uint32(1<<precision) - 1

	for index := range native.Components {
		native.Components[index] = jp2.Component{
			Width:         bounds.Dx(),
			Height:        bounds.Dy(),
			DX:            1,
			DY:            1,
			BitsPerSample: precision,
			Alpha:         index,
			Values:        make([]int32, bounds.Dx()*bounds.Dy()),
		}
	}

	index := 0

	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			pixel := codec.NRGBA64At(img, column, row)
			native.Components[0].Values[index] = int32((uint32(pixel.R)*maximum + 32767) / 65535)
			native.Components[1].Values[index] = int32((uint32(pixel.A)*maximum + 32767) / 65535)
			index++
		}
	}

	return native
}

func displayImage(native *jp2.NativeImage) (image.Image, error) {
	if len(native.Components) == 0 || native.ColorSpace != jp2.SRGB && native.ColorSpace != jp2.ICCRGB && native.ColorSpace != jp2.Gray {
		return nil, fmt.Errorf("jpeg2000: native color space requires explicit display conversion")
	}

	first := &native.Components[0]
	colors := 3

	if native.ColorSpace == jp2.Gray {
		colors = 1
	}

	channels := len(native.Components)
	if channels != colors && channels != colors+1 {
		return nil, fmt.Errorf("jpeg2000: unsupported component count")
	}

	for index := range native.Components {
		component := &native.Components[index]
		if component.Signed || component.BitsPerSample < 2 || component.BitsPerSample > 16 || component.BitsPerSample != first.BitsPerSample {
			return nil, fmt.Errorf("jpeg2000: signed or mixed-precision components require explicit display conversion")
		}

		if component.Width != first.Width || component.Height != first.Height || component.X0 != first.X0 || component.Y0 != first.Y0 || component.DX != first.DX || component.DY != first.DY {
			return nil, fmt.Errorf("jpeg2000: mismatched component grids require explicit display conversion")
		}

		expectedAlpha := 0

		if index == colors {
			expectedAlpha = 1
		}

		if component.Alpha != expectedAlpha || len(component.Values) != first.Width*first.Height {
			return nil, fmt.Errorf("jpeg2000: unsupported alpha or sample layout")
		}
	}

	// DecodeNative has enforced the caller's pixel limit. NativeImage.Image
	// reapplies the default cap; materialize directly to honor raised limits.
	bounds := image.Rect(0, 0, first.Width, first.Height)
	wide := first.BitsPerSample > 8
	outputChannels := 4

	var (
		img    image.Image
		pixels []byte
	)

	if channels == 1 {
		outputChannels = 1

		if wide {
			gray := image.NewGray16(bounds)
			img = gray
			pixels = gray.Pix
		} else {
			gray := image.NewGray(bounds)
			img = gray
			pixels = gray.Pix
		}
	} else if wide {
		rgba := image.NewNRGBA64(bounds)
		img = rgba
		pixels = rgba.Pix
	} else {
		rgba := image.NewNRGBA(bounds)
		img = rgba
		pixels = rgba.Pix
	}

	maximum := uint32(1<<first.BitsPerSample) - 1
	target := uint32(255)

	if wide {
		target = 65535
	}

	destination := 0

	for index := range first.Values {
		for channel := range outputChannels {
			source := channel

			if colors == 1 && channel < 3 {
				source = 0
			} else if channel == 3 {
				source = colors
			}

			value := maximum

			if source < channels {
				value = uint32(native.Components[source].Values[index])
				if value > maximum {
					return nil, fmt.Errorf("jpeg2000: sample exceeds component precision")
				}
			}

			value = (value*target + maximum/2) / maximum

			if wide {
				binary.BigEndian.PutUint16(pixels[destination:], uint16(value))
				destination += 2
			} else {
				pixels[destination] = byte(value)
				destination++
			}
		}
	}

	return img, nil
}
