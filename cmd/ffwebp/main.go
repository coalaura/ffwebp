package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/coalaura/ffwebp/internal/builtins"
	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/nfnt/resize"
	"github.com/urfave/cli/v3"
)

var Version = "dev"

func main() {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Usage:   "input file or pattern (\"-\" = stdin, supports globs and %d sequences)",
			Value:   "-",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "output file, directory, or pattern (\"-\" = stdout)",
			Value:   "-",
		},
		&cli.IntFlag{
			Name:  "start-number",
			Usage: "starting number for %%d output patterns",
			Value: 1,
		},
		&cli.StringFlag{
			Name:    "codec",
			Aliases: []string{"c"},
			Usage:   "force output codec (jpeg, png, ...)",
		},
		&cli.BoolFlag{
			Name:    "sniff",
			Aliases: []string{"f"},
			Usage:   "force sniffing of input codec (ignore extension)",
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
	}

	flags = codec.Flags(flags)
	flags = effects.Flags(flags)

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
	)

	common.Quality = cmd.Int("quality")
	common.Lossless = cmd.Bool("lossless")

	common.FillDefaults()

	input = cmd.String("input")
	output = cmd.String("output")

	var inputs []string

	if input == "-" {
		inputs = []string{"-"}
	} else if hasSeq(input) {
		files, err := expandSeq(input)
		if err != nil {
			return err
		}

		if len(files) == 0 {
			return fmt.Errorf("no inputs match sequence: %s", filepath.ToSlash(input))
		}

		inputs = files
	} else if isGlob(input) {
		files, err := expandGlob(input)
		if err != nil {
			return err
		}

		if len(files) == 0 {
			return fmt.Errorf("no inputs match glob: %s", filepath.ToSlash(input))
		}

		inputs = files
	} else {
		inputs = []string{input}
	}

	startNum := cmd.Int("start-number")

	var outputs []string

	if len(inputs) == 1 {
		outputs = []string{output}
	} else {
		switch {
		case output == "-":
			return fmt.Errorf("multiple inputs require an output pattern or directory, not '-' ")
		case hasSeq(output):
			outs := make([]string, len(inputs))

			for i := range inputs {
				outs[i] = formatSeq(output, i, startNum)
			}

			outputs = outs
		default:
			outDir := output
			if outDir == "" {
				outDir = "."
			}

			if fi, err := os.Stat(outDir); err == nil {
				if !fi.IsDir() {
					return fmt.Errorf("output must be a directory or pattern when multiple inputs: %s", filepath.ToSlash(output))
				}
			} else {
				if err := os.MkdirAll(outDir, 0755); err != nil {
					return fmt.Errorf("create output directory: %w", err)
				}
			}

			outs := make([]string, len(inputs))

			for i := range inputs {
				outs[i] = output
			}

			outputs = outs
		}
	}

	for i := range inputs {
		in := inputs[i]
		out := outputs[i]

		if err := processOne(in, out, cmd, &common); err != nil {
			return err
		}
	}

	return nil
}

func processOne(input, output string, cmd *cli.Command, common *opts.Common) error {
	var (
		reader io.Reader    = os.Stdin
		writer *countWriter = &countWriter{w: os.Stdout}
	)

	if input != "-" {
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

	sniffed, reader2, err := codec.Sniff(reader, input, cmd.Bool("sniff"))
	if err != nil {
		return err
	}

	reader = reader2

	logx.Printf("sniffed codec: %s (%q)\n", sniffed.Codec, sniffed)

	var mappedFromDir bool

	if output != "-" {
		if fi, err := os.Stat(output); err == nil && fi.IsDir() {
			name := filepath.Base(input)
			output = filepath.Join(output, name)

			mappedFromDir = true
		} else if strings.HasSuffix(output, string(os.PathSeparator)) {
			if err := os.MkdirAll(output, 0755); err != nil {
				return err
			}

			name := filepath.Base(input)
			output = filepath.Join(output, name)

			mappedFromDir = true
		}
	}

	oCodec, oExt, err := codec.Detect(output, cmd.String("codec"))
	if err != nil {
		return err
	}

	common.OutputExtension = oExt

	logx.Printf("output codec: %s (forced=%v)\n", oCodec, cmd.IsSet("codec"))

	if output != "-" {
		curExt := strings.TrimPrefix(filepath.Ext(output), ".")

		if mappedFromDir || curExt == "" || curExt != oExt {
			base := strings.TrimSuffix(output, filepath.Ext(output))
			output = base + "." + oExt
		}
	}

	if output != "-" {
		logx.Printf("opening output file %q\n", filepath.ToSlash(output))

		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			return err
		}

		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		defer file.Close()

		writer = &countWriter{w: file}
	} else {
		logx.Printf("writing output to <stdout>\n")
	}

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

	var n int

	img, n, err = effects.ApplyAll(img)
	if err != nil {
		return err
	} else if n > 0 {
		logx.Printf("applied %d effect(s) in %s\n", n, time.Since(t1).Truncate(time.Millisecond))
	}

	t2 := time.Now()

	if err := oCodec.Encode(writer, img, *common); err != nil {
		return err
	}

	logx.Printf("encoded %d KiB in %s\n", (writer.n+1023)/1024, time.Since(t2).Truncate(time.Millisecond))

	return nil
}
