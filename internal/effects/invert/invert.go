package invert

import (
	"image"

	"github.com/anthonynsimon/bild/effect"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/logx"
)

type impl struct{}

func init() {
	effects.Register(impl{})
}

func (impl) String() string {
	return "invert"
}

func (impl) Apply(img image.Image, _ string) (image.Image, error) {
	logx.Printf("  applying invert\n")

	return effect.Invert(img), nil
}
