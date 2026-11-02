package jpeg2000

import (
	"fmt"
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/jpeg/detect"
	jp2 "github.com/coalaura/jpeg/jpeg2000"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var command *cli.Command

var (
	progressions = map[string]jp2.Progression{"lrcp": jp2.LRCP, "rlcp": jp2.RLCP, "rpcl": jp2.RPCL, "pcrl": jp2.PCRL, "cprl": jp2.CPRL}
	colors       = map[string]jp2.ColorSpace{"unspecified": jp2.Unspecified, "rgb": jp2.SRGB, "gray": jp2.Gray, "sycc": jp2.SYCC, "eycc": jp2.EYCC, "cmyk": jp2.CMYK}
)

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "jpeg2000"
}

func (impl) Extensions() []string {
	return []string{"jp2", "j2k", "j2c"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Configure(cmd *cli.Command) error {
	command = cmd

	return nil
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	flags = append(flags,
		&cli.IntFlag{Name: "jpeg2000.precision", Usage: "JPEG 2000: bits/sample, 2..16; 0 preserves source precision"},
		&cli.BoolFlag{Name: "jpeg2000.irreversible", Usage: "JPEG 2000: lossy 9/7 wavelets; default is reversible exact lossless, independent of --quality"},
		&cli.StringFlag{Name: "jpeg2000.ratios", Usage: "JPEG 2000: comma-separated decreasing compression ratios >=1; final 1 is lossless with reversible wavelets; conflicts with psnr"},
		&cli.StringFlag{Name: "jpeg2000.psnr", Usage: "JPEG 2000: comma-separated increasing PSNR layers in dB; final 0 is lossless with reversible wavelets; conflicts with ratios"},
		jpegutil.Enum("jpeg2000.progression", "lrcp", "lrcp rlcp rpcl pcrl cprl", "JPEG 2000: progression order"),
		&cli.IntFlag{Name: "jpeg2000.resolutions", Usage: "JPEG 2000: resolution count; 0 chooses up to 6 according to tile size"},
		&cli.IntFlag{Name: "jpeg2000.tile-width", Usage: "JPEG 2000: tile width; specify both dimensions"},
		&cli.IntFlag{Name: "jpeg2000.tile-height", Usage: "JPEG 2000: tile height"},
		&cli.IntFlag{Name: "jpeg2000.code-block-width", Usage: "JPEG 2000: code block width; 0=64, powers of two 4..1024, area <=4096"},
		&cli.IntFlag{Name: "jpeg2000.code-block-height", Usage: "JPEG 2000: code block height; 0=64"},
		&cli.BoolFlag{Name: "jpeg2000.component-transform", Value: true, Usage: "JPEG 2000: RGB component transform; omitted selects by color space; explicit false disables it"},
		&cli.BoolFlag{Name: "jpeg2000.sop", Usage: "JPEG 2000: start-of-packet markers"},
		&cli.BoolFlag{Name: "jpeg2000.eph", Usage: "JPEG 2000: end-of-packet-header markers"},
		&cli.IntFlag{Name: "jpeg2000.threads", Usage: "JPEG 2000: native encode/decode workers; 0=1, maximum 16"},
		&cli.IntFlag{Name: "jpeg2000.reduce", Usage: "JPEG 2000 input: discard highest resolution levels, 0..30"},
		&cli.IntFlag{Name: "jpeg2000.layers", Usage: "JPEG 2000 input: quality layer count; 0=all"},
		&cli.StringFlag{Name: "jpeg2000.region", Usage: "JPEG 2000 input: X0,Y0,X1,Y1 region in full-resolution coordinates"},
		jpegutil.Enum("jpeg2000.raw-color", "unspecified", "unspecified rgb gray sycc eycc cmyk", "Raw J2K has no color metadata: supply --jpeg2000.raw-color rgb or gray; YCC/CMYK require external display conversion"),
		&cli.IntFlag{Name: "jpeg2000.raw-alpha", Usage: "Raw J2K input: explicitly identify a straight-alpha component by zero-based index; omission means no alpha"},
	)

	return jpegutil.Limits(flags, "jpeg2000")
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	return jpegutil.Sniff(reader, detect.JP2, detect.J2K)
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	settings := jp2.DecodeOptions{
		Reduce:        command.Int("jpeg2000.reduce"),
		Layers:        command.Int("jpeg2000.layers"),
		Threads:       command.Int("jpeg2000.threads"),
		RawColorSpace: colors[command.String("jpeg2000.raw-color")],
		MaxPixels:     command.Uint64("jpeg2000.max-pixels"),
		MaxInputBytes: command.Int64("jpeg2000.max-input-bytes"),
	}

	if command.IsSet("jpeg2000.raw-alpha") {
		settings.RawAlphaComponent = new(command.Int("jpeg2000.raw-alpha"))
	}

	if command.IsSet("jpeg2000.region") {
		values, err := jpegutil.Integers(command.String("jpeg2000.region"), 4)
		if err != nil {
			return nil, err
		}

		if values[0] < 0 || values[1] < 0 || values[2] <= values[0] || values[3] <= values[1] {
			return nil, fmt.Errorf("jpeg2000: region requires 0 <= X0 < X1 and 0 <= Y0 < Y1")
		}

		settings.Region = new(image.Rect(values[0], values[1], values[2], values[3]))
	}

	img, err := jp2.DecodeWithOptions(reader, &settings)
	if err != nil {
		return nil, fmt.Errorf("%w; raw J2K needs --jpeg2000.raw-color rgb|gray (and --jpeg2000.raw-alpha for alpha); signed, YCC, CMYK or mismatched component grids require external display conversion", err)
	}

	return img, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	settings := jp2.Options{
		BitsPerSample:           command.Int("jpeg2000.precision"),
		Irreversible:            command.Bool("jpeg2000.irreversible"),
		Progression:             progressions[command.String("jpeg2000.progression")],
		Resolutions:             command.Int("jpeg2000.resolutions"),
		TileWidth:               command.Int("jpeg2000.tile-width"),
		TileHeight:              command.Int("jpeg2000.tile-height"),
		CodeBlockWidth:          command.Int("jpeg2000.code-block-width"),
		CodeBlockHeight:         command.Int("jpeg2000.code-block-height"),
		MultiComponentTransform: jpegutil.OptionalBool(command, "jpeg2000.component-transform"),
		SOP:                     command.Bool("jpeg2000.sop"),
		EPH:                     command.Bool("jpeg2000.eph"),
		Threads:                 command.Int("jpeg2000.threads"),
	}

	if common.OutputExtension == "j2k" || common.OutputExtension == "j2c" {
		settings.Format = jp2.J2K
	}

	if command.IsSet("jpeg2000.ratios") && command.IsSet("jpeg2000.psnr") {
		return fmt.Errorf("jpeg2000: ratios and PSNR targets are mutually exclusive")
	}

	var err error

	settings.Ratios, err = jpegutil.Floats(command.String("jpeg2000.ratios"))
	if err != nil {
		return err
	}

	settings.PSNR, err = jpegutil.Floats(command.String("jpeg2000.psnr"))
	if err != nil {
		return err
	}

	lossyRatio := len(settings.Ratios) > 0 && settings.Ratios[len(settings.Ratios)-1] != 1
	lossyPSNR := len(settings.PSNR) > 0 && settings.PSNR[len(settings.PSNR)-1] != 0
	precisionLoss := settings.BitsPerSample != 0 && settings.BitsPerSample < jpegutil.Precision(img)

	if common.Lossless && (settings.Irreversible || lossyRatio || lossyPSNR || precisionLoss) {
		return fmt.Errorf("jpeg2000: --lossless requires reversible wavelets, a lossless final layer and source precision")
	}

	return jp2.Encode(writer, img, &settings)
}
