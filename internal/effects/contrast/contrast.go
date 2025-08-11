package contrast

import (
	"fmt"
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/logx"
)

type impl struct{}

func init() {
	effects.Register(impl{})
}

func (impl) String() string {
	return "contrast"
}

func (impl) Apply(img image.Image, args string) (image.Image, error) {
	var change float64 = 0.1

	if args != "" {
		f64, err := strconv.ParseFloat(args, 64)
		if err != nil || f64 < -1 || f64 > 1 || f64 == 0 {
			return nil, fmt.Errorf("invalid contrast change: %s", args)
		}

		change = f64
	}

	logx.Printf("  applying contrast (change=%.f)\n", change)

	return adjust.Contrast(img, change), nil
}
