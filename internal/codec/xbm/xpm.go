package xbm

import (
	"bytes"
	"image"
	"io"

	"github.com/coalaura/xpm"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var name string

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "xpm"
}

func (impl) Extensions() []string {
	return []string{"xpm"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.StringFlag{
			Name:        "xpm.name",
			Usage:       "XPM: name of the image definition",
			Value:       "image",
			Destination: &name,
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 128)

	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, nil, err
	}

	buf = buf[:n]

	if bytes.Contains(buf, []byte("/* XPM */")) && bytes.Contains(buf, []byte("bits[]")) {
		return 90, buf, nil
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	return xpm.Decode(r)
}

func (impl) Encode(w io.Writer, img image.Image, _ opts.Common) error {
	return xpm.Encode(w, img, xpm.XPMOptions{
		Name: name,
	})
}
