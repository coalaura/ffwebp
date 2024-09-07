package main

import (
	"image"
	"image/gif"
	"image/jpeg"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"golang.org/x/image/tiff"
)

func GetWebPOptions() webp.Options {
	return webp.Options{
		Lossless: arguments.GetBool("l", "lossless", false),
		Quality:  int(arguments.GetUint64("q", "quality", 100, 0, 100)),
		Method:   int(arguments.GetUint64("m", "method", 4, 0, 6)),
		Exact:    arguments.GetBool("x", "exact", false),
	}
}

func LogWebPOptions(options webp.Options) {
	info("Using output options:")
	info(" - lossless: %v", options.Lossless)
	info(" - quality:  %v", options.Quality)
	info(" - method:   %v", options.Method)
	info(" - exact:    %v", options.Exact)
}

func GetJpegOptions() *jpeg.Options {
	return &jpeg.Options{
		Quality: int(arguments.GetUint64("q", "quality", 100, 0, 100)),
	}
}

func LogJpegOptions(options *jpeg.Options) {
	info("Using output options:")
	info(" - quality: %v", options.Quality)
}

func GetGifOptions() *gif.Options {
	return &gif.Options{
		NumColors: int(arguments.GetUint64("c", "colors", 256, 0, 256)),
	}
}

func LogGifOptions(options *gif.Options) {
	info("Using output options:")
	info(" - colors: %v", options.NumColors)
}

func GetTiffOptions() *tiff.Options {
	return &tiff.Options{
		Compression: GetTiffCompressionType(),
	}
}

func LogTiffOptions(options *tiff.Options) {
	info("Using output options:")
	info(" - compression: %s", TiffCompressionTypeToString(options.Compression))
}

func GetAvifOptions() avif.Options {
	return avif.Options{
		Quality:           int(arguments.GetUint64("q", "quality", 100, 0, 100)),
		QualityAlpha:      int(arguments.GetUint64("qa", "quality-alpha", 100, 0, 100)),
		Speed:             int(arguments.GetUint64("s", "speed", 6, 0, 10)),
		ChromaSubsampling: GetAvifYCbCrSubsampleRatio(),
	}
}

func LogAvifOptions(options avif.Options) {
	info("Using output options:")
	info(" - quality: %v", options.Quality)
	info(" - quality-alpha: %v", options.QualityAlpha)
	info(" - speed: %v", options.Speed)
	info(" - chroma subsampling: %s", options.ChromaSubsampling.String())
}

func GetJxlOptions() jpegxl.Options {
	return jpegxl.Options{
		Quality: int(arguments.GetUint64("q", "quality", 100, 0, 100)),
		Effort:  int(arguments.GetUint64("e", "effort", 7, 0, 10)),
	}
}

func LogJxlOptions(options jpegxl.Options) {
	info("Using output options:")
	info(" - quality: %v", options.Quality)
	info(" - effort: %v", options.Effort)
}

func GetTiffCompressionType() tiff.CompressionType {
	compression := arguments.GetUint64("z", "compression", 1, 0, 4)

	switch compression {
	case 0:
		return tiff.Uncompressed
	case 1:
		return tiff.Deflate
	case 2:
		return tiff.LZW
	case 3:
		return tiff.CCITTGroup3
	case 4:
		return tiff.CCITTGroup4
	}

	return tiff.Deflate
}

func TiffCompressionTypeToString(compression tiff.CompressionType) string {
	switch compression {
	case tiff.Uncompressed:
		return "uncompressed"
	case tiff.Deflate:
		return "deflate"
	case tiff.LZW:
		return "lzw"
	case tiff.CCITTGroup3:
		return "ccittgroup3"
	case tiff.CCITTGroup4:
		return "ccittgroup4"
	default:
		return "unknown"
	}
}

func GetAvifYCbCrSubsampleRatio() image.YCbCrSubsampleRatio {
	sampleRatio := arguments.GetUint64("r", "sample-ratio", 0, 0, 5)

	switch sampleRatio {
	case 0:
		return image.YCbCrSubsampleRatio444
	case 1:
		return image.YCbCrSubsampleRatio422
	case 2:
		return image.YCbCrSubsampleRatio420
	case 3:
		return image.YCbCrSubsampleRatio440
	case 4:
		return image.YCbCrSubsampleRatio411
	case 5:
		return image.YCbCrSubsampleRatio410
	}

	return image.YCbCrSubsampleRatio444
}
