package main

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"

	"github.com/biessek/golang-ico"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

var (
	options = map[string]string{
		"c / colors":      "Number of colors (1-256) (gif)",
		"e / effort":      "Encoder effort level (0-10) (jxl)",
		"f / format":      "Output format (avif, bmp, gif, jpeg, jxl, png, tiff, webp)",
		"h / help":        "Show this help page",
		"l / lossless":    "Use lossless compression (webp)",
		"m / method":      "Encoder method (0=fast, 6=slower-better) (webp)",
		"r / ratio":       "YCbCr subsample-ratio (0=444, 1=422, 2=420, 3=440, 4=411, 5=410) (avif)",
		"s / silent":      "Do not print any output",
		"q / quality":     "Set quality (0-100) (avif, jpeg, jxl, webp)",
		"x / exact":       "Preserve RGB values in transparent area (webp)",
		"z / compression": "Compression type (0=uncompressed, 1=deflate, 2=lzw, 3=ccittgroup3, 4=ccittgroup4) (tiff)",
	}
)

func ReadImage(input *os.File) (image.Image, error) {
	decoder, err := GetDecoderFromContent(input)
	if err != nil {
		return nil, err
	}

	return decoder(input)
}

func WriteImage(output *os.File, img image.Image, format string) error {
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
		return png.Encode(output, img)
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
