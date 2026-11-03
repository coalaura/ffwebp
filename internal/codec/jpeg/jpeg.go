package jpeg

import (
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	_ "github.com/coalaura/ffwebp/internal/codec/jpeg/embedded"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/jpeg/classic"
	"github.com/coalaura/jpeg/detect"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var command *cli.Command

var subsampling = map[string]classic.Subsampling{
	"auto": classic.Subsample420,
	"420":  classic.Subsample420,
	"444":  classic.Subsample444,
	"422":  classic.Subsample422,
	"440":  classic.Subsample440,
	"411":  classic.Subsample411,
	"441":  classic.Subsample441,
}

var dctMethods = map[string]classic.DCTMethod{
	"accurate": classic.Accurate,
	"fast":     classic.Fast,
	"float":    classic.Float,
}

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "jpeg"
}

func (impl) Extensions() []string {
	return []string{"jpg", "jpeg", "jpe"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Configure(cmd *cli.Command) error {
	command = cmd

	if cmd.Bool("jpeg.adaptive-quantization") {
		return fmt.Errorf("--jpeg.adaptive-quantization has no equivalent in the classic backend; remove it and use --quality, --jpeg.optimize or --jpeg.progressive")
	}

	return nil
}

func (impl) InputLimit() int64 {
	return command.Int64("jpeg.max-input-bytes")
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	flags = append(flags,
		&cli.BoolFlag{Name: "jpeg.progressive", Usage: "JPEG: progressive DCT encoding"},
		&cli.BoolFlag{Name: "jpeg.optimize", Usage: "JPEG: optimize Huffman coding"},
		&cli.BoolFlag{Name: "jpeg.adaptive-quantization", Usage: "JPEG: obsolete; enabling this reports migration instructions"},
		jpegutil.Enum("jpeg.subsampling", "auto", "auto 444 440 422 420 411 441 gray", "JPEG: chroma subsampling; gray converts RGB to grayscale"),
		&cli.IntFlag{Name: "jpeg.precision", Usage: "JPEG: bits/sample; 0=8 for DCT, source precision for predictive; DCT supports 8/12, predictive 2..16"},
		&cli.BoolFlag{Name: "jpeg.lossless", Usage: "JPEG: predictive coding (also selected by --lossless); not DCT quality 100"},
		&cli.IntFlag{Name: "jpeg.predictor", Value: 1, Usage: "JPEG: predictive lossless predictor, 1..7"},
		&cli.IntFlag{Name: "jpeg.point-transform", Usage: "JPEG: discarded low bits in predictive mode; --lossless requires zero"},
		&cli.BoolFlag{Name: "jpeg.arithmetic", Usage: "JPEG: arithmetic entropy coding (DCT only)"},
		&cli.IntFlag{Name: "jpeg.restart-interval", Usage: "JPEG: restart interval in MCUs, 0..65535; conflicts with restart-rows"},
		&cli.IntFlag{Name: "jpeg.restart-rows", Usage: "JPEG: restart interval in MCU rows, 0..65535"},
		jpegutil.Enum("jpeg.dct", "accurate", "accurate fast float", "JPEG: encoder and decoder DCT method"),
		&cli.IntFlag{Name: "jpeg.scale-numerator", Value: 1, Usage: "JPEG input: IDCT scale numerator, 1..16"},
		&cli.IntFlag{Name: "jpeg.scale-denominator", Value: 1, Usage: "JPEG input: IDCT scale denominator, 1..16; rounded to a supported scale"},
		&cli.BoolFlag{Name: "jpeg.fancy-upsampling", Value: true, Usage: "JPEG input: fancy chroma upsampling"},
		&cli.BoolFlag{Name: "jpeg.block-smoothing", Value: true, Usage: "JPEG input: progressive block smoothing"},
		&cli.BoolFlag{Name: "jpeg.allow-warnings", Usage: "JPEG input: allow native warnings including truncation"},
		&cli.IntFlag{Name: "jpeg.max-scans", Usage: "JPEG input: scan limit, 0=1000, maximum 10000"},
	)

	return jpegutil.Limits(flags, "jpeg")
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	return jpegutil.Sniff(reader, detect.JPEG)
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	settings := classic.DecodeOptions{
		ScaleNumerator:   command.Int("jpeg.scale-numerator"),
		ScaleDenominator: command.Int("jpeg.scale-denominator"),
		DCT:              dctMethods[command.String("jpeg.dct")],
		FancyUpsampling:  jpegutil.OptionalBool(command, "jpeg.fancy-upsampling"),
		BlockSmoothing:   jpegutil.OptionalBool(command, "jpeg.block-smoothing"),
		AllowWarnings:    command.Bool("jpeg.allow-warnings"),
		ReadICC:          true,
		MaxScans:         command.Int("jpeg.max-scans"),
		MaxPixels:        command.Uint64("jpeg.max-pixels"),
		MaxInputBytes:    command.Int64("jpeg.max-input-bytes"),
	}

	samples, err := classic.DecodeSamples(reader, &settings)
	if err != nil {
		return nil, err
	}

	profile := samples.ICC

	if len(profile) > 0 && samples.ColorSpace == classic.CMYK {
		return nil, fmt.Errorf("jpeg: CMYK ICC profiles require color conversion; convert the source profile and pixels to RGB together before processing")
	}

	img, err := displayImage(samples)
	if err != nil {
		return nil, err
	}

	return &codec.ProfiledImage{Image: img, ICC: profile, Channels: codec.ImageChannels(img)}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	settings := classic.Options{
		Quality:         new(common.Quality),
		Progressive:     command.Bool("jpeg.progressive"),
		OptimizeHuffman: command.Bool("jpeg.optimize"),
		Subsampling:     subsampling[command.String("jpeg.subsampling")],
		Arithmetic:      command.Bool("jpeg.arithmetic"),
		BitsPerSample:   command.Int("jpeg.precision"),
		RestartInterval: command.Int("jpeg.restart-interval"),
		RestartRows:     command.Int("jpeg.restart-rows"),
		DCT:             dctMethods[command.String("jpeg.dct")],
	}

	predictive := common.Lossless || command.Bool("jpeg.lossless")
	if predictive {
		if settings.Progressive || settings.Arithmetic {
			return fmt.Errorf("jpeg: progressive and arithmetic coding require DCT mode and conflict with predictive lossless")
		}

		settings.Lossless = &classic.Lossless{
			Predictor:      command.Int("jpeg.predictor"),
			PointTransform: command.Int("jpeg.point-transform"),
		}

		if settings.BitsPerSample == 0 {
			settings.BitsPerSample = jpegutil.Precision(img)
		}
	} else if command.IsSet("jpeg.predictor") || command.IsSet("jpeg.point-transform") {
		return fmt.Errorf("jpeg: predictor and point-transform require --jpeg.lossless or --lossless")
	}

	if common.Lossless && (settings.Lossless.PointTransform != 0 || settings.BitsPerSample < jpegutil.Precision(img)) {
		return fmt.Errorf("jpeg: --lossless requires zero point transform and source precision")
	}

	if command.String("jpeg.subsampling") == "gray" {
		// Standard sRGB needs no profile transform for the existing implicit gray conversion.
		if codec.StandardSRGB(common.ICC) {
			common.ICC = nil
		}

		var err error

		img, err = grayscale(img)
		if err != nil {
			return err
		}
	}

	err := codec.ValidateICC(codec.ImageChannels(img), common.ICC)
	if err != nil {
		if command.String("jpeg.subsampling") == "gray" {
			return fmt.Errorf("jpeg: %w; remove --jpeg.subsampling gray to retain an RGB profile", err)
		}

		return err
	}

	settings.Markers, err = profileMarkers(common.ICC)
	if err != nil {
		return err
	}

	return classic.Encode(writer, img, &settings)
}

func grayscale(img image.Image) (image.Image, error) {
	bounds := img.Bounds()

	if jpegutil.Precision(img) == 16 {
		gray := image.NewGray16(bounds)

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, alpha := img.At(x, y).RGBA()
				if alpha != 65535 {
					return nil, fmt.Errorf("jpeg: grayscale output cannot retain alpha; composite onto an opaque background first")
				}

				gray.SetGray16(x, y, color.Gray16Model.Convert(img.At(x, y)).(color.Gray16))
			}
		}

		return gray, nil
	}

	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha != 65535 {
				return nil, fmt.Errorf("jpeg: grayscale output cannot retain alpha; composite onto an opaque background first")
			}

			gray.SetGray(x, y, color.GrayModel.Convert(img.At(x, y)).(color.Gray))
		}
	}

	return gray, nil
}
