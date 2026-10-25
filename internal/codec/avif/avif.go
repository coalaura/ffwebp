package avif

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"
	"math"

	"github.com/gen2brain/avif"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var (
	qualityA int
	speed    int
	chroma   int
)

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "avif"
}

func (impl) Extensions() []string {
	return []string{"avif"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "avif.quality-alpha",
			Usage:       "AVIF: alpha channel quality (0-100; lossless forces 100)",
			Value:       60,
			Destination: &qualityA,
			Validator: func(v int) error {
				if v < 0 || v > 100 {
					return fmt.Errorf("invalid avif.quality-alpha: %d", v)
				}

				return nil
			},
		},
		&cli.IntFlag{
			Name:        "avif.speed",
			Usage:       "AVIF: encoding speed (0=slowest/best, 10=fastest/worst)",
			Value:       6,
			Destination: &speed,
			Validator: func(v int) error {
				if v < 0 || v > 10 {
					return fmt.Errorf("invalid avif.speed: %d", v)
				}

				return nil
			},
		},
		&cli.IntFlag{
			Name:        "avif.chroma",
			Usage:       "AVIF: chroma subsampling (444=best, 422, 420=smallest; lossless forces 444)",
			Value:       444,
			Destination: &chroma,
			Validator: func(v int) error {
				if v != 444 && v != 422 && v != 420 {
					return fmt.Errorf("invalid avif.chroma: %d", v)
				}

				return nil
			},
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 12)

	_, err := reader.ReadAt(buf, 0)
	if err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf[4:12], []byte("ftypavif")) || bytes.Equal(buf[4:12], []byte("ftypavis")) {
		return 100, buf[:12], nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return avif.Decode(reader)
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	anim, err := avif.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	if anim == nil || len(anim.Image) == 0 {
		return nil, fmt.Errorf("avif: decoded animation has no frames")
	}

	delays := make([]int, len(anim.Image))

	for index := range delays {
		if index < len(anim.Delay) {
			delays[index] = int(math.Round(anim.Delay[index] * 1000))
		}
	}

	return &codec.Animation{
		Frames:    anim.Image,
		Delays:    delays,
		LoopCount: anim.LoopCount,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	settings := encoderOptions(options)
	logx.Printf("avif: quality=%d quality-alpha=%d speed=%d chroma=%s lossless=%t\n", settings.Quality, settings.QualityAlpha, settings.Speed, settings.ChromaSubsampling, settings.Lossless)

	return avif.Encode(writer, img, settings)
}

func (impl) EncodeAll(writer io.Writer, anim *codec.Animation, options opts.Common) error {
	if anim == nil || len(anim.Frames) == 0 {
		return fmt.Errorf("avif: animation has no frames")
	}

	// The pinned encoder maps both 0 and -1 to infinite repetition, so it cannot express single-play.
	if anim.LoopCount < 0 {
		return fmt.Errorf("avif: single-play animations are not supported by the encoder")
	}

	frames := make([]image.Image, len(anim.Frames))
	delays := make([]float64, len(anim.Frames))

	for index, frame := range anim.Frames {
		if frame == nil {
			return fmt.Errorf("avif: frame %d is nil", index)
		}

		// The pinned encoder copies RGBA Pix as packed rows, ignoring the source stride.
		if rgba, ok := frame.(*image.RGBA); ok && rgba.Stride != rgba.Bounds().Dx()*4 {
			bounds := rgba.Bounds()
			packed := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
			draw.Draw(packed, packed.Bounds(), rgba, bounds.Min, draw.Src)
			frame = packed
		}

		frames[index] = frame

		if index < len(anim.Delays) {
			delays[index] = float64(anim.Delays[index]) / 1000
		}
	}

	settings := encoderOptions(options)
	logx.Printf("avif: frames=%d quality=%d quality-alpha=%d speed=%d chroma=%s lossless=%t\n", len(frames), settings.Quality, settings.QualityAlpha, settings.Speed, settings.ChromaSubsampling, settings.Lossless)

	output := &avif.AVIF{
		Image:     frames,
		Delay:     delays,
		LoopCount: anim.LoopCount,
	}

	return avif.EncodeAll(writer, output, settings)
}

func encoderOptions(options opts.Common) avif.Options {
	settings := avif.Options{
		Quality:           options.Quality,
		QualityAlpha:      qualityA,
		Speed:             speed,
		ChromaSubsampling: chromaSubsampling(chroma),
		Lossless:          options.Lossless,
	}

	if options.Lossless {
		settings.Quality = 100
		settings.QualityAlpha = 100
		settings.ChromaSubsampling = image.YCbCrSubsampleRatio444
	}

	return settings
}

func chromaSubsampling(c int) image.YCbCrSubsampleRatio {
	switch c {
	case 444:
		return image.YCbCrSubsampleRatio444
	case 422:
		return image.YCbCrSubsampleRatio422
	case 420:
		return image.YCbCrSubsampleRatio420
	default:
		return image.YCbCrSubsampleRatio444
	}
}
