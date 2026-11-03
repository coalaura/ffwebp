package main

import (
	"fmt"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
)

func validateOutputICC(output codec.Codec, common opts.Common) error {
	if len(common.ICC) == 0 || !common.Lossless {
		return nil
	}

	switch output.String() {
	case "jpeg2000", "jpegxl", "png":
		return nil
	default:
		return fmt.Errorf("%s output cannot preserve the decoded ICC profile losslessly; use JP2, JPEG XL or PNG, or convert the profile and pixels to sRGB first", output)
	}
}
