package dds

import (
	"fmt"
	"io"
)

func (impl) Help(writer io.Writer) error {
	fmt.Fprintln(writer, `
NOTES:
   DDS-to-DDS preserves the complete texture by default. Surface selectors choose
   image scope; default extraction is mip=0, item=0, slice=0.
   Use --dds.scope image for thumbnails, effects, join/split, playback or frame/time.
   Image extraction is strict and precision-preserving. HDR/signed/integer data
   needs --dds.display clamp; this is not tone mapping or normal reconstruction.
   --dds.input-alpha declares existing samples; --dds.alpha converts association.
   BC encoding is lossy and --quality is unsupported. --lossless permits unchanged
   BC payloads, not BC encoding/recompression or lossy precision/range reduction.

NAMED FLAGS:
   Comma-separated, case-insensitive; default/none (or empty) means no flags.
   One filter mode, one dither mode, no wrap+mirror on the same axis.
   Addressing/filter modes are mipmap-only; separate-alpha requires native mips.`)

	fmt.Fprintf(writer, "   load: %s\n", bitNames(loadNames))
	fmt.Fprintf(writer, "   filters: %s\n", bitNames(filterNames))
	fmt.Fprintf(writer, "   compression: %s\n", bitNames(compressionNames))
	fmt.Fprintln(writer, "\nFORMATS:")
	fmt.Fprintln(writer, "   Case-insensitive DXGI names, with optional DXGI_FORMAT_ prefix.")
	fmt.Fprint(writer, "   Aliases: ")

	for index, entry := range formatAliases {
		if index > 0 {
			fmt.Fprint(writer, ", ")
		}

		fmt.Fprint(writer, entry.name)
	}

	_, err := fmt.Fprintln(writer, `
   DXT2/4: use bc2/bc3 with --dds.alpha premultiplied instead.
   Format capabilities and color/alpha details: https://pkg.go.dev/github.com/coalaura/dds

EXAMPLES:
   ffwebp -i image.png -o image.dds --lossless
   ffwebp -i image.png -o bc7.dds --dds.format bc7-srgb --dds.mipmaps full
   ffwebp -i cube.dds -o face.png --dds.cube 1 --dds.face nz --dds.mip 2`)

	return err
}
