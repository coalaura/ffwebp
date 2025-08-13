package xcf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"

	"github.com/gonutz/xcf"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "xcf"
}

func (impl) Extensions() []string {
	return []string{"xcf"}
}

func (impl) CanEncode() bool {
	return false
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte("gimp xcf ")

	buf := make([]byte, len(magic))
	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(buf)

	canvas, err := xcf.Decode(reader)
	if err != nil {
		return nil, err
	}

	dst := image.NewNRGBA(image.Rect(0, 0, int(canvas.Width), int(canvas.Height)))

	for i := len(canvas.Layers) - 1; i >= 0; i-- {
		layer := canvas.Layers[i]
		if !layer.Visible {
			continue
		}

		var src image.Image = layer.RGBA

		if layer.Opacity < 255 {
			src = applyOpacity(src, layer.Opacity)
		}

		dr := src.Bounds().Intersect(dst.Bounds())
		if dr.Empty() {
			continue
		}

		draw.Draw(dst, dr, src, dr.Min, draw.Over)
	}

	return dst, nil
}

func (impl) Encode(w io.Writer, img image.Image, _ opts.Common) error {
	return fmt.Errorf("xcf: encoding not supported")
}

func applyOpacity(img image.Image, opacity uint8) *image.NRGBA {
	bounds := img.Bounds()
	out := image.NewNRGBA(bounds)

	opa := int(opacity)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()

			r := uint8(r16 >> 8)
			g := uint8(g16 >> 8)
			b := uint8(b16 >> 8)
			a := int(uint8(a16 >> 8))

			a = (a*opa + 127) / 255

			out.Set(x, y, color.NRGBA{R: r, G: g, B: b, A: uint8(a)})
		}
	}

	return out
}
