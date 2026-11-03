package png

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/gen2brain/pngn"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var compression int

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "png"
}

func (impl) Extensions() []string {
	return []string{"png", "apng"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:        "png.compression",
		Usage:       "PNG: compression level (0=none, 1=default, 2=speed, 3=best)",
		Value:       1,
		Destination: &compression,
		Validator: func(value int) error {
			if value < 0 || value > 3 {
				return fmt.Errorf("invalid png.compression: %d", value)
			}

			return nil
		},
	})
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	buf := make([]byte, len(magic))

	_, err := reader.ReadAt(buf, 0)
	if err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (adapter impl) Decode(reader io.Reader) (image.Image, error) {
	animation, err := adapter.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	return &codec.ProfiledImage{Image: animation.Frames[0], ICC: animation.ICC, Channels: animation.Channels}, nil
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	// Observe chunk metadata during the sole pixel decode, including on stdin.
	input := &metadataReader{reader: reader, remaining: 8, channels: 4}
	metadata := new(pngn.Metadata)

	animation, err := pngn.DecodeAll(input, &pngn.Options{Metadata: metadata})
	if err != nil {
		return nil, err
	}

	if input.animated && len(input.delays) != len(animation.Image) {
		return nil, fmt.Errorf("png: frame delay fractions do not match the decoded frame count")
	}

	channels := input.channels

	var profile []byte

	if metadata.ICCProfile != nil {
		profile = metadata.ICCProfile.Profile
	}

	if channels == 1 {
		for index, frame := range animation.Image {
			if opaque, ok := frame.(interface{ Opaque() bool }); ok && !opaque.Opaque() {
				channels = 2

				break
			}

			animation.Image[index], err = codec.GrayImage(frame, false)
			if err != nil {
				return nil, err
			}
		}
	}

	loops := animation.LoopCount

	// pngn reads/writes acTL.num_plays directly, not GIF's repeat count.
	if loops == 1 {
		loops = -1
	} else if loops > 1 {
		loops--
	}

	// DecodeAll already composites/disposes frames and excludes a separate default PNG image.
	return &codec.Animation{
		Frames:         animation.Image,
		DelaySeconds:   animation.Delay,
		DelayFractions: input.delays,
		LoopCount:      loops,
		Animated:       input.animated,
		ICC:            profile,
		Channels:       channels,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	err := codec.ValidateICC(codec.ImageChannels(img), common.ICC)
	if err != nil {
		return err
	}

	logx.Printf("png: compression=%d\n", compression)

	settings := pngn.EncodeOptions{
		CompressionLevel: compressionLevel(compression),
		Metadata:         profileMetadata(common.ICC),
	}

	return pngn.Encode(writer, img, &settings)
}

func (adapter impl) EncodeAll(writer io.Writer, animation *codec.Animation, options opts.Common) error {
	if animation == nil || len(animation.Frames) == 0 {
		return fmt.Errorf("png: animation has no frames")
	}

	for index, frame := range animation.Frames {
		if frame == nil {
			return fmt.Errorf("png: frame %d is nil", index)
		}

		err := codec.ValidateICC(codec.ImageChannels(frame), options.ICC)
		if err != nil {
			return err
		}
	}

	if len(animation.Frames) == 1 && !animation.Animated {
		return adapter.Encode(writer, animation.Frames[0], options)
	}

	delays, err := animationDelays(animation)
	if err != nil {
		return err
	}

	loops := animation.LoopCount
	if loops < -1 || int64(loops) >= math.MaxUint32 {
		return fmt.Errorf("png: unsupported loop count %d", loops)
	}

	if loops == -1 {
		loops = 1
	} else if loops > 0 {
		loops++
	}

	settings := pngn.EncodeOptions{
		CompressionLevel: compressionLevel(compression),
		Metadata:         profileMetadata(options.ICC),
	}

	output := pngn.PNG{
		Image:     animation.Frames,
		Delay:     make([]float64, len(delays)),
		LoopCount: loops,
	}

	logx.Printf("png: frames=%d compression=%d\n", len(output.Image), compression)

	// pngn's float API cannot express every APNG fraction. Replace only fcTL
	// timing and its CRC while forwarding the single pixel encode unchanged.
	timing := &timingWriter{writer: writer, delays: delays, remaining: 8}

	err = pngn.EncodeAll(timing, &output, &settings)
	if err != nil {
		return err
	}

	return timing.finish()
}

func profileMetadata(profile []byte) *pngn.Metadata {
	if len(profile) == 0 {
		return nil
	}

	return &pngn.Metadata{ICCProfile: &pngn.ICCProfile{Name: "ICC", Profile: profile}}
}

func compressionLevel(level int) pngn.CompressionLevel {
	switch level {
	case 0:
		return pngn.NoCompression
	case 1:
		return pngn.DefaultCompression
	case 2:
		return pngn.BestSpeed
	case 3:
		return pngn.BestCompression
	default:
		return pngn.DefaultCompression
	}
}

func animationDelays(animation *codec.Animation) ([]codec.FrameDelay, error) {
	count := len(animation.Frames)

	switch {
	case len(animation.Durations) > 0:
		if len(animation.Durations) != count || animation.TicksNumerator == 0 || animation.TicksDenominator == 0 {
			return nil, fmt.Errorf("png: invalid native animation timing")
		}
	case len(animation.DelayFractions) > 0:
		if len(animation.DelayFractions) != count {
			return nil, fmt.Errorf("png: invalid animation fractions")
		}
	case len(animation.DelaySeconds) > 0:
		if len(animation.DelaySeconds) != count {
			return nil, fmt.Errorf("png: invalid animation timing")
		}
	}

	delays := make([]codec.FrameDelay, count)

	var residual float64

	for index := range delays {
		seconds := animation.SecondsAt(index)
		if math.IsNaN(seconds) || seconds < 0 || seconds > math.MaxUint16 {
			return nil, fmt.Errorf("png: frame %d delay is outside APNG's range; use JPEG XL output", index)
		}

		delay := nearestPNGDelay(seconds)
		if delay.Numerator == 0 && seconds > 0 {
			return nil, fmt.Errorf("png: visible frame %d is too short for APNG timing; use JPEG XL output", index)
		}

		if delaySeconds(delay) != seconds {
			// Only frames that need approximation absorb previous rounding error.
			if residual != 0 {
				adjusted := min(max(seconds+residual, 0), math.MaxUint16)

				corrected := nearestPNGDelay(adjusted)
				if corrected.Numerator != 0 {
					delay = corrected
				}
			}

			residual += seconds - delaySeconds(delay)
		}

		delays[index] = delay
	}

	return delays, nil
}

func delaySeconds(delay codec.FrameDelay) float64 {
	return float64(delay.Numerator) / float64(delay.Denominator)
}

// nearestPNGDelay finds the closest fraction whose two fields fit in 16 bits.
// Continued fractions also recover rates such as 1/60 from a seconds-only decoder.
func nearestPNGDelay(seconds float64) codec.FrameDelay {
	previous := codec.FrameDelay{Denominator: 1}
	current := codec.FrameDelay{Numerator: 1}

	value := seconds

	for {
		limit := uint32(math.MaxUint16)
		if current.Numerator != 0 {
			limit = min(limit, (math.MaxUint16-previous.Numerator)/current.Numerator)
		}

		if current.Denominator != 0 {
			limit = min(limit, (math.MaxUint16-previous.Denominator)/current.Denominator)
		}

		whole := math.Floor(value)
		step := uint32(min(whole, float64(limit)))

		next := codec.FrameDelay{
			Numerator:   previous.Numerator + step*current.Numerator,
			Denominator: previous.Denominator + step*current.Denominator,
		}

		if whole > float64(limit) {
			// At the field limit the closest candidate is either the last
			// convergent or the furthest intermediate fraction that fits.
			if math.Abs(seconds-delaySeconds(current)) < math.Abs(seconds-delaySeconds(next)) {
				return current
			}

			return next
		}

		if value == whole || delaySeconds(next) == seconds {
			return next
		}

		previous, current = current, next
		value = 1 / (value - whole)
	}
}
