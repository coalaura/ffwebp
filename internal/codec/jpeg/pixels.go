package jpeg

import (
	"encoding/binary"
	"fmt"
	"image"

	"github.com/coalaura/jpeg/classic"
)

func displayImage(samples *classic.Samples) (image.Image, error) {
	// DecodeSamples already checked the caller's limits. Match classic's display
	// range expansion here without decoding again or imposing a second pixel cap.
	bounds := image.Rect(0, 0, samples.Width, samples.Height)

	if samples.ColorSpace == classic.CMYK {
		if samples.BitsPerSample != 8 {
			return nil, fmt.Errorf("classic: high-precision CMYK requires DecodeSamples")
		}

		img := image.NewCMYK(bounds)

		for index, value := range samples.Values {
			img.Pix[index] = byte(value)
		}

		return img, nil
	}

	var (
		img    image.Image
		pixels []byte
	)

	channels := 3
	wide := samples.BitsPerSample > 8

	if samples.ColorSpace == classic.Grayscale {
		channels = 1

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

	maximum := uint32(1<<samples.BitsPerSample) - 1
	target := uint32(255)

	if wide {
		target = 65535
	}

	destination := 0

	for source := 0; source < len(samples.Values); source += channels {
		for channel := range channels {
			value := uint32(samples.Values[source+channel]) * target / maximum

			if wide {
				binary.BigEndian.PutUint16(pixels[destination:], uint16(value))
				destination += 2
			} else {
				pixels[destination] = byte(value)
				destination++
			}
		}

		if channels == 3 {
			pixels[destination] = 255
			destination++

			if wide {
				pixels[destination] = 255
				destination++
			}
		}
	}

	return img, nil
}
