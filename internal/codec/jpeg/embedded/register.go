// Package embedded registers JPEG for libraries that use image.Decode internally.
package embedded

import (
	"image"

	"github.com/coalaura/jpeg/classic"
)

func init() {
	// Keep this registration independent of the CLI so it precedes TGA's
	// catch-all image handler. TIFF relies on it for embedded JPEG strips.
	// ffwebp's top-level sniffer separately distinguishes JPEG from JPEG-LS.
	image.RegisterFormat("jpeg", "\xff\xd8\xff", classic.Decode, classic.DecodeConfig)
}
