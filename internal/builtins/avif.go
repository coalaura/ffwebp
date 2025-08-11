//go:build avif || core || full
// +build avif core full

package builtins

import (
	_ "github.com/coalaura/ffwebp/internal/codec/avif"
)
