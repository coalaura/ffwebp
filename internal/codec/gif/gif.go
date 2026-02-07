package gif

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var numColors int

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "gif"
}

func (impl) Extensions() []string {
	return []string{"gif"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:        "gif.colors",
		Usage:       "GIF: Number of colors (1-256)",
		Value:       256,
		Destination: &numColors,
		Validator: func(value int) error {
			if value < 1 || value > 256 {
				return fmt.Errorf("invalid gif.colors: %d", value)
			}

			return nil
		},
	})
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic7a := []byte("GIF87a")
	magic9a := []byte("GIF89a")

	buf := make([]byte, 6)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic7a) {
		return 100, magic7a, nil
	}

	if bytes.Equal(buf, magic9a) {
		return 100, magic9a, nil
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return gif.Decode(reader)
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	g, err := gif.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	if len(g.Image) == 0 {
		return nil, fmt.Errorf("gif: no frames")
	}

	background := color.RGBA{0, 0, 0, 0}

	if g.Config.ColorModel != nil {
		if p, ok := g.Config.ColorModel.(color.Palette); ok && int(g.BackgroundIndex) < len(p) {
			background = color.RGBAModel.Convert(p[g.BackgroundIndex]).(color.RGBA)
		}
	}

	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	canvas := image.NewRGBA(bounds)

	draw.Draw(canvas, bounds, &image.Uniform{background}, image.Point{}, draw.Src)

	frames := make([]image.Image, len(g.Image))

	var prevCanvas *image.RGBA // For disposal method 3 (restore to previous)

	for i, srcFrame := range g.Image {
		if i < len(g.Disposal) && g.Disposal[i] == 3 {
			prevCanvas = image.NewRGBA(bounds)

			draw.Draw(prevCanvas, bounds, canvas, bounds.Min, draw.Src)
		}

		draw.Draw(canvas, srcFrame.Bounds(), srcFrame, srcFrame.Bounds().Min, draw.Over)

		frameCopy := image.NewRGBA(bounds)

		draw.Draw(frameCopy, bounds, canvas, bounds.Min, draw.Src)

		frames[i] = frameCopy

		if i < len(g.Image)-1 {
			disposal := byte(0)

			if i < len(g.Disposal) {
				disposal = g.Disposal[i]
			}

			switch disposal {
			case 0, 1: // No disposal specified or do not dispose - keep canvas as is
				// Do nothing
			case 2: // Restore to background color - clear frame area to background
				draw.Draw(canvas, srcFrame.Bounds(), &image.Uniform{background}, image.Point{}, draw.Src)
			case 3: // Restore to previous - restore canvas to state before this frame
				if prevCanvas != nil {
					draw.Draw(canvas, bounds, prevCanvas, bounds.Min, draw.Src)
				}
			}
		}
	}

	delays := make([]int, len(g.Delay))

	for i, delay := range g.Delay {
		if delay < 0 {
			delay = 0
		}

		delays[i] = delay * 10
	}

	if len(delays) != len(frames) {
		fixed := make([]int, len(frames))

		copy(fixed, delays)

		delays = fixed
	}

	return &codec.Animation{
		Frames:     frames,
		Delays:     delays,
		LoopCount:  g.LoopCount,
		Background: background,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("gif: colors=%d\n", numColors)

	return gif.Encode(writer, img, &gif.Options{
		NumColors: numColors,
	})
}

func (impl) EncodeAll(writer io.Writer, anim *codec.Animation, options opts.Common) error {
	var frames int

	if anim != nil {
		frames = len(anim.Frames)
	}

	logx.Printf("gif: frames=%d colors=%d\n", frames, numColors)

	if anim == nil {
		return fmt.Errorf("gif: animation is nil")
	}

	if len(anim.Frames) == 0 {
		return fmt.Errorf("gif: animation has no frames")
	}

	pal := color.Palette(palette.Plan9)

	if numColors > 0 && numColors < len(pal) {
		pal = pal[:numColors]
	}

	framesOut := make([]*image.Paletted, len(anim.Frames))

	for i, frame := range anim.Frames {
		if frame == nil {
			return fmt.Errorf("gif: frame %d is nil", i)
		}

		framesOut[i] = toPaletted(frame, pal)
	}

	delays := make([]int, len(anim.Delays))

	copy(delays, anim.Delays)

	if len(delays) != len(framesOut) {
		fixed := make([]int, len(framesOut))

		copy(fixed, delays)

		delays = fixed
	}

	for i, delay := range delays {
		if delay < 0 {
			delay = 0
		}

		delays[i] = (delay + 5) / 10
	}

	bounds := framesOut[0].Bounds()

	backgroundIndex := uint8(0)

	if len(pal) > 0 {
		backgroundIndex = uint8(pal.Index(anim.Background))
	}

	gifAnim := &gif.GIF{
		Image:           framesOut,
		Delay:           delays,
		LoopCount:       anim.LoopCount,
		BackgroundIndex: backgroundIndex,
		Config: image.Config{
			ColorModel: pal,
			Width:      bounds.Dx(),
			Height:     bounds.Dy(),
		},
	}

	return gif.EncodeAll(writer, gifAnim)
}

func toPaletted(img image.Image, pal color.Palette) *image.Paletted {
	bounds := img.Bounds()

	if paletted, ok := img.(*image.Paletted); ok && samePalette(paletted.Palette, pal) {
		return paletted
	}

	out := image.NewPaletted(bounds, pal)

	draw.FloydSteinberg.Draw(out, bounds, img, bounds.Min)

	return out
}

func samePalette(a, b color.Palette) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
