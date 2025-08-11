package ico

import (
	"bytes"
	"image"
	"io"

	"github.com/sergeymakinen/go-ico"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "ico"
}

func (impl) Extensions() []string {
	return []string{"ico", "cur"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magicICO := []byte{0x00, 0x00, 0x01, 0x00}
	magicCUR := []byte{0x00, 0x00, 0x02, 0x00}

	buf := make([]byte, 4)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magicICO) || bytes.Equal(buf, magicCUR) {
		return 100, buf, nil
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	images, err := ico.DecodeAll(r)
	if err != nil {
		return nil, err
	}

	var (
		best     image.Image
		bestArea int
	)

	for _, img := range images {
		bounds := img.Bounds()
		area := bounds.Dx() * bounds.Dy()

		if area > bestArea {
			best = img
			bestArea = area
		}
	}

	if best == nil {
		return nil, io.EOF
	}

	return best, nil
}

func (impl) Encode(w io.Writer, img image.Image, _ opts.Common) error {
	return ico.Encode(w, img)
}
