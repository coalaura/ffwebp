package png

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	compression int
)

type impl struct{}

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "png"
}

func (impl) Extensions() []string {
	return []string{"png"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:        "png.compression",
		Usage:       "PNG: compression level (0=none, 1=default, 2=speed, 3=best)",
		Value:       1,
		Destination: &compression,
		Validator: func(value int) error {
			if value < 0 || value > 3 {
				return fmt.Errorf("invalid compression level: %q", value)
			}

			return nil
		},
	})
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	buf := make([]byte, len(magic))

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return png.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	logx.Printf("png: compression=%d\n", compression)

	encoder := png.Encoder{
		CompressionLevel: compressionLevel(compression),
	}

	return encoder.Encode(writer, img)
}

func compressionLevel(level int) png.CompressionLevel {
	switch level {
	case 0:
		return png.NoCompression
	case 1:
		return png.DefaultCompression
	case 2:
		return png.BestSpeed
	case 3:
		return png.BestCompression
	default:
		return png.DefaultCompression
	}
}
