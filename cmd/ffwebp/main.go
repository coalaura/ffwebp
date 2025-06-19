package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

var Version = "dev"

func main() {
	log.SetOutput(os.Stderr)

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
		&cli.StringFlag{
			Name:    "resize",
			Aliases: []string{"r"},
			Usage:   "WxH, Wx or xH (keep aspect)",
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
		log.Fatal(err)
	}
}

func run(_ context.Context, cmd *cli.Command) error {
	var (
		input  string
		output string

		common opts.Common

		reader io.Reader = os.Stdin
		writer io.Writer = os.Stdout
	)

	if input = cmd.String("input"); input != "-" {
		file, err := os.OpenFile(input, os.O_RDONLY, 0)
		if err != nil {
			return err
		}

		defer file.Close()

		reader = file
	}

	if output = cmd.String("output"); output != "-" {
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		defer file.Close()

		writer = file
	}

	common.Quality = cmd.Int("quality")
	common.Lossless = cmd.Bool("lossless")

	oCodec, err := codec.Detect(output, cmd.String("codec"))
	if err != nil {
		return err
	}

	iCodec, reader, err := codec.Sniff(reader)
	if err != nil {
		return err
	}

	img, err := iCodec.Decode(reader)
	if err != nil {
		return err
	}

	resized, err := resize(img, cmd)
	if err != nil {
		return err
	}

	return oCodec.Encode(writer, resized, common)
}
