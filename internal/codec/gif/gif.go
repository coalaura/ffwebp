package gif

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"math/rand/v2"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/ericpauley/go-quantize/quantize"
	"github.com/urfave/cli/v3"
)

const paletteSamples = 65536

type impl struct{}

var numColors int

func init() {
	codec.Register(impl{})
}

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
		Usage:       "GIF: Total palette colors, including transparency (1-256)",
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

	_, err := reader.ReadAt(buf, 0)
	if err != nil {
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

	initialBackground := frameBackground(g.Image[0], background)

	draw.Draw(canvas, bounds, &image.Uniform{C: initialBackground}, image.Point{}, draw.Src)

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
			case 2: // Transparent frames restore to a transparent canvas, not the RGB global table.
				clearColor := frameBackground(srcFrame, background)

				draw.Draw(canvas, srcFrame.Bounds(), &image.Uniform{C: clearColor}, image.Point{}, draw.Src)
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
		Background: initialBackground,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("gif: colors=%d\n", numColors)

	frames := []image.Image{img}

	pal, err := makePalette(frames)
	if err != nil {
		return err
	}

	return gif.Encode(writer, toPaletted(img, pal), nil)
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

	pal, err := makePalette(anim.Frames)
	if err != nil {
		return err
	}

	framesOut := make([]*image.Paletted, len(anim.Frames))
	disposal := make([]byte, len(anim.Frames))

	bounds := anim.Frames[0].Bounds()

	for i, frame := range anim.Frames {
		if frame.Bounds().Size() != bounds.Size() {
			return fmt.Errorf("gif: frame %d has different dimensions", i)
		}

		framesOut[i] = toPaletted(frame, pal)

		// Frames are complete composites. Clear each one so disappearing pixels cannot trail.
		disposal[i] = gif.DisposalBackground
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

	backgroundIndex := uint8(0)

	_, _, _, alpha := pal[len(pal)-1].RGBA()
	if alpha == 0 {
		backgroundIndex = uint8(len(pal) - 1)
	} else {
		backgroundIndex = uint8(pal.Index(anim.Background))
	}

	gifAnim := &gif.GIF{
		Image:           framesOut,
		Delay:           delays,
		Disposal:        disposal,
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

func frameBackground(frame *image.Paletted, background color.RGBA) color.RGBA {
	for _, entry := range frame.Palette {
		_, _, _, alpha := entry.RGBA()
		if alpha == 0 {
			return color.RGBA{}
		}
	}

	return background
}

func makePalette(frames []image.Image) (color.Palette, error) {
	if numColors < 1 || numColors > 256 {
		return nil, fmt.Errorf("gif: invalid color budget %d (want 1-256)", numColors)
	}

	// Reservoir sampling bounds the quantizer's memory while considering opaque pixels in every frame.
	samples := image.NewRGBA(image.Rect(0, 0, paletteSamples, 1))
	random := rand.New(rand.NewPCG(0, 0))

	var (
		opaque      int
		transparent bool
	)

	for index, frame := range frames {
		if frame == nil || frame.Bounds().Empty() {
			return nil, fmt.Errorf("gif: frame %d is nil or empty", index)
		}

		bounds := frame.Bounds()

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				pixel := nrgbaAt(frame, x, y)
				if pixel.A < 128 {
					transparent = true

					continue
				}

				slot := opaque
				opaque++

				if opaque > paletteSamples {
					slot = random.IntN(opaque)
				}

				if slot < paletteSamples {
					samples.SetRGBA(slot, 0, color.RGBA{R: pixel.R, G: pixel.G, B: pixel.B, A: 255})
				}
			}
		}
	}

	budget := numColors

	if transparent {
		budget--
	}

	if opaque > 0 && budget == 0 {
		return nil, fmt.Errorf("gif: --gif.colors=%d cannot represent both transparent and opaque pixels (need at least 2)", numColors)
	}

	pal := make(color.Palette, 0, numColors)

	if opaque > 0 {
		samples.Rect.Max.X = min(opaque, paletteSamples)

		quantizer := quantize.MedianCutQuantizer{Aggregation: quantize.Mean}
		quantized := quantizer.Quantize(pal[:0:budget], samples)

		pal = pal[:len(quantized)]
	}

	if transparent {
		pal = append(pal, color.NRGBA{})
	}

	return pal, nil
}

func toPaletted(img image.Image, pal color.Palette) *image.Paletted {
	bounds := img.Bounds()
	outputBounds := image.Rectangle{Max: bounds.Size()}
	opaqueColors := len(pal)

	_, _, _, alpha := pal[len(pal)-1].RGBA()
	if alpha == 0 {
		opaqueColors--
	}

	out := image.NewPaletted(outputBounds, pal)

	if opaqueColors == 0 {
		return out
	}

	// Dither opaque RGB only, then restore the exact alpha mask. The transparent
	// entry must never compete with opaque colors in the nearest-color search.
	normalized := image.NewNRGBA(outputBounds)
	filler := color.NRGBAModel.Convert(pal[0]).(color.NRGBA)

	for y := range outputBounds.Dy() {
		for x := range outputBounds.Dx() {
			pixel := nrgbaAt(img, x+bounds.Min.X, y+bounds.Min.Y)
			if pixel.A < 128 {
				pixel = filler
			}

			pixel.A = 255
			normalized.SetNRGBA(x, y, pixel)
		}
	}

	out.Palette = pal[:opaqueColors]

	draw.FloydSteinberg.Draw(out, outputBounds, normalized, image.Point{})

	out.Palette = pal

	if opaqueColors < len(pal) {
		for y := range outputBounds.Dy() {
			for x := range outputBounds.Dx() {
				pixel := nrgbaAt(img, x+bounds.Min.X, y+bounds.Min.Y)
				if pixel.A < 128 {
					out.SetColorIndex(x, y, uint8(opaqueColors))
				}
			}
		}
	}

	return out
}

func nrgbaAt(img image.Image, x, y int) color.NRGBA {
	// Concrete accessors avoid boxing a color for each sampled or mapped pixel.
	switch source := img.(type) {
	case *image.NRGBA:
		return source.NRGBAAt(x, y)
	case *image.RGBA:
		pixel := source.RGBAAt(x, y)
		if pixel.A == 0 {
			return color.NRGBA{}
		}

		return color.NRGBA{
			R: uint8(uint32(pixel.R) * 0xffff / uint32(pixel.A) >> 8),
			G: uint8(uint32(pixel.G) * 0xffff / uint32(pixel.A) >> 8),
			B: uint8(uint32(pixel.B) * 0xffff / uint32(pixel.A) >> 8),
			A: pixel.A,
		}
	case *image.YCbCr:
		pixel := source.YCbCrAt(x, y)
		red, green, blue := color.YCbCrToRGB(pixel.Y, pixel.Cb, pixel.Cr)

		return color.NRGBA{R: red, G: green, B: blue, A: 255}
	default:
		return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	}
}
