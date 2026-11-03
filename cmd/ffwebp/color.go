package main

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/opts"
)

func validateOutputICC(output codec.Codec, common *opts.Common) error {
	if len(common.ICC) == 0 {
		return nil
	}

	err := codec.ValidateICC(common.Channels, common.ICC)
	if err != nil {
		return err
	}

	switch output.String() {
	case "jpegxl":
		return nil
	case "jpeg2000":
		if common.OutputExtension != "j2k" && common.OutputExtension != "j2c" {
			return nil
		}
	case "png":
		if common.Channels != 2 {
			return nil
		}
	case "webp":
		if common.Channels >= 3 {
			return nil
		}
	}

	if codec.StandardSRGB(common.ICC) {
		// Repeating standard sRGB gray into RGB needs no numerical transform.
		common.ICC = nil

		return nil
	}

	return fmt.Errorf("%s output cannot carry this ICC profile with its channel layout; use JP2 or JPEG XL, or convert the profile and pixels to sRGB first", output)
}

func applyEffects(img image.Image, common *opts.Common) (image.Image, int, error) {
	precision := jpegutil.Precision(img)

	processed, count, err := effects.ApplyAll(img)
	if err != nil || count == 0 {
		return processed, count, err
	}

	if jpegutil.Precision(processed) < precision {
		return nil, 0, fmt.Errorf("effects cannot preserve %d-bit samples; convert explicitly to 8-bit before applying effects", precision)
	}

	if common.Channels > 2 {
		return processed, count, nil
	}

	if opaque, ok := processed.(interface{ Opaque() bool }); ok && !opaque.Opaque() {
		common.Channels = 2
	}

	gray, err := codec.GrayImage(processed, common.Channels == 2)
	if err == nil {
		return gray, count, nil
	}

	if !codec.StandardSRGB(common.ICC) {
		return nil, 0, fmt.Errorf("effect introduces RGB color under a GRAY ICC profile; convert the profile and pixels to sRGB before applying color effects")
	}

	common.Channels = 4
	common.ICC = nil

	return processed, count, nil
}

func normalizeColorFrames(frames []image.Image, channels int) {
	if channels <= 2 {
		return
	}

	// A color effect can change only some frames; encode every frame in the new RGB layout.
	for index, frame := range frames {
		if codec.ImageChannels(frame) != 1 {
			continue
		}

		var destination draw.Image

		if jpegutil.Precision(frame) == 16 {
			destination = image.NewNRGBA64(frame.Bounds())
		} else {
			destination = image.NewNRGBA(frame.Bounds())
		}

		draw.Draw(destination, destination.Bounds(), frame, frame.Bounds().Min, draw.Src)
		frames[index] = destination
	}
}
