package avif

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/vegidio/avif-go"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	qualityA int
	speed    int
	chroma   int
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "avif"
}

func (impl) Extensions() []string {
	return []string{"avif"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "avif.quality-alpha",
			Usage:       "AVIF: alpha channel quality (0-100)",
			Value:       60,
			Destination: &qualityA,
			Validator: func(v int) error {
				if v < 0 || v > 100 {
					return fmt.Errorf("invalid avif.quality-alpha: %d", v)
				}

				return nil
			},
		},
		&cli.IntFlag{
			Name:        "avif.speed",
			Usage:       "AVIF: encoding speed (0=slowest/best, 10=fastest/worst)",
			Value:       6,
			Destination: &speed,
			Validator: func(v int) error {
				if v < 0 || v > 10 {
					return fmt.Errorf("invalid avif.speed: %d", v)
				}

				return nil
			},
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 12)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf[4:12], []byte("ftypavif")) {
		return 100, buf[:12], nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return avif.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("avif: quality=%d quality-alpha=%d speed=%d\n", options.Quality, qualityA, speed)

	return avif.Encode(writer, img, &avif.Options{
		ColorQuality: options.Quality,
		AlphaQuality: qualityA,
		Speed:        speed,
	})
}
