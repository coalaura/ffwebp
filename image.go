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

type Encoder func(io.Writer, image.Image) error

func ReadImage(input io.ReadSeeker) (image.Image, error) {
	decoder, err := GetDecoderFromContent(input)
	if err != nil {
		return nil, err
	}

	return decoder(input)
}

func ResolveImageEncoder() (Encoder, error) {
	table := NewOptionsTable()

	switch opts.Format {
	case "webp":
		options := GetWebPOptions()

		table.AddWebPOptions(options)

		return func(output io.Writer, img image.Image) error {
			return webp.Encode(output, img, options)
		}, nil
	case "jpeg":
		options := GetJpegOptions()

		table.AddJpegOptions(options)

		return func(output io.Writer, img image.Image) error {
			return jpeg.Encode(output, img, options)
		}, nil
	case "png":
		encoder := GetPNGOptions()

		table.AddPNGOptions(encoder)

		return func(output io.Writer, img image.Image) error {
			return encoder.Encode(output, img)
		}, nil
	case "gif":
		options := GetGifOptions()

		table.AddGifOptions(options)

		return func(output io.Writer, img image.Image) error {
			return gif.Encode(output, img, options)
		}, nil
	case "bmp":
		return func(output io.Writer, img image.Image) error {
			return bmp.Encode(output, img)
		}, nil
	case "tiff":
		options := GetTiffOptions()

		table.AddTiffOptions(options)

		return func(output io.Writer, img image.Image) error {
			return tiff.Encode(output, img, options)
		}, nil
	case "avif":
		options := GetAvifOptions()

		table.AddAvifOptions(options)

		return func(output io.Writer, img image.Image) error {
			return avif.Encode(output, img, options)
		}, nil
	case "jxl":
		options := GetJxlOptions()

		table.AddJxlOptions(options)

		return func(output io.Writer, img image.Image) error {
			return jpegxl.Encode(output, img, options)
		}, nil
	case "ico":
		table.Print()

		return func(output io.Writer, img image.Image) error {
			return ico.Encode(output, img)
		}, nil
	}

	return nil, fmt.Errorf("unsupported output format: %s", opts.Format)
}
