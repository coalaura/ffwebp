package hue

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
	return "hue"
}

func (impl) Apply(img image.Image, args string) (image.Image, error) {
	var change int64 = 10

	if args != "" {
		i64, err := strconv.ParseInt(args, 10, 64)
		if err != nil || i64 < -360 || i64 > 360 || i64 == 0 {
			return nil, fmt.Errorf("invalid hue change: %s", args)
		}

		change = i64
	}

	logx.Printf("  applying hue (change=%d)\n", change)

	return adjust.Hue(img, int(change)), nil
}
