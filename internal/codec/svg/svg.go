package svg

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/urfave/cli/v3"
)

var (
	svgWidth      int
	svgHeight     int
	svgBackground string
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
			Usage:       "SVG: output width in pixels (0 = auto)",
			Value:       0,
			Destination: &svgWidth,
		},
		&cli.IntFlag{
			Name:        "svg.height",
			Usage:       "SVG: output height in pixels (0 = auto)",
			Value:       0,
			Destination: &svgHeight,
		},
		&cli.StringFlag{
			Name:        "svg.background",
			Usage:       "SVG: background color",
			Value:       "",
			Destination: &svgBackground,
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
	icon, err := oksvg.ReadIconStream(r)
	if err != nil {
		return nil, err
	}

	vbw := int(icon.ViewBox.W)
	vbh := int(icon.ViewBox.H)

	var w, h int

	switch {
	case svgWidth > 0 && svgHeight > 0:
		w, h = svgWidth, svgHeight
	case svgWidth > 0 && svgHeight == 0:
		w = svgWidth

		if vbw > 0 && vbh > 0 {
			h = int(float64(svgWidth) * float64(vbh) / float64(vbw))
		} else {
			h = svgWidth
		}
	case svgHeight > 0 && svgWidth == 0:
		h = svgHeight

		if vbw > 0 && vbh > 0 {
			w = int(float64(svgHeight) * float64(vbw) / float64(vbh))
		} else {
			w = svgHeight
		}
	default:
		if vbw > 0 && vbh > 0 {
			w, h = vbw, vbh
		} else {
			w, h = 256, 256
		}
	}

	if w <= 0 {
		w = 256
	}

	if h <= 0 {
		h = 256
	}

	logx.Printf("svg: size=%dx%d background=%s\n", w, h, svgBackground)

	rgba := image.NewRGBA(image.Rect(0, 0, w, h))

	if svgBackground != "" && !strings.EqualFold(svgBackground, "transparent") {
		bgc, err := oksvg.ParseSVGColor(svgBackground)
		if err != nil {
			return nil, err
		} else if bgc == nil {
			return nil, fmt.Errorf("invalid svg.background: %s", svgBackground)
		}

		draw.Draw(rgba, rgba.Bounds(), &image.Uniform{bgc}, image.Point{}, draw.Src)
	}

	icon.SetTarget(0, 0, float64(w), float64(h))

	scanner := rasterx.NewScannerGV(w, h, rgba, rgba.Bounds())

	d := rasterx.NewDasher(w, h, scanner)

	icon.Draw(d, 1.0)

	return rgba, nil
}

func (impl) Encode(w io.Writer, img image.Image, options opts.Common) error {
	return errors.New("svg: encoding not supported")
}
