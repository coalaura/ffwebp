package svg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/kanrichan/resvg-go"
	"github.com/urfave/cli/v3"
)

var (
	svgWidth      int
	svgHeight     int
	svgBackground string
	svgDpi        float64
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "svg"
}

func (impl) Extensions() []string {
	return []string{"svg"}
}

func (impl) CanEncode() bool {
	return false
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "svg.width",
			Usage:       "SVG: output width in pixels (0 = auto from viewBox)",
			Value:       0,
			Destination: &svgWidth,
		},
		&cli.IntFlag{
			Name:        "svg.height",
			Usage:       "SVG: output height in pixels (0 = auto from viewBox)",
			Value:       0,
			Destination: &svgHeight,
		},
		&cli.StringFlag{
			Name:        "svg.background",
			Usage:       "SVG: background color (CSS color or transparent)",
			Value:       "transparent",
			Destination: &svgBackground,
		},
		&cli.Float64Flag{
			Name:        "svg.dpi",
			Usage:       "SVG: dots per inch for rendering",
			Value:       96.0,
			Destination: &svgDpi,
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 256)

	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, nil, err
	}

	buf = buf[:n]

	if strings.Contains(strings.ToLower(string(buf)), "<svg") {
		sniff := buf

		if len(sniff) > 64 {
			sniff = sniff[:64]
		}

		return 80, sniff, nil
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	svgData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(svgData) == 0 {
		return nil, fmt.Errorf("svg: empty input")
	}

	ctx := context.Background()

	resvgCtx, err := resvg.NewContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("svg: failed to create resvg context: %w", err)
	}

	defer resvgCtx.Close()

	renderer, err := resvgCtx.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("svg: failed to create renderer: %w", err)
	}

	defer renderer.Close()

	err = renderer.SetDpi(float32(svgDpi))
	if err != nil {
		return nil, fmt.Errorf("svg: failed to set DPI: %w", err)
	}

	err = renderer.LoadSystemFonts()
	if err != nil {
		logx.Printf("svg: warning: failed to load system fonts: %v\n", err)
	}

	var pngData []byte

	if svgWidth > 0 || svgHeight > 0 {
		tmpPng, err := renderer.Render(svgData)
		if err == nil && len(tmpPng) > 0 {
			cfg, err := png.DecodeConfig(bytes.NewReader(tmpPng))
			if err == nil {
				origW, origH := int(cfg.Width), int(cfg.Height)

				w, h := calculateSize(origW, origH, svgWidth, svgHeight)

				logx.Printf("svg: rendering at %dx%d (original: %dx%d)\n", w, h, origW, origH)

				pngData, err = renderer.RenderWithSize(svgData, uint32(w), uint32(h))
				if err != nil {
					return nil, fmt.Errorf("svg: render with size failed: %w", err)
				}
			}
		}
	}

	if len(pngData) == 0 {
		pngData, err = renderer.Render(svgData)
		if err != nil {
			return nil, fmt.Errorf("svg: render failed: %w", err)
		}
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("svg: failed to decode rendered PNG: %w", err)
	}

	if svgBackground != "" && !strings.EqualFold(svgBackground, "transparent") {
		img = applyBackground(img, svgBackground)
	}

	return img, nil
}

func (impl) Encode(w io.Writer, img image.Image, options opts.Common) error {
	return fmt.Errorf("svg: encoding not supported")
}

func calculateSize(origW, origH, targetW, targetH int) (int, int) {
	w, h := origW, origH

	if targetW > 0 && targetH > 0 {
		return targetW, targetH
	} else if targetW > 0 {
		w = targetW

		if origH > 0 && origW > 0 {
			h = targetW * origH / origW
		}
	} else if targetH > 0 {
		h = targetH

		if origW > 0 && origH > 0 {
			w = targetH * origW / origH
		}
	}

	if w < 1 {
		w = 1
	}

	if h < 1 {
		h = 1
	}

	return w, h
}

func applyBackground(img image.Image, bg string) image.Image {
	bgColor := parseColor(bg)
	if bgColor == nil {
		return img
	}

	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)

	draw.Draw(rgba, rgba.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Over)

	return rgba
}

func parseColor(s string) color.Color {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "white", "#fff", "#ffffff":
		return color.White
	case "black", "#000", "#000000":
		return color.Black
	case "red":
		return color.RGBA{R: 255, G: 0, B: 0, A: 255}
	case "green":
		return color.RGBA{R: 0, G: 255, B: 0, A: 255}
	case "blue":
		return color.RGBA{R: 0, G: 0, B: 255, A: 255}
	case "yellow":
		return color.RGBA{R: 255, G: 255, B: 0, A: 255}
	case "cyan", "aqua":
		return color.RGBA{R: 0, G: 255, B: 255, A: 255}
	case "magenta", "fuchsia":
		return color.RGBA{R: 255, G: 0, B: 255, A: 255}
	case "silver":
		return color.RGBA{R: 192, G: 192, B: 192, A: 255}
	case "gray", "grey":
		return color.RGBA{R: 128, G: 128, B: 128, A: 255}
	case "maroon":
		return color.RGBA{R: 128, G: 0, B: 0, A: 255}
	case "olive":
		return color.RGBA{R: 128, G: 128, B: 0, A: 255}
	case "lime":
		return color.RGBA{R: 0, G: 255, B: 0, A: 255}
	case "teal":
		return color.RGBA{R: 0, G: 128, B: 128, A: 255}
	case "navy":
		return color.RGBA{R: 0, G: 0, B: 128, A: 255}
	case "purple":
		return color.RGBA{R: 128, G: 0, B: 128, A: 255}
	}

	if strings.HasPrefix(s, "#") {
		s = s[1:]

		if len(s) == 3 {
			s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
		}

		if len(s) == 6 {
			var r, g, b uint8

			_, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
			if err == nil {
				return color.RGBA{R: r, G: g, B: b, A: 255}
			}
		}
	}

	return nil
}
