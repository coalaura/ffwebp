package gif

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var numColors int

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "gif"
}

func (impl) Extensions() []string {
	return []string{"gif"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:        "gif.colors",
		Usage:       "GIF: Number of colors (1-256)",
		Value:       256,
		Destination: &numColors,
		Validator: func(value int) error {
			if value < 1 || value > 256 {
				return fmt.Errorf("invalid number of colors: %d", value)
			}

			return nil
		},
	})
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic7a := []byte("GIF87a")
	magic9a := []byte("GIF89a")

	buf := make([]byte, 6)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic7a) {
		return 100, magic7a, nil
	}

	if bytes.Equal(buf, magic9a) {
		return 100, magic9a, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return gif.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("gif: colors=%d\n", numColors)

	return gif.Encode(writer, img, &gif.Options{
		NumColors: numColors,
	})
}
