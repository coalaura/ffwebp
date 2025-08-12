package jpegxl

import (
	"bytes"
	"fmt"
	"image"
	"io"

	jxl "github.com/gen2brain/jpegxl"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	effort int
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "jpegxl"
}

func (impl) Extensions() []string {
	return []string{"jxl"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "jpegxl.effort",
			Usage:       "JPEGXL: encode effort (1=fast .. 10=slow). Default 7",
			Value:       7,
			Destination: &effort,
			Validator: func(value int) error {
				if value < 1 || value > 10 {
					return fmt.Errorf("invalid jpegxl.effort: %d", value)
				}

				return nil
			},
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	containerMagic := []byte{
		0x00, 0x00, 0x00, 0x0C,
		'j', 'x', 'l', ' ',
		0x0D, 0x0A, 0x87, 0x0A,
	}

	buf := make([]byte, 12)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		if err == io.EOF {
			return 0, nil, nil
		}

		return 0, nil, err
	}

	if bytes.Equal(buf, containerMagic) {
		return 100, containerMagic, nil
	}

	if buf[0] == 0xFF && buf[1] == 0x0A {
		return 100, buf[:2], nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return jxl.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("jpegxl: quality=%d effort=%d\n", options.Quality, effort)

	return jxl.Encode(writer, img, jxl.Options{
		Quality: options.Quality,
		Effort:  effort,
	})
}
