package codec

import (
	"fmt"
	"image"
)

// ProfiledImage associates decoded samples with their color space until the
// processing pipeline separates metadata from the concrete pixel storage.
type ProfiledImage struct {
	image.Image
	ICC []byte
}

func ValidateICC(img image.Image, profile []byte) error {
	if len(profile) == 0 {
		return nil
	}

	if len(profile) < 128 {
		return fmt.Errorf("invalid ICC profile header")
	}

	space := "RGB "

	switch img.(type) {
	case *image.Gray, *image.Gray16:
		space = "GRAY"
	}

	if string(profile[16:20]) != space {
		return fmt.Errorf("ICC color space %q does not describe the output %s channels; convert the source profile and pixels to RGB together before processing", profile[16:20], space)
	}

	return nil
}
