package farbfeld

import (
	"bytes"
	"image"
	"io"

	farbfeld "github.com/hullerob/go.farbfeld"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "farbfeld"
}

func (impl) Extensions() []string {
	return []string{"farbfeld", "ff"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 8)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, []byte("farbfeld")) {
		return 100, buf, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return farbfeld.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	return farbfeld.Encode(writer, img)
}
