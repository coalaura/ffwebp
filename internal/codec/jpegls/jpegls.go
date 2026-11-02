package jpegls

import (
	"fmt"
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/jpeg/detect"
	ls "github.com/coalaura/jpeg/jpegls"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var command *cli.Command

var (
	interleaves = map[string]ls.Interleave{"planar": ls.Planar, "line": ls.Line, "sample": ls.Sample}
	transforms  = map[string]ls.Transform{"none": ls.NoTransform, "hp1": ls.HP1, "hp2": ls.HP2, "hp3": ls.HP3}
)

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "jpegls"
}

func (impl) Extensions() []string {
	return []string{"jls"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Configure(cmd *cli.Command) error {
	command = cmd

	return nil
}

func (impl) InputLimit() int64 {
	return command.Int64("jpegls.max-input-bytes")
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	flags = append(flags,
		&cli.IntFlag{Name: "jpegls.precision", Usage: "JPEG-LS: bits/sample, 2..16; 0 preserves source precision"},
		&cli.IntFlag{Name: "jpegls.near", Usage: "JPEG-LS: maximum error in native sample units, 0=exact lossless (default); 0..min(255, maximum/2); independent of --quality"},
		jpegutil.Enum("jpegls.interleave", "auto", "auto planar line sample", "JPEG-LS: auto uses planar grayscale or sample-interleaved color"),
		jpegutil.Enum("jpegls.transform", "none", "none hp1 hp2 hp3", "JPEG-LS: HP transform requires three 8/16-bit components and NEAR=0"),
		&cli.StringFlag{Name: "jpegls.preset", Usage: "JPEG-LS: MAXVAL,T1,T2,T3,RESET coding parameters; omitted uses ISO defaults"},
		&cli.BoolFlag{Name: "jpegls.rgba", Usage: "JPEG-LS: explicitly encode/interpret four components as straight RGBA by caller agreement; not stored as metadata"},
	)

	return jpegutil.Limits(flags, "jpegls")
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	return jpegutil.Sniff(reader, detect.JPEGLS)
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	settings := ls.DecodeOptions{
		RGBA:          command.Bool("jpegls.rgba"),
		MaxPixels:     command.Uint64("jpegls.max-pixels"),
		MaxInputBytes: command.Int64("jpegls.max-input-bytes"),
	}

	img, err := ls.DecodeWithOptions(reader, &settings)
	if err != nil {
		return nil, fmt.Errorf("%w; four-component input needs --jpegls.rgba when its channels are RGBA", err)
	}

	return img, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	settings := ls.Options{
		BitsPerSample: command.Int("jpegls.precision"),
		NEAR:          command.Int("jpegls.near"),
		Transform:     transforms[command.String("jpegls.transform")],
		RGBA:          command.Bool("jpegls.rgba"),
	}

	if common.Lossless && settings.NEAR != 0 {
		return fmt.Errorf("jpegls: --lossless requires NEAR=0; remove --jpegls.near or set it to zero")
	}

	if settings.Transform != ls.NoTransform && settings.NEAR != 0 {
		return fmt.Errorf("jpegls: HP transform requires NEAR=0; remove the transform or set --jpegls.near to zero")
	}

	if common.Lossless && settings.BitsPerSample != 0 && settings.BitsPerSample < jpegutil.Precision(img) {
		return fmt.Errorf("jpegls: --lossless requires source precision")
	}

	if command.String("jpegls.interleave") != "auto" {
		settings.Interleave = new(interleaves[command.String("jpegls.interleave")])
	}

	if command.IsSet("jpegls.preset") {
		values, err := jpegutil.Integers(command.String("jpegls.preset"), 5)
		if err != nil {
			return err
		}

		settings.Preset = &ls.Preset{
			Maximum:    values[0],
			Threshold1: values[1],
			Threshold2: values[2],
			Threshold3: values[3],
			Reset:      values[4],
		}
	}

	err := ls.Encode(writer, img, &settings)
	if err != nil {
		return fmt.Errorf("%w; alpha input requires --jpegls.rgba or an opaque composite", err)
	}

	return nil
}
