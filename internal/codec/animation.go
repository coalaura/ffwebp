package codec

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"math/bits"
	"time"

	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/ffwebp/internal/resize"
)

// Animation retains native ticks when available; otherwise Delays are milliseconds.
// LoopCount follows GIF: 0 loops forever, -1 plays once, and positive values count repeats.
type Animation struct {
	Frames           []image.Image
	Delays           []int
	LoopCount        int
	Background       color.RGBA
	Animated         bool
	Durations        []uint32
	TicksNumerator   uint32
	TicksDenominator uint32
	ICC              []byte
}

// AnimatedDecoder can decode all frames from an input.
type AnimatedDecoder interface {
	DecodeAll(io.Reader) (*Animation, error)
}

// AnimatedEncoder can encode all frames into an animated output.
type AnimatedEncoder interface {
	EncodeAll(io.Writer, *Animation, opts.Common) error
}

func (animation *Animation) FrameAt(target time.Duration) int {
	position := uint64(target.Milliseconds())

	if len(animation.Durations) > 0 {
		high, low := bits.Mul64(uint64(target), uint64(animation.TicksNumerator))
		divisor := uint64(animation.TicksDenominator) * uint64(time.Second)

		if divisor == 0 || high >= divisor {
			return len(animation.Frames) - 1
		}

		position, _ = bits.Div64(high, low, divisor)
	}

	elapsed := uint64(0)

	for index := range animation.Frames {
		duration := uint64(0)

		if index < len(animation.Durations) {
			duration = uint64(animation.Durations[index])
		} else if index < len(animation.Delays) {
			duration = uint64(animation.Delays[index])
		}

		elapsed += duration
		if elapsed > position {
			return index
		}
	}

	return len(animation.Frames) - 1
}

func (animation *Animation) MillisecondDelays() ([]int, error) {
	if len(animation.Durations) == 0 {
		return animation.Delays, nil
	}

	numerator := uint64(animation.TicksNumerator)
	if numerator == 0 || animation.TicksDenominator == 0 {
		return nil, fmt.Errorf("invalid animation time base")
	}

	delays := make([]int, len(animation.Durations))

	// Round cumulative boundaries, carrying the fractional part to avoid drift.
	remainder := numerator / 2
	total := 0

	for index, duration := range animation.Durations {
		high, low := bits.Mul64(uint64(duration)*uint64(animation.TicksDenominator), 1000)
		low, carry := bits.Add64(low, remainder, 0)

		high += carry
		if high >= numerator {
			return nil, fmt.Errorf("frame %d duration exceeds the millisecond timeline", index)
		}

		var delay uint64

		delay, remainder = bits.Div64(high, low, numerator)
		if delay > uint64(math.MaxInt-total) {
			return nil, fmt.Errorf("animation duration exceeds the millisecond timeline")
		}

		if duration > 0 && delay == 0 {
			return nil, fmt.Errorf("visible frame %d cannot be represented in milliseconds; use JPEG XL output to preserve native timing", index)
		}

		delays[index] = int(delay)
		total += int(delay)
	}

	return delays, nil
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
