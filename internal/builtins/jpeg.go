//go:build jpeg || core || full
// +build jpeg core full

package builtins

import (
	_ "github.com/coalaura/ffwebp/internal/codec/jpeg"
)
