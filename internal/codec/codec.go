package codec

import (
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type Codec interface {
	String() string

	Flags([]cli.Flag) []cli.Flag
	Extensions() []string

	CanEncode() bool

	Sniff(io.ReaderAt) (int, []byte, error)
	Decode(io.Reader) (image.Image, error)
	Encode(io.Writer, image.Image, opts.Common) error
}

// Configurator receives parsed CLI options before conversions start.
type Configurator interface {
	Configure(*cli.Command) error
}

// ByteTranscoder optionally converts encoded data without entering the pixel pipeline.
type ByteTranscoder interface {
	Transcode(io.Reader, Codec, Codec) ([]byte, bool, error)
}

// InputLimiter bounds detection before the codec's own bounded loader takes over.
type InputLimiter interface {
	InputLimit() int64
}

// InputProcessor optionally retains a native resource instead of extracting an image.
type InputProcessor interface {
	Configure(*cli.Command) error
	ValidateRoute(input, output Codec) error
	DecodeInput(io.Reader, Codec, io.Writer) (image.Image, func(io.Writer) error, error)
}

var codecs = map[string]Codec{}

func Register(c Codec) {
	codecs[c.String()] = c
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
