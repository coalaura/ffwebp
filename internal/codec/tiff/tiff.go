package tiff

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"golang.org/x/image/tiff"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	compression int
	predictor   bool
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

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
			Usage:       "TIFF: compression (0=none, 1=deflate, 2=lzw, 3=ccitt3, 4=ccitt4)",
			Value:       1,
			Destination: &compression,
			Validator: func(value int) error {
				if value < 0 || value > 4 {
					return fmt.Errorf("invalid compression: %d", value)
				}

				return nil
			},
		},
		&cli.BoolFlag{
			Name:        "tiff.predictor",
			Usage:       "TIFF: enable differencing predictor (improves compression for photos)",
			Value:       false,
			Destination: &predictor,
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magicLE := []byte{0x49, 0x49, 0x2A, 0x00}
	magicBE := []byte{0x4D, 0x4D, 0x00, 0x2A}

	buf := make([]byte, 4)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magicLE) {
		return 100, magicLE, nil
	}

	if bytes.Equal(buf, magicBE) {
		return 100, magicBE, nil
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
	switch level {
	case 0:
		return tiff.Uncompressed
	case 1:
		return tiff.Deflate
	case 2:
		return tiff.LZW
	case 3:
		return tiff.CCITTGroup3
	case 4:
		return tiff.CCITTGroup4
	default:
		return tiff.Deflate
	}
}
