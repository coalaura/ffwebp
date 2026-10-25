package tiff

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"

	"golang.org/x/image/tiff"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var (
	compression int
	predictor   bool
)

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "tiff"
}

func (impl) Extensions() []string {
	return []string{"tiff", "tif"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "tiff.compression",
			Usage:       "TIFF: compression (0=none, 1=deflate)",
			Value:       1,
			Destination: &compression,
			Validator: func(value int) error {
				if value < 0 || value > 1 {
					return fmt.Errorf("invalid tiff.compression: %d", value)
				}

				return nil
			},
		},
		&cli.BoolFlag{
			Name:        "tiff.predictor",
			Usage:       "TIFF: enable differencing predictor with Deflate compression",
			Value:       false,
			Destination: &predictor,
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 8)

	n, err := reader.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, nil, err
	}

	buf = buf[:n]

	if len(buf) < 8 {
		return 0, nil, nil
	}

	if bytes.Equal(buf[0:4], []byte{0x49, 0x49, 0x2A, 0x00}) || bytes.Equal(buf[0:4], []byte{0x4D, 0x4D, 0x00, 0x2A}) {
		return 100, buf[:8], nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return tiff.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	logx.Printf("tiff: compression=%d predictor=%t\n", compression, predictor)

	return tiff.Encode(writer, img, &tiff.Options{
		Compression: compressionType(compression),
		Predictor:   predictor,
	})
}

func compressionType(level int) tiff.CompressionType {
	if level == 0 {
		return tiff.Uncompressed
	}

	return tiff.Deflate
}
