package main

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"io"

	ico "github.com/biessek/golang-ico"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func ReadImage(input io.ReadSeeker) (image.Image, error) {
	decoder, err := GetDecoderFromContent(input)
	if err != nil {
		return nil, err
	}

	return decoder(input)
}

func WriteImage(output io.Writer, img image.Image, format string) error {
	switch format {
	case "webp":
		options := GetWebPOptions()

		LogWebPOptions(options)

		return webp.Encode(output, img, options)
	case "jpeg":
		options := GetJpegOptions()

		LogJpegOptions(options)

		return jpeg.Encode(output, img, options)
	case "png":
		encoder := GetPNGOptions()

		LogPNGOptions(encoder)

		return encoder.Encode(output, img)
	case "gif":
		options := GetGifOptions()

		LogGifOptions(options)

		return gif.Encode(output, img, options)
	case "bmp":
		return bmp.Encode(output, img)
	case "tiff":
		options := GetTiffOptions()

		LogTiffOptions(options)

		return tiff.Encode(output, img, options)
	case "avif":
		options := GetAvifOptions()

		LogAvifOptions(options)

		return avif.Encode(output, img, options)
	case "jxl":
		options := GetJxlOptions()

		LogJxlOptions(options)

		jpegxl.Encode(output, img, options)
	case "ico":
		return ico.Encode(output, img)
	}

	return fmt.Errorf("unsupported output format: %s", format)
}
