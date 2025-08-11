package psd

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/oov/psd"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	skipMerged bool
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "psd"
}

func (impl) Extensions() []string {
	return []string{"psd", "psb"}
}

func (impl) CanEncode() bool {
	return false
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.BoolFlag{
			Name:        "psd.skip-merged",
			Usage:       "PSD: skip decoding merged/composite image and only decode layer images (where supported)",
			Value:       false,
			Destination: &skipMerged,
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0x38, 0x42, 0x50, 0x53}

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
	logx.Printf("psd: skipMerged=%t\n", skipMerged)

	img, _, err := psd.Decode(reader, &psd.DecodeOptions{
		SkipMergedImage: skipMerged,
	})

	if err != nil {
		return nil, err
	}

	return img.Picker, nil
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	return fmt.Errorf("psd: encoding not supported")
}
