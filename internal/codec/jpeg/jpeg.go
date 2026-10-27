package jpeg

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/gen2brain/jpegn"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var (
	progressive          bool
	optimize             bool
	adaptiveQuantization bool
	subsampling          string
	subsamplingValues    = map[string]jpegn.Subsampling{
		"auto": jpegn.SubsampleAuto,
		"444":  jpegn.Subsample444,
		"440":  jpegn.Subsample440,
		"422":  jpegn.Subsample422,
		"420":  jpegn.Subsample420,
		"gray": jpegn.SubsampleGray,
	}
)

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "jpeg"
}

func (impl) Extensions() []string {
	return []string{"jpg", "jpeg", "jpe"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.BoolFlag{
			Name:        "jpeg.progressive",
			Usage:       "JPEG: progressive encoding (extra encoding time for potentially smaller files)",
			Destination: &progressive,
		},
		&cli.BoolFlag{
			Name:        "jpeg.optimize",
			Usage:       "JPEG: optimize Huffman coding (extra encoding time for potentially smaller files)",
			Destination: &optimize,
		},
		&cli.BoolFlag{
			Name:        "jpeg.adaptive-quantization",
			Usage:       "JPEG: discard less visible coefficients (extra encoding time for potentially smaller files)",
			Destination: &adaptiveQuantization,
		},
		&cli.StringFlag{
			Name:        "jpeg.subsampling",
			Usage:       "JPEG: chroma subsampling (auto, 444, 440, 422, 420, gray)",
			Value:       "auto",
			Destination: &subsampling,
			Validator: func(value string) error {
				if _, ok := subsamplingValues[value]; !ok {
					return fmt.Errorf("invalid jpeg.subsampling: %q (want auto, 444, 440, 422, 420, or gray)", value)
				}

				return nil
			},
		},
	)
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0xFF, 0xD8, 0xFF}

	buf := make([]byte, 3)

	n, err := reader.ReadAt(buf, 0)
	if err != nil {
		return 0, nil, err
	}

	buf = buf[:n]

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return jpegn.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("jpeg: quality=%d progressive=%t optimize=%t adaptive-quantization=%t subsampling=%s\n", options.Quality, progressive, optimize, adaptiveQuantization, subsampling)

	return jpegn.Encode(writer, img, &jpegn.EncodeOptions{
		Quality:              options.Quality,
		Progressive:          progressive,
		OptimizeCoding:       optimize,
		AdaptiveQuantization: adaptiveQuantization,
		Subsampling:          subsamplingValues[subsampling],
	})
}
