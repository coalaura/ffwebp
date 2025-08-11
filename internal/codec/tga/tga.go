package tga

import (
	"image"
	"io"

	"github.com/ftrvxmtrx/tga"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "tga"
}

func (impl) Extensions() []string {
	return []string{"tga"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 3)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	colorMapType := buf[1]

	if colorMapType > 1 {
		return 0, nil, nil
	}

	validImageTypes := map[byte]bool{
		0:  true, // no image data
		1:  true, // colormapped, uncompressed
		2:  true, // truecolor, uncompressed
		3:  true, // grayscale, uncompressed
		9:  true, // colormapped, RLE
		10: true, // truecolor, RLE
		11: true, // grayscale, RLE
	}

	imageType := buf[2]

	if !validImageTypes[imageType] {
		return 0, nil, nil
	}

	header := make([]byte, 3)
	copy(header, buf)

	return 100, header, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return tga.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	return tga.Encode(writer, img)
}
