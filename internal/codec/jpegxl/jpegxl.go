package jpegxl

import (
	"fmt"
	"image"
	"io"
	"math"

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

func (adapter impl) Decode(reader io.Reader) (image.Image, error) {
	animation, err := adapter.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	return &codec.ProfiledImage{Image: animation.Frames[0], ICC: animation.ICC}, nil
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	settings := decodeOptions()

	native, err := jxl.DecodeAnimation(reader, &settings)
	if err != nil {
		return nil, err
	}

	animation := &codec.Animation{
		Frames:           make([]image.Image, len(native.Frames)),
		LoopCount:        int(native.LoopCount) - 1,
		Animated:         native.Animated,
		TicksNumerator:   native.TicksNumerator,
		TicksDenominator: native.TicksDenominator,
		ICC:              native.OutputICC,
	}

	if native.Animated {
		animation.Durations = make([]uint32, len(native.Frames))
	}

	switch native.LoopCount {
	case 0:
		animation.LoopCount = 0
	case 1:
		animation.LoopCount = -1
	}

	for index := range native.Frames {
		frame := &native.Frames[index]

		animation.Frames[index], err = displayImage(frame.Samples)
		if err != nil {
			return nil, fmt.Errorf("jpegxl frame %d: %w", index, err)
		}

		if native.Animated {
			animation.Durations[index] = frame.Duration
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

	err = codec.ValidateICC(img, settings.ICC)
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

	if len(settings.ICC) == 0 && !command.IsSet("jpegxl.linear") {
		settings.ICC = animation.ICC
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

	if len(animation.Durations) > 0 {
		if len(animation.Durations) != len(animation.Frames) {
			return fmt.Errorf("jpegxl: native durations must match the frame count")
		}

		native.TicksNumerator = animation.TicksNumerator
		native.TicksDenominator = animation.TicksDenominator
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

		err = codec.ValidateICC(img, settings.ICC)
		if err != nil {
			return err
		}

		if len(animation.Durations) > 0 {
			native.Frames[index] = jxl.Frame{Image: img, Duration: animation.Durations[index]}

			continue
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

func validatePrecision(img image.Image, settings *jxl.Options) error {
	if settings.Lossless && settings.BitsPerSample != 0 && settings.BitsPerSample < jpegutil.Precision(img) {
		return fmt.Errorf("jpegxl: lossless encoding requires source precision")
	}

	return nil
}
