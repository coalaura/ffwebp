package jpegxl

import (
	"encoding/binary"
	"fmt"
	"image"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	jxl "github.com/coalaura/jpeg/jpegxl"
)

func grayAlphaSamples(img image.Image) *jxl.Samples {
	bounds := img.Bounds()
	precision := jpegutil.Precision(img)
	samples := &jxl.Samples{
		Width:         bounds.Dx(),
		Height:        bounds.Dy(),
		Channels:      2,
		BitsPerSample: precision,
		Unsigned:      make([]uint16, bounds.Dx()*bounds.Dy()*2),
	}
	shift := 16 - precision
	index := 0

	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			pixel := codec.NRGBA64At(img, column, row)
			samples.Unsigned[index] = pixel.R >> shift
			samples.Unsigned[index+1] = pixel.A >> shift
			index += 2
		}
	}

	return samples
}

func displayImage(samples *jxl.Samples) (image.Image, error) {
	// DecodeAnimation has already validated samples against the caller's limits.
	// Samples.Image applies a fixed default pixel cap, so materialize here to
	// honor larger explicit limits without losing 16-bit or gray-alpha samples.
	if samples.ExponentBits != 0 || samples.PremultipliedAlpha {
		return nil, fmt.Errorf("floating-point HDR or premultiplied pixels require explicit display conversion; premultiplied input can use --jpegxl.unpremultiply-alpha")
	}

	bounds := image.Rect(0, 0, samples.Width, samples.Height)

	var (
		img    image.Image
		pixels []byte
	)

	channels := samples.Channels
	if channels == 2 {
		channels = 4
	}

	wide := samples.BitsPerSample > 8

	if channels == 1 {
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
		channels = 4
	} else {
		rgba := image.NewNRGBA(bounds)
		img = rgba
		pixels = rgba.Pix
		channels = 4
	}

	maximum := uint32(1<<samples.BitsPerSample) - 1
	target := uint32(255)

	if wide {
		target = 65535
	}

	destination := 0

	for source := 0; source < len(samples.Unsigned); source += samples.Channels {
		for channel := 0; channel < channels; channel++ {
			value := maximum
			sampleChannel := channel

			if samples.Channels == 2 {
				sampleChannel = 0

				if channel == 3 {
					sampleChannel = 1
				}
			}

			if sampleChannel < samples.Channels {
				value = uint32(samples.Unsigned[source+sampleChannel])
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
