package codec

import (
	"image"
	"image/color"
	"io"

	"github.com/coalaura/ffwebp/internal/opts"
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
