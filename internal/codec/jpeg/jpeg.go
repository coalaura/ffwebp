package jpeg

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

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
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0xFF, 0xD8, 0xFF}

	buf := make([]byte, 3)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return jpeg.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("jpeg: quality=%d\n", options.Quality)

	return jpeg.Encode(writer, img, &jpeg.Options{
		Quality: options.Quality,
	})
}
