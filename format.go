package main

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/biessek/golang-ico"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/heic"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

var (
	OutputFormats = []string{
		"jpeg",
		"png",
		"webp",
		"gif",
		"bmp",
		"tiff",
		"avif",
		"jxl",
		"ico",
	}

	InputFormats = []string{
		"jpeg",
		"png",
		"webp",
		"gif",
		"bmp",
		"tiff",
		"avif",
		"jxl",
		"ico",
		"heic",
		"heif",
	}
)

type Decoder func(io.Reader) (image.Image, error)

func GetDecoderFromContent(in *os.File) (Decoder, error) {
	buffer := make([]byte, 128)

	_, err := in.Read(buffer)
	if err != nil {
		return nil, err
	}

	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if IsJPEG(buffer) {
		return jpeg.Decode, nil
	} else if IsPNG(buffer) {
		return png.Decode, nil
	} else if IsGIF(buffer) {
		return gif.Decode, nil
	} else if IsBMP(buffer) {
		return bmp.Decode, nil
	} else if IsWebP(buffer) {
		return webp.Decode, nil
	} else if IsTIFF(buffer) {
		return tiff.Decode, nil
	} else if IsICO(buffer) {
		return ico.Decode, nil
	} else if IsHEIC(buffer) {
		return heic.Decode, nil
	} else if IsAVIF(buffer) {
		return avif.Decode, nil
	} else if IsJpegXL(buffer) {
		return jpegxl.Decode, nil
	}

	return nil, fmt.Errorf("unsupported input format")
}

func IsJPEG(buffer []byte) bool {
	return len(buffer) > 2 && buffer[0] == 0xFF && buffer[1] == 0xD8
}

func IsPNG(buffer []byte) bool {
	return len(buffer) > 8 && string(buffer[:8]) == "\x89PNG\r\n\x1a\n"
}

func IsGIF(buffer []byte) bool {
	return len(buffer) > 6 && (string(buffer[:6]) == "GIF87a" || string(buffer[:6]) == "GIF89a")
}

func IsBMP(buffer []byte) bool {
	return len(buffer) > 2 && string(buffer[:2]) == "BM"
}

func IsICO(buffer []byte) bool {
	return len(buffer) > 4 && buffer[0] == 0x00 && buffer[1] == 0x00 && buffer[2] == 0x01 && buffer[3] == 0x00
}

func IsWebP(buffer []byte) bool {
	// Check if its VP8L
	if len(buffer) > 16 && string(buffer[12:16]) == "VP8L" {
		return true
	}

	// Check if its WebP or RIFF WEBP
	return len(buffer) > 12 && string(buffer[:4]) == "RIFF" && string(buffer[8:12]) == "WEBP"
}

func IsAVIF(buffer []byte) bool {
	return len(buffer) > 12 && string(buffer[4:8]) == "ftyp" && string(buffer[8:12]) == "avif"
}

func IsTIFF(buffer []byte) bool {
	return len(buffer) > 4 && (string(buffer[:4]) == "II*\x00" || string(buffer[:4]) == "MM\x00*")
}

func IsHEIC(buffer []byte) bool {
	return len(buffer) > 12 && string(buffer[4:8]) == "ftyp" && (string(buffer[8:12]) == "heic" || string(buffer[8:12]) == "heix")
}

func IsJpegXL(buffer []byte) bool {
	// Check for JPEG XL codestream (starts with 0xFF 0x0A)
	if len(buffer) > 2 && buffer[0] == 0xFF && buffer[1] == 0x0A {
		return true
	}

	// Check for JPEG XL container (starts with "JXL ")
	return len(buffer) > 12 && string(buffer[:4]) == "JXL "
}

func OutputFormatFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".webp", ".riff":
		return "webp"
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".png":
		return "png"
	case ".gif":
		return "gif"
	case ".bmp":
		return "bmp"
	case ".tiff", ".tif":
		return "tiff"
	case ".avif", ".avifs":
		return "avif"
	case ".jxl":
		return "jxl"
	case ".ico":
		return "ico"
	}

	return "webp"
}

func IsValidOutputFormat(format string) bool {
	for _, f := range OutputFormats {
		if f == format {
			return true
		}
	}

	return false
}
