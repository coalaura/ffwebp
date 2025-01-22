package main

import (
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/coalaura/arguments"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"golang.org/x/image/tiff"
)

type Options struct {
	Help   bool
	Input  string
	Output string

	Silent      bool
	NumColors   int
	Effort      int
	Format      string
	Lossless    bool
	Method      int
	Ratio       int
	Quality     int
	Exact       bool
	Compression int
	Level       int
	Speed       int
}

var opts = Options{
	Help:   false,
	Input:  "",
	Output: "",

	Silent:      false,
	NumColors:   256,
	Effort:      10,
	Format:      "",
	Lossless:    false,
	Method:      6,
	Ratio:       0,
	Quality:     90,
	Exact:       false,
	Compression: 2,
	Level:       2,
	Speed:       0,
}

func parse() {
	// General options
	arguments.Register("help", 'h', &opts.Help).WithHelp("Show this help message")
	arguments.Register("silent", 's', &opts.Silent).WithHelp("Do not print any output")
	arguments.Register("format", 'f', &opts.Format).WithHelp("Output format (avif, bmp, gif, jpeg, jxl, png, tiff, webp, ico)")

	// Common image options
	arguments.Register("quality", 'q', &opts.Quality).WithHelp("[avif|jpeg|jxl|webp] Quality level (1-100)")

	// AVIF
	arguments.Register("ratio", 'r', &opts.Ratio).WithHelp("[avif] YCbCr subsample-ratio (0=444, 1=422, 2=420, 3=440, 4=411, 5=410)")
	arguments.Register("speed", 'p', &opts.Speed).WithHelp("[avif] Encoder speed level (0=fast, 10=slower-better)")

	// GIF
	arguments.Register("colors", 'c', &opts.NumColors).WithHelp("[gif] Number of colors to use (1-256)")

	// JXL
	arguments.Register("effort", 'e', &opts.Effort).WithHelp("[jxl] Encoder effort level (0=fast, 10=slower-better)")

	// PNG
	arguments.Register("level", 'g', &opts.Level).WithHelp("[png] Compression level (0=no-compression, 1=best-speed, 2=best-compression)")

	// TIFF
	arguments.Register("compression", 't', &opts.Compression).WithHelp("[tiff] Compression type (0=uncompressed, 1=deflate, 2=lzw, 3=ccittgroup3, 4=ccittgroup4)")

	// WebP
	arguments.Register("exact", 'x', &opts.Exact).WithHelp("[webp] Preserve RGB values in transparent area")
	arguments.Register("lossless", 'l', &opts.Lossless).WithHelp("[webp] Use lossless compression")
	arguments.Register("method", 'm', &opts.Method).WithHelp("[webp] Encoder method (0=fast, 6=slower-better)")

	arguments.Parse()

	help()

	if len(arguments.Args) < 1 {
		fatalf("Missing input file")
	}

	opts.Input = arguments.Args[0]

	if len(arguments.Args) > 1 {
		opts.Output = arguments.Args[1]
	}

	if opts.Format != "" && !IsValidOutputFormat(opts.Format) {
		fatalf("Invalid output format: %s", opts.Format)
	}

	// Resolve format from output file
	if opts.Format == "" && opts.Output != "" {
		opts.Format = GetFormatFromPath(opts.Output)
	}

	// Otherwise resolve format from input file
	if opts.Format == "" {
		opts.Format = GetFormatFromPath(opts.Input)
	}

	// Or default to webp
	if opts.Format == "" {
		opts.Format = "webp"
	} else if opts.Format == "jpg" {
		opts.Format = "jpeg"
	}

	// NumColors must be between 1 and 256
	if opts.NumColors < 1 || opts.NumColors > 256 {
		opts.NumColors = 256
	}

	// Effort must be between 0 and 10
	if opts.Effort < 0 || opts.Effort > 10 {
		opts.Effort = 10
	}

	// Method must be between 0 and 6
	if opts.Method < 0 || opts.Method > 6 {
		opts.Method = 6
	}

	// Quality must be between 1 and 100
	if opts.Quality < 1 || opts.Quality > 100 {
		opts.Quality = 90
	}

	// Ratio must be between 0 and 5
	if opts.Ratio < 0 || opts.Ratio > 5 {
		opts.Ratio = 0
	}

	// Compression must be between 0 and 4
	if opts.Compression < 0 || opts.Compression > 4 {
		opts.Compression = 2
	}

	// Level must be between 0 and 2
	if opts.Level < 0 || opts.Level > 2 {
		opts.Level = 2
	}
}

func GetWebPOptions() webp.Options {
	return webp.Options{
		Lossless: opts.Lossless,
		Quality:  opts.Quality,
		Method:   opts.Method,
		Exact:    opts.Exact,
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
		Quality: opts.Quality,
	}
}

func LogJpegOptions(options *jpeg.Options) {
	info("Using output options:")
	info(" - quality: %v", options.Quality)
}

func GetPNGOptions() *png.Encoder {
	return &png.Encoder{
		CompressionLevel: GetPNGCompressionLevel(),
	}
}

func LogPNGOptions(encoder *png.Encoder) {
	info("Using output options:")
	info(" - level: %s", PNGCompressionLevelToString(encoder.CompressionLevel))
}

func GetGifOptions() *gif.Options {
	return &gif.Options{
		NumColors: opts.NumColors,
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
		Quality:           opts.Quality,
		QualityAlpha:      opts.Quality,
		Speed:             opts.Speed,
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
		Quality: opts.Quality,
		Effort:  opts.Effort,
	}
}

func LogJxlOptions(options jpegxl.Options) {
	info("Using output options:")
	info(" - quality: %v", options.Quality)
	info(" - effort: %v", options.Effort)
}

func GetTiffCompressionType() tiff.CompressionType {
	switch opts.Compression {
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
	switch opts.Ratio {
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

func GetPNGCompressionLevel() png.CompressionLevel {
	switch opts.Level {
	case 0:
		return png.NoCompression
	case 1:
		return png.BestSpeed
	case 2:
		return png.BestCompression
	}

	return png.BestCompression
}

func PNGCompressionLevelToString(level png.CompressionLevel) string {
	switch level {
	case png.NoCompression:
		return "no-compression"
	case png.BestSpeed:
		return "best-speed"
	case png.BestCompression:
		return "best-compression"
	default:
		return "unknown"
	}
}
