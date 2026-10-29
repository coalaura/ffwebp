package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "github.com/coalaura/ffwebp/internal/builtins"
	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/help"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/ffwebp/internal/play"
	"github.com/coalaura/ffwebp/internal/resize"
	"github.com/urfave/cli/v3"
)

type job struct {
	i int
}

var Version = "dev"

func main() {
	flags := []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "input",
			Aliases: []string{"i"},
			Usage:   "input file or pattern; repeat to select ordered inputs (\"-\" = stdin, supports globs and %d sequences)",
			Value:   []string{"-"},
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "output file, directory or pattern (can also be passed as a positional argument; \"-\" = stdout, supports %d and templates)",
			Value:   "-",
		},
		&cli.BoolFlag{
			Name:    "play",
			Aliases: []string{"p"},
			Usage:   "play/display the input image in a GUI window instead of writing to a file (like ffplay)",
		},
		&cli.IntFlag{
			Name:  "start-number",
			Usage: "first output number for %d, padded sequences and {#}",
			Value: 1,
		},
		&cli.StringFlag{
			Name:    "codec",
			Aliases: []string{"c"},
			Usage:   "force output codec (jpeg, png, ...)",
		},
		&cli.StringFlag{
			Name:  "input.codec",
			Usage: "force input codec, skip sniffing (jpeg, png, ...)",
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
		&cli.StringFlag{
			Name:  "split",
			Usage: "split output into XxY tiles (example: 3x2). Order: left->right, top->bottom",
		},
		&cli.StringFlag{
			Name:  "join",
			Usage: "join inputs into COLSxROWS cells (e.g. 2x2, 3xauto, autox1); unused cells stay empty",
		},
		&cli.StringFlag{
			Name:  "join-background",
			Usage: "cell background: transparent, white, black, #RGB, #RGBA, #RRGGBB or #RRGGBBAA",
			Value: "transparent",
		},
		&cli.StringFlag{
			Name:  "join-mode",
			Usage: "image sizing within cells: original, fit, cover or stretch",
			Value: "original",
		},
		&cli.StringFlag{
			Name:  "join-align",
			Usage: "cell alignment: center, top, bottom, left, right, top-left, top-right, bottom-left or bottom-right",
			Value: "center",
		},
		&cli.StringFlag{
			Name:  "join-cell",
			Usage: "fixed cell WIDTHxHEIGHT in pixels; default uses the widest image per column and tallest per row",
		},
		&cli.IntFlag{
			Name:  "threads",
			Usage: "number of worker threads (0=auto)",
			Value: 0,
		},
		&cli.IntFlag{
			Name:  "frame",
			Usage: "extract specific frame from animation (0-based index). When set, forces static output",
			Value: -1,
		},
		&cli.DurationFlag{
			Name:  "time",
			Usage: "extract frame at specific time (e.g., '2s', '500ms', '1.5s'). When set, forces static output",
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
		&cli.BoolFlag{
			Name:  "skip-existing",
			Usage: "skip files that already exist at the output path",
		},
	}

	flags = codec.Flags(flags)
	flags = effects.Flags(flags)

	app := &cli.Command{
		Name:                      "ffwebp",
		Usage:                     "Convert any image format into any other image format",
		ArgsUsage:                 "[output]",
		Version:                   fmt.Sprintf("%s %s", Version, tags()),
		Flags:                     flags,
		Action:                    run,
		Writer:                    os.Stderr,
		ErrWriter:                 os.Stderr,
		EnableShellCompletion:     true,
		UseShortOptionHandling:    true,
		DisableSliceFlagSeparator: true,
		Suggest:                   true,
		Commands: []*cli.Command{
			help.Command(),
		},
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		logx.Errorf("fatal: %v\n", err)
	}
}

func run(_ context.Context, cmd *cli.Command) error {
	banner()

	var (
		output string

		common opts.Common
	)

	common.Quality = cmd.Int("quality")
	common.Lossless = cmd.Bool("lossless")

	common.FillDefaults()

	output = cmd.String("output")

	args := cmd.Args()
	if args.Len() > 1 {
		return errors.New("expected at most one output argument")
	}

	if args.Len() == 1 {
		if cmd.IsSet("output") {
			return errors.New("output specified both as an argument and with --output")
		}

		output = args.First()
	}

	inputs, err := expandInputs(cmd.StringSlice("input"))
	if err != nil {
		return err
	}

	err = validateJoinFlags(cmd)
	if err != nil {
		return err
	}

	if cmd.IsSet("join") {
		return processJoin(inputs, output, cmd, &common)
	}

	if cmd.Bool("play") && len(inputs) > 1 {
		return errors.New("--play only supports a single input file")
	}

	splitX, splitY, err := planSplit(cmd, len(inputs))
	if err != nil {
		return err
	}

	startNum := cmd.Int("start-number")

	var (
		outputs         []string
		outputDirectory bool
	)

	if len(inputs) == 1 {
		out := output

		if out != "-" {
			if hasTemplate(out) {
				out = formatTemplate(out, inputs[0], 0, startNum)
			} else if hasSeq(out) {
				out = formatSeq(out, 0, startNum)
			}
		}

		outputs = []string{out}
	} else {
		switch {
		case output == "-":
			return fmt.Errorf("multiple inputs require an output pattern or directory, not '-' ")
		case hasTemplate(output):
			outs := make([]string, len(inputs))

			for i := range inputs {
				outs[i] = formatTemplate(output, inputs[i], i, startNum)
			}

			outputs = outs
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

			fi, err := os.Stat(outDir)
			if err == nil {
				if !fi.IsDir() {
					return fmt.Errorf("output must be a directory or pattern when multiple inputs: %s", filepath.ToSlash(output))
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			outputDirectory = true

			outs := make([]string, len(inputs))

			for i := range inputs {
				outs[i] = outDir
			}

			outputs = outs
		}
	}

	plans := make([]outputPlan, len(inputs))

	for index := range inputs {
		plan, err := resolveOutput(inputs[index], outputs[index], outputDirectory, cmd, splitX, splitY)
		if err != nil {
			return err
		}

		plans[index] = plan
	}

	if !cmd.Bool("play") {
		err := validateOutputs(inputs, plans)
		if err != nil {
			return err
		}
	}

	if len(inputs) == 1 {
		return processOne(inputs[0], &plans[0], cmd, &common, nil)
	}

	threads := cmd.Int("threads")

	if threads <= 0 {
		threads = runtime.NumCPU()
	}

	threads = min(threads, len(inputs))

	logx.Printf("using %d threads\n", threads)

	var (
		jobs = make(chan job)
		errs = make(chan error, len(inputs))
	)

	for w := 0; w < threads; w++ {
		go func() {
			for j := range jobs {
				var logger bytes.Buffer

				in := inputs[j.i]
				out := plans[j.i].path

				err := processOne(in, &plans[j.i], cmd, &common, &logger)
				if err != nil {
					errs <- fmt.Errorf("%s -> %s: %w", filepath.ToSlash(in), filepath.ToSlash(out), err)
				} else {
					errs <- nil
				}

				logx.Print(logger.String())
			}
		}()
	}

	go func() {
		for i := range inputs {
			jobs <- job{i: i}
		}

		close(jobs)
	}()

	var first error

	for range inputs {
		err := <-errs

		if err != nil && first == nil {
			first = err
		}
	}

	return first
}

func processOne(input string, plan *outputPlan, cmd *cli.Command, common *opts.Common, logger io.Writer) error {
	output := plan.path
	doSplit := len(plan.tiles) > 0
	doPlay := cmd.Bool("play")
	oCodec := plan.codec

	localOpts := *common
	localOpts.OutputExtension = plan.extension

	if !doPlay {
		logx.Fprintf(logger, "output codec: %s (forced=%v)\n", oCodec, cmd.IsSet("codec"))

		if !doSplit {
			skipped, err := skipOutput(output, cmd.Bool("skip-existing"), logger)
			if err != nil || skipped {
				return err
			}
		}
	}

	t0 := time.Now()

	img, anim, err := decodeInput(input, cmd, logger)
	if err != nil {
		return err
	}

	var (
		animEncoder    codec.AnimatedEncoder
		hasAnimEncoder bool
	)

	if oCodec != nil {
		animEncoder, hasAnimEncoder = oCodec.(codec.AnimatedEncoder)
	}

	if anim != nil {
		if cmd.Int("frame") >= 0 || cmd.IsSet("time") {
			img, err = selectAnimationFrame(anim, cmd, logger)
			if err != nil {
				return err
			}
		} else if (!doPlay && !hasAnimEncoder) || doSplit {
			img = anim.Frames[0]

			if doSplit {
				logx.Fprintf(logger, "split enabled, using first frame of %d\n", len(anim.Frames))
			} else {
				logx.Fprintf(logger, "output codec doesn't support animation, using first frame of %d\n", len(anim.Frames))
			}
		} else if len(anim.Frames) > 1 {
			first := anim.Frames[0]

			logx.Fprintf(logger, "decoded animation: %d frames %dx%d %s in %s\n", len(anim.Frames), first.Bounds().Dx(), first.Bounds().Dy(), colorModel(first), time.Since(t0).Truncate(time.Millisecond))

			thumbnail := cmd.Uint("thumbnail")
			if thumbnail > 0 {
				t2 := time.Now()

				for i, frame := range anim.Frames {
					anim.Frames[i] = resize.Thumbnail(thumbnail, thumbnail, frame)
				}

				first = anim.Frames[0]

				logx.Fprintf(logger, "resized animation: %dx%d in %s\n", first.Bounds().Dx(), first.Bounds().Dy(), time.Since(t2).Truncate(time.Millisecond))
			}

			t1 := time.Now()

			var n int

			for i, frame := range anim.Frames {
				frame, n, err = effects.ApplyAll(frame)
				if err != nil {
					return err
				}

				anim.Frames[i] = frame
			}

			bounds := anim.Frames[0].Bounds()

			anim.Frames = codec.NormalizeAnimationFrames(anim.Frames, bounds.Dx(), bounds.Dy())

			if n > 0 {
				logx.Fprintf(logger, "applied %d effect(s) to %d frame(s) in %s\n", n, len(anim.Frames), time.Since(t1).Truncate(time.Millisecond))
			}

			if doPlay {
				logx.Fprintf(logger, "playing animation...\n")

				return play.PlayAnimation(anim)
			}

			t2 := time.Now()

			written, err := writeOutput(output, cmd.Bool("skip-existing"), logger, func(writer io.Writer) error {
				return animEncoder.EncodeAll(writer, anim, localOpts)
			})

			if err != nil {
				return err
			}

			logx.Fprintf(logger, "encoded %d KiB in %s\n", (written+1023)/1024, time.Since(t2).Truncate(time.Millisecond))

			return nil
		}

		if img == nil && len(anim.Frames) == 1 {
			img = anim.Frames[0]
		}
	}

	logx.Fprintf(logger, "decoded image: %dx%d %s in %s\n", img.Bounds().Dx(), img.Bounds().Dy(), colorModel(img), time.Since(t0).Truncate(time.Millisecond))

	return processImage(img, plan, cmd, common, logger)
}

func processImage(img image.Image, plan *outputPlan, cmd *cli.Command, common *opts.Common, logger io.Writer) error {
	localOpts := *common
	localOpts.OutputExtension = plan.extension

	thumbnail := cmd.Uint("thumbnail")
	if thumbnail > 0 {
		t2 := time.Now()

		img = resize.Thumbnail(thumbnail, thumbnail, img)

		logx.Fprintf(logger, "resized image: %dx%d in %s\n", img.Bounds().Dx(), img.Bounds().Dy(), time.Since(t2).Truncate(time.Millisecond))
	}

	t1 := time.Now()

	var (
		n   int
		err error
	)

	img, n, err = effects.ApplyAll(img)
	if err != nil {
		return err
	} else if n > 0 {
		logx.Fprintf(logger, "applied %d effect(s) in %s\n", n, time.Since(t1).Truncate(time.Millisecond))
	}

	if cmd.Bool("play") {
		logx.Fprintf(logger, "playing image...\n")

		return play.PlayImage(img)
	}

	t2 := time.Now()

	if len(plan.tiles) > 0 {
		tiles, err := resize.Split(img, plan.splitX, plan.splitY)
		if err != nil {
			return err
		}

		for i, tile := range tiles {
			row := i / plan.splitX
			col := i % plan.splitX
			tileOut := plan.tiles[i]

			written, err := writeOutput(tileOut, cmd.Bool("skip-existing"), logger, func(writer io.Writer) error {
				return plan.codec.Encode(writer, tile, localOpts)
			})

			if err != nil {
				return err
			}

			if written > 0 {
				logx.Fprintf(logger, "wrote tile r=%d c=%d -> %q\n", row, col, filepath.ToSlash(tileOut))
			}
		}

		return nil
	}

	written, err := writeOutput(plan.path, cmd.Bool("skip-existing"), logger, func(writer io.Writer) error {
		return plan.codec.Encode(writer, img, localOpts)
	})

	if err != nil {
		return err
	}

	logx.Fprintf(logger, "encoded %d KiB in %s\n", (written+1023)/1024, time.Since(t2).Truncate(time.Millisecond))

	return nil
}

func parseSplit(v string) (x, y int, ok bool, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, 0, false, nil
	}

	separators := []string{"x", "X", ":", ","}

	for _, sep := range separators {
		parts := strings.Split(v, sep)
		if len(parts) != 2 {
			continue
		}

		x, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, false, fmt.Errorf("invalid split x value: %q", parts[0])
		}

		y, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, false, fmt.Errorf("invalid split y value: %q", parts[1])
		}

		if x <= 0 || y <= 0 {
			return 0, 0, false, fmt.Errorf("split values must be > 0 (got %dx%d)", x, y)
		}

		return x, y, true, nil
	}

	return 0, 0, false, fmt.Errorf("invalid --split format %q (expected XxY, e.g. 3x2)", v)
}

func splitOutputPath(base string, row, col int) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	return fmt.Sprintf("%s_r%d_c%d%s", stem, row, col, ext)
}
