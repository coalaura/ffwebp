package codec

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"math/bits"
	"time"

	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/ffwebp/internal/resize"
)

type FrameDelay struct {
	Numerator   uint32
	Denominator uint32
}

// Animation retains native ticks, fractions or seconds when available; Delays are milliseconds.
// LoopCount follows GIF: 0 loops forever, -1 plays once, and positive values count repeats.
type Animation struct {
	Frames           []image.Image
	Delays           []int
	DelaySeconds     []float64
	DelayFractions   []FrameDelay
	LoopCount        int
	Background       color.RGBA
	Animated         bool
	Durations        []uint32
	TicksNumerator   uint32
	TicksDenominator uint32
	ICC              []byte
	Channels         int
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
	if len(animation.DelaySeconds) > 0 && len(animation.Durations) == 0 {
		elapsed := float64(0)

		for index, seconds := range animation.DelaySeconds {
			elapsed += seconds
			if elapsed > target.Seconds() {
				return index
			}
		}

		return len(animation.Frames) - 1
	}

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
	return animation.DelaysIn(1000)
}

// DelaysIn quantizes cumulative boundaries once, directly in destination units.
func (animation *Animation) DelaysIn(units uint64) ([]int, error) {
	if len(animation.DelaySeconds) > 0 && len(animation.Durations) == 0 {
		return animation.secondsDelaysIn(units)
	}

	numerator := uint64(1000)
	denominator := uint64(1)
	native := len(animation.Durations) > 0

	if native {
		if len(animation.Durations) != len(animation.Frames) {
			return nil, fmt.Errorf("native durations must match the frame count")
		}

		numerator = uint64(animation.TicksNumerator)
		denominator = uint64(animation.TicksDenominator)
	}

	if numerator == 0 || denominator == 0 || units == 0 {
		return nil, fmt.Errorf("invalid animation time base")
	}

	delays := make([]int, len(animation.Frames))

	// Round cumulative boundaries, carrying the fractional part to avoid drift.
	remainder := numerator / 2
	total := 0

	for index := range delays {
		duration := uint64(0)

		if native {
			duration = uint64(animation.Durations[index])
		} else if index < len(animation.Delays) {
			if animation.Delays[index] < 0 {
				return nil, fmt.Errorf("frame %d has a negative delay", index)
			}

			duration = uint64(animation.Delays[index])
		}

		high, low := bits.Mul64(duration*denominator, units)
		low, carry := bits.Add64(low, remainder, 0)

		high += carry
		if high >= numerator {
			return nil, fmt.Errorf("frame %d duration exceeds the destination timeline", index)
		}

		var delay uint64

		delay, remainder = bits.Div64(high, low, numerator)
		if delay > uint64(math.MaxInt-total) {
			return nil, fmt.Errorf("animation duration exceeds the destination timeline")
		}

		if duration > 0 && delay == 0 {
			return nil, fmt.Errorf("visible frame %d cannot be represented at %d timing units per second; use JPEG XL output to preserve native timing", index, units)
		}

		delays[index] = int(delay)
		total += int(delay)
	}

	return delays, nil
}

func (animation *Animation) SecondsAt(index int) float64 {
	if index < len(animation.Durations) {
		return float64(animation.Durations[index]) * float64(animation.TicksDenominator) / float64(animation.TicksNumerator)
	}

	if index < len(animation.DelayFractions) {
		delay := animation.DelayFractions[index]

		return float64(delay.Numerator) / float64(delay.Denominator)
	}

	if index < len(animation.DelaySeconds) {
		return animation.DelaySeconds[index]
	}

	if index < len(animation.Delays) {
		return float64(animation.Delays[index]) / 1000
	}

	return 0
}

func (animation *Animation) secondsDelaysIn(units uint64) ([]int, error) {
	if units == 0 || len(animation.DelaySeconds) != len(animation.Frames) {
		return nil, fmt.Errorf("invalid animation timing")
	}

	delays := make([]int, len(animation.Frames))

	var (
		elapsed  float64
		previous int
	)

	for index, seconds := range animation.DelaySeconds {
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
			return nil, fmt.Errorf("frame %d has an invalid delay", index)
		}

		elapsed += seconds

		boundary := math.Round(elapsed * float64(units))
		if boundary >= float64(math.MaxInt) {
			return nil, fmt.Errorf("animation duration exceeds the destination timeline")
		}

		delay := int(boundary) - previous
		if seconds > 0 && delay == 0 {
			return nil, fmt.Errorf("visible frame %d cannot be represented at %d timing units per second", index, units)
		}

		delays[index] = delay
		previous = int(boundary)
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
			normalized[i] = resize.Resize(width, height, frame)
		}
	}

	return normalized
}
