package codec

import (
	"image"
	"image/color"
	"image/draw"
	"io"

	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/ffwebp/internal/resize"
)

// Animation represents a simple multi-frame image. Delays are in milliseconds.
type Animation struct {
	Frames     []image.Image
	Delays     []int
	LoopCount  int
	Background color.RGBA
}

// AnimatedDecoder can decode all frames from an input.
type AnimatedDecoder interface {
	DecodeAll(io.Reader) (*Animation, error)
}

// AnimatedEncoder can encode all frames into an animated output.
type AnimatedEncoder interface {
	EncodeAll(io.Writer, *Animation, opts.Common) error
}

func NormalizeAnimationFrames(frames []image.Image, width, height int) []image.Image {
	if len(frames) == 0 {
		return frames
	}

	normalized := make([]image.Image, len(frames))

	for i, frame := range frames {
		bounds := frame.Bounds()

		if bounds.Dx() == width && bounds.Dy() == height && bounds.Min.X == 0 && bounds.Min.Y == 0 {
			normalized[i] = frame
		} else {
			canvas := image.NewRGBA(image.Rect(0, 0, width, height))

			resized := resize.Resize(width, height, frame)

			draw.Draw(canvas, image.Rect(0, 0, width, height), resized, resized.Bounds().Min, draw.Over)

			normalized[i] = canvas
		}
	}

	return normalized
}
