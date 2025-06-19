package webp

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/gen2brain/webp"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	method int
	exact  bool
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "webp"
}

func (impl) Extensions() []string {
	return []string{"webp"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "webp.method",
			Usage:       "WebP: quality/speed trade-off (0=fast, 6=slower-better)",
			Value:       4,
			Destination: &method,
			Validator: func(v int) error {
				if v < 0 || v > 6 {
					return fmt.Errorf("invalid webp.method: %d (must be 0-6)", v)
				}

				return nil
			},
		},
		&cli.BoolFlag{
			Name:        "webp.exact",
			Usage:       "WebP: preserve exact RGB values in transparent areas",
			Value:       false,
			Destination: &exact,
		},
	)
}

func (impl) Sniff(rd io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 12)

	if _, err := rd.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf[0:4], []byte("RIFF")) && bytes.Equal(buf[8:12], []byte("WEBP")) {
		return 100, buf[:12], nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return webp.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("webp: quality=%d lossless=%t method=%d exact=%t\n", options.Quality, options.Lossless, method, exact)

	return webp.Encode(writer, img, webp.Options{
		Quality:  options.Quality,
		Lossless: options.Lossless,
		Method:   method,
		Exact:    exact,
	})
}
