package blur

import (
	"fmt"
	"image"
	"strconv"

	"github.com/anthonynsimon/bild/blur"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/logx"
)

type impl struct{}

func init() {
	effects.Register(impl{})
}

func (impl) String() string {
	return "blur"
}

func (impl) Apply(img image.Image, args string) (image.Image, error) {
	var radius float64 = 3

	if args != "" {
		f64, err := strconv.ParseFloat(args, 64)
		if err != nil || f64 <= 0 {
			return nil, fmt.Errorf("invalid blur radius: %s", args)
		}

		radius = f64
	}

	logx.Printf("  applying blur (radius=%.f)\n", radius)

	return blur.Gaussian(img, radius), nil
}
