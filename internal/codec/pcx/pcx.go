package pcx

import (
	"image"
	"io"

	"github.com/samuel/go-pcx/pcx"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "pcx"
}

func (impl) Extensions() []string {
	return []string{"pcx"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 4)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if buf[0] != 0x0A {
		return 0, nil, nil
	}

	if buf[2] != 0x01 {
		return 50, buf, nil
	}

	return 100, buf, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	return pcx.Decode(r)
}

func (impl) Encode(w io.Writer, img image.Image, options opts.Common) error {
	return pcx.Encode(w, img)
}
