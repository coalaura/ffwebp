package jpegxl

import (
	"fmt"
	"image"
	"io"
	"math"
	"math/bits"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/jpeg/detect"
	jxl "github.com/coalaura/jpeg/jpegxl"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var command *cli.Command

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "jpegxl"
}

func (impl) Extensions() []string {
	return []string{"jxl"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Configure(cmd *cli.Command) error {
	command = cmd

	return validateTranscode()
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	return jpegutil.Sniff(reader, detect.JXLCodestream, detect.JXLContainer)
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	settings := decodeOptions()
	return jxl.DecodeWithOptions(reader, &settings)
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	settings := decodeOptions()

	native, err := jxl.DecodeAnimation(reader, &settings)
	if err != nil {
		return nil, err
	}

	animation := &codec.Animation{
		Frames:    make([]image.Image, len(native.Frames)),
		Delays:    make([]int, len(native.Frames)),
		LoopCount: int(native.LoopCount) - 1,
	}

	switch native.LoopCount {
	case 0:
		animation.LoopCount = 0
	case 1:
		animation.LoopCount = -1
	}

	// Carry the fractional boundary forward, rounding cumulative time rather
	// than individual frames so rational frame rates do not accumulate drift.
	remainder := uint64(native.TicksNumerator) / 2
	total := 0

	for index := range native.Frames {
		frame := &native.Frames[index]

		animation.Frames[index], err = displayImage(frame.Samples)
		if err != nil {
			return nil, fmt.Errorf("jpegxl frame %d: %w", index, err)
		}

		if native.Animated {
			animation.Delays[index], remainder, err = milliseconds(frame.Duration, native.TicksNumerator, native.TicksDenominator, remainder)
			if err != nil || animation.Delays[index] > math.MaxInt-total {
				return nil, fmt.Errorf("jpegxl: animation duration exceeds the millisecond timeline")
			}

			total += animation.Delays[index]
		}
	}

	return animation, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	settings, err := encodeOptions(common)
	if err != nil {
		return err
	}

	err = validatePrecision(img, &settings)
	if err != nil {
		return err
	}

	return jxl.Encode(writer, img, &settings)
}

func (impl) EncodeAll(writer io.Writer, animation *codec.Animation, common opts.Common) error {
	settings, err := encodeOptions(common)
	if err != nil {
		return err
	}

	if animation.LoopCount < -1 || int64(animation.LoopCount) >= math.MaxUint32 {
		return fmt.Errorf("jpegxl: loop count is outside the supported range")
	}

	native := &jxl.Animation{
		Frames:           make([]jxl.Frame, len(animation.Frames)),
		Animated:         true,
		TicksNumerator:   1000,
		TicksDenominator: 1,
		LoopCount:        uint32(animation.LoopCount + 1),
	}

	switch animation.LoopCount {
	case -1:
		native.LoopCount = 1
	case 0:
		native.LoopCount = 0
	}

	for index, img := range animation.Frames {
		err = validatePrecision(img, &settings)
		if err != nil {
			return err
		}

		delay := 0

		if index < len(animation.Delays) {
			delay = animation.Delays[index]
		}

		if delay < 0 || int64(delay) > math.MaxUint32 {
			return fmt.Errorf("jpegxl: frame %d delay is outside the supported range", index)
		}

		native.Frames[index] = jxl.Frame{Image: img, Duration: uint32(delay)}
	}

	return jxl.EncodeAnimation(writer, native, &settings)
}

func milliseconds(duration, numerator, denominator uint32, remainder uint64) (int, uint64, error) {
	if numerator == 0 || denominator == 0 {
		return 0, 0, fmt.Errorf("jpegxl: invalid animation time base")
	}

	high, low := bits.Mul64(uint64(duration)*uint64(denominator), 1000)
	low, carry := bits.Add64(low, remainder, 0)

	high += carry
	if high >= uint64(numerator) {
		return 0, 0, fmt.Errorf("jpegxl: frame duration overflows")
	}

	quotient, remainder := bits.Div64(high, low, uint64(numerator))
	if quotient > math.MaxInt {
		return 0, 0, fmt.Errorf("jpegxl: frame duration overflows")
	}

	return int(quotient), remainder, nil
}

func validatePrecision(img image.Image, settings *jxl.Options) error {
	if settings.Lossless && settings.BitsPerSample != 0 && settings.BitsPerSample < jpegutil.Precision(img) {
		return fmt.Errorf("jpegxl: lossless encoding requires source precision")
	}

	return nil
}
