//go:build effects || core || full
// +build effects core full

package builtins

import (
	_ "github.com/coalaura/ffwebp/internal/effects/blur"
	_ "github.com/coalaura/ffwebp/internal/effects/brightness"
	_ "github.com/coalaura/ffwebp/internal/effects/contrast"
	_ "github.com/coalaura/ffwebp/internal/effects/grayscale"
	_ "github.com/coalaura/ffwebp/internal/effects/hue"
	_ "github.com/coalaura/ffwebp/internal/effects/invert"
	_ "github.com/coalaura/ffwebp/internal/effects/saturation"
	_ "github.com/coalaura/ffwebp/internal/effects/sepia"
	_ "github.com/coalaura/ffwebp/internal/effects/sharpen"
)
