package sepia

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
	return "sepia"
}

func (impl) Apply(img image.Image, _ string) (image.Image, error) {
	logx.Printf("  applying sepia\n")

	return effect.Sepia(img), nil
}
