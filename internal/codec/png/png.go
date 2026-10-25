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

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return pngn.Decode(reader)
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	animation, err := pngn.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	delays := make([]int, len(animation.Image))

	for index, seconds := range animation.Delay {
		// ffwebp uses milliseconds: round to nearest, with half milliseconds rounded up.
		delays[index] = int(math.Round(seconds * 1000))
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
		Frames:    animation.Image,
		Delays:    delays,
		LoopCount: loops,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	logx.Printf("png: compression=%d\n", compression)

	settings := pngn.EncodeOptions{
		CompressionLevel: compressionLevel(compression),
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
	}

	if len(animation.Frames) == 1 {
		return adapter.Encode(writer, animation.Frames[0], options)
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

	delays := make([]float64, len(animation.Frames))

	for index := range delays {
		if index < len(animation.Delays) {
			seconds, err := delaySeconds(animation.Delays[index])
			if err != nil {
				return fmt.Errorf("png: frame %d: %w", index, err)
			}

			delays[index] = seconds
		}
	}

	settings := pngn.EncodeOptions{
		CompressionLevel: compressionLevel(compression),
	}

	output := pngn.PNG{
		Image:     animation.Frames,
		Delay:     delays,
		LoopCount: loops,
	}

	logx.Printf("png: frames=%d compression=%d\n", len(output.Image), compression)

	return pngn.EncodeAll(writer, &output, &settings)
}

func init() {
	codec.Register(impl{})
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

func delaySeconds(milliseconds int) (float64, error) {
	if milliseconds < 0 {
		return 0, fmt.Errorf("negative delay %dms", milliseconds)
	}

	seconds := float64(milliseconds) / 1000

	if milliseconds <= math.MaxUint16 {
		return seconds, nil
	}

	// pngn v0.1.3 tries these denominators, then clamps to 65535ms. Reject that lossy fallback.
	denominators := [...]float64{1000, 100, 60, 30, 25, 24, 10, 1}

	for _, denominator := range denominators {
		numerator := seconds * denominator
		if numerator <= math.MaxUint16 && numerator == math.Trunc(numerator) {
			return seconds, nil
		}
	}

	return 0, fmt.Errorf("delay %dms cannot be represented by the PNG encoder", milliseconds)
}
