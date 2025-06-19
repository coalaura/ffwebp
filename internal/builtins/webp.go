//go:build webp || core || full
// +build webp core full

package builtins

import (
	_ "github.com/coalaura/ffwebp/internal/codec/webp"
)
