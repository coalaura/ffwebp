package codec

import (
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type Codec interface {
	Name() string

	Flags([]cli.Flag) []cli.Flag
	Extensions() []string

	Sniff(io.ReaderAt) (int, error)
	Decode(io.Reader) (image.Image, error)
	Encode(io.Writer, image.Image, opts.Common) error
}

var codecs = map[string]Codec{}

func Register(c Codec) {
	codecs[c.Name()] = c
}

func Flags(flags []cli.Flag) []cli.Flag {
	for _, codec := range codecs {
		flags = codec.Flags(flags)
	}

	return flags
}

func Get(name string) (Codec, bool) {
	c, ok := codecs[name]

	return c, ok
}

func All() []Codec {
	out := make([]Codec, 0, len(codecs))

	for _, c := range codecs {
		out = append(out, c)
	}

	return out
}
