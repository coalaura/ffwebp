package bmp

import (
	"bytes"
	"image"
	"io"

	"golang.org/x/image/bmp"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "bmp"
}

func (impl) Extensions() []string {
	return []string{"bmp"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0x42, 0x4D}

	buf := make([]byte, 2)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return bmp.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	return bmp.Encode(writer, img)
}
