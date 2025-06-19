package jpeg

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

func init() {
	codec.Register(impl{})
}

func (impl) Name() string {
	return "jpeg"
}

func (impl) Extensions() []string {
	return []string{"jpg", "jpeg", "jpe"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, error) {
	marker := []byte{0xFF, 0xD8, 0xFF}

	buf := make([]byte, 3)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, err
	}

	if bytes.Equal(buf, marker) {
		return 100, nil
	}

	return 0, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return jpeg.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	return jpeg.Encode(writer, img, &jpeg.Options{
		Quality: options.Quality,
	})
}
