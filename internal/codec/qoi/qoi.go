package qoi

import (
	"bytes"
	"image"
	"io"

	"github.com/kriticalflare/qoi"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "qoi"
}

func (impl) Extensions() []string {
	return []string{"qoi"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{'q', 'o', 'i', 'f'}

	buf := make([]byte, 4)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return qoi.ImageDecode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	return qoi.ImageEncode(writer, img)
}
