package pnm

import (
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/spakin/netpbm"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var (
	plain     bool
	maxValue  uint16
	formatStr string
	tupleType string
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "pnm"
}

func (impl) Extensions() []string {
	return []string{"ppm", "pgm", "pnm", "pbm", "pam"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.BoolFlag{
			Name:        "pnm.plain",
			Usage:       "PNM: produce plain (ASCII) format (P1/P2/P3/P7 with ASCII raster)",
			Value:       false,
			Destination: &plain,
		},
		&cli.Uint16Flag{
			Name:        "pnm.maxval",
			Usage:       "PNM: maximum sample value (1..65535). Controls 1- or 2-byte samples for binary formats",
			Value:       255,
			Destination: &maxValue,
		},
		&cli.StringFlag{
			Name:        "pnm.format",
			Usage:       "PNM: force output subformat (pbm, pgm, ppm, pam). If empty the codec will use the extension of the output file or infer it from the image.",
			Value:       "",
			Destination: &formatStr,
		},
		&cli.StringFlag{
			Name:        "pnm.pam-tupletype",
			Usage:       "PNM: when writing PAM (P7), set TUPLETYPE (e.g. RGB, RGB_ALPHA, GRAYSCALE). If empty a sensible value is chosen.",
			Value:       "",
			Destination: &tupleType,
		},
	)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 2)

	if _, err := reader.ReadAt(buf, 0); err != nil {
		return 0, nil, err
	}

	if buf[0] == 'P' && buf[1] >= '1' && buf[1] <= '7' {
		return 100, buf, nil
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	return netpbm.Decode(r, nil)
}

func (impl) Encode(w io.Writer, img image.Image, common opts.Common) error {
	var (
		format netpbm.Format
		found  bool
	)

	if formatStr != "" {
		f, ok := pnmFormat(formatStr)
		if !ok {
			return fmt.Errorf("invalid pnm.format: %q", formatStr)
		}

		found = true
		format = f
	} else if common.OutputExtension != "" {
		f, ok := pnmFormat(common.OutputExtension)
		if ok {
			found = true
			format = f
		}
	}

	if !found {
		if imageHasAlpha(img) {
			format = netpbm.PAM
		} else if imageIsGrayscale(img) {
			format = netpbm.PGM
		} else {
			format = netpbm.PPM
		}
	}

	opts := &netpbm.EncodeOptions{
		Format:    format,
		Plain:     plain,
		MaxValue:  maxValue,
		TupleType: tupleType,
	}

	if opts.Format == netpbm.PBM {
		opts.MaxValue = 1
	}

	logx.Printf("pnm: format=%s plain=%t maxval=%d tupltype=%q\n", opts.Format.String(), opts.Plain, opts.MaxValue, opts.TupleType)

	return netpbm.Encode(w, img, opts)
}

func pnmFormat(format string) (netpbm.Format, bool) {
	switch strings.ToLower(format) {
	case "pbm":
		return netpbm.PBM, true
	case "pgm":
		return netpbm.PGM, true
	case "ppm":
		return netpbm.PPM, true
	case "pam":
		return netpbm.PAM, true
	}

	return 0, false
}

func imageHasAlpha(img image.Image) bool {
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()

			if a != 0xFFFF {
				return true
			}
		}
	}

	return false
}

func imageIsGrayscale(img image.Image) bool {
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()

			if r != g || r != b {
				return false
			}
		}
	}

	return true
}
