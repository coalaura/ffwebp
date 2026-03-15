//go:build !play

package play

import (
	"errors"
	"image"

	"github.com/coalaura/ffwebp/internal/codec"
)

var errNoPlay = errors.New("this binary was compiled without GUI support. Recompile with '-tags play' to use the --play feature")

func PlayImage(img image.Image) error {
	return errNoPlay
}

func PlayAnimation(anim *codec.Animation) error {
	return errNoPlay
}
