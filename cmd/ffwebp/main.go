package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/coalaura/ffwebp/internal/builtins"
	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/nfnt/resize"
	"github.com/urfave/cli/v3"
)

var Version = "dev"

func main() {
	flags := codec.Flags([]cli.Flag{
		&cli.StringFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Usage:   "input file (\"-\" = stdin)",
			Value:   "-",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "output file (\"-\" = stdout)",
			Value:   "-",
		},
		&cli.StringFlag{
			Name:    "codec",
			Aliases: []string{"c"},
			Usage:   "force output codec (jpeg, png, ...)",
		},
		&cli.IntFlag{
			Name:    "quality",
			Aliases: []string{"q"},
			Usage:   "0-100 quality for lossy codecs",
			Value:   85,
		},
		&cli.BoolFlag{
			Name:    "lossless",
			Aliases: []string{"l"},
			Usage:   "force lossless mode (overrides --quality)",
		},
		&cli.UintFlag{
			Name:    "thumbnail",
			Aliases: []string{"t"},
			Usage:   "create a thumbnail no wider/taller than the specified size",
		},
		&cli.BoolFlag{
			Name:    "silent",
			Aliases: []string{"s"},
			Usage:   "hides all output",
			Action: func(_ context.Context, _ *cli.Command, silent bool) error {
				if silent {
					logx.SetSilent()
				}

				return nil
			},
		},
	})

	app := &cli.Command{
		Name:                   "ffwebp",
		Usage:                  "Convert any image format into any other image format",
		Version:                Version,
		Flags:                  flags,
		Action:                 run,
		Writer:                 os.Stderr,
		ErrWriter:              os.Stderr,
		EnableShellCompletion:  true,
		UseShortOptionHandling: true,
		Suggest:                true,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		logx.Errorf("fatal: %v\n", err)
	}
}

func run(_ context.Context, cmd *cli.Command) error {
	banner()

	var (
		input  string
		output string

		common opts.Common

		reader io.Reader    = os.Stdin
		writer *countWriter = &countWriter{w: os.Stdout}
	)

	if input = cmd.String("input"); input != "-" {
		logx.Printf("opening input file %q\n", filepath.ToSlash(input))

		file, err := os.OpenFile(input, os.O_RDONLY, 0)
		if err != nil {
			return err
		}

		defer file.Close()

		reader = file
	} else {
		logx.Printf("reading input from <stdin>\n")
	}

	sniffed, reader, err := codec.Sniff(reader, input)
	if err != nil {
		return err
	}

	logx.Printf("sniffed codec: %s (%q)\n", sniffed.Codec, sniffed)

	if output = cmd.String("output"); output != "-" {
		logx.Printf("opening output file %q\n", filepath.ToSlash(output))

		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		defer file.Close()

		writer = &countWriter{w: file}
	} else {
		logx.Printf("writing output to <stdout>\n")
	}

	common.Quality = cmd.Int("quality")
	common.Lossless = cmd.Bool("lossless")

	common.FillDefaults()

	oCodec, oExt, err := codec.Detect(output, cmd.String("codec"))
	if err != nil {
		return err
	}

	common.OutputExtension = oExt

	logx.Printf("output codec: %s (forced=%v)\n", oCodec, cmd.IsSet("codec"))

	t0 := time.Now()

	img, err := sniffed.Codec.Decode(reader)
	if err != nil {
		return err
	}

	logx.Printf("decoded image: %dx%d %s in %s\n", img.Bounds().Dx(), img.Bounds().Dy(), colorModel(img), time.Since(t0).Truncate(time.Millisecond))

	if thumbnail := cmd.Uint("thumbnail"); thumbnail > 0 {
		t2 := time.Now()

		img = resize.Thumbnail(thumbnail, thumbnail, img, resize.Lanczos3)

		logx.Printf("resized image: %dx%d in %s\n", img.Bounds().Dx(), img.Bounds().Dy(), time.Since(t2).Truncate(time.Millisecond))
	}

	t1 := time.Now()

	err = oCodec.Encode(writer, img, common)
	if err != nil {
		return err
	}

	logx.Printf("encoded %d KiB in %s\n", (writer.n+1023)/1024, time.Since(t1).Truncate(time.Millisecond))

	return nil
}
