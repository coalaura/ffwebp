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
			Usage:   "output file, directory, or pattern (\"-\" = stdout, supports %d and templates)",
			Value:   "-",
		},
		&cli.BoolFlag{
			Name:    "play",
			Aliases: []string{"p"},
			Usage:   "play/display the input image in a GUI window instead of writing to a file (like ffplay)",
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
		Name:                   "ffwebp",
		Usage:                  "Convert any image format into any other image format",
		Version:                fmt.Sprintf("%s %s", Version, tags()),
		Flags:                  flags,
		Action:                 run,
		Writer:                 os.Stderr,
		ErrWriter:              os.Stderr,
		EnableShellCompletion:  true,
		UseShortOptionHandling: true,
		Suggest:                true,
		Commands: []*cli.Command{
			help.Command(),
		},
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

	if cmd.Bool("play") && len(inputs) > 1 {
		return errors.New("--play only supports a single input file")
	}

	startNum := cmd.Int("start-number")

	var outputs []string

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
				outs[i] = outDir
			}

			outputs = outs
		}
	}

	if len(inputs) == 1 {
		return processOne(inputs[0], outputs[0], cmd, &common, nil)
	}

	threads := cmd.Int("threads")

	if threads <= 0 {
		threads = runtime.NumCPU()
	}

	threads = min(threads, len(inputs))

	logx.Printf("using %d threads\n", threads)

	type job struct {
		i int
	}

	var (
		jobs = make(chan job)
		errs = make(chan error, len(inputs))
	)

	for w := 0; w < threads; w++ {
		go func() {
			for j := range jobs {
				var logger bytes.Buffer

				in := inputs[j.i]
				out := outputs[j.i]

				if err := processOne(in, out, cmd, &common, &logger); err != nil {
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

func processOne(input, output string, cmd *cli.Command, common *opts.Common, logger io.Writer) error {
	var (
		reader io.Reader    = os.Stdin
		writer *countWriter = &countWriter{
			w: os.Stdout,
		}
	)

	if input != "-" {
		logx.Fprintf(logger, "opening input file %q\n", filepath.ToSlash(input))

		file, err := os.OpenFile(input, os.O_RDONLY, 0)
		if err != nil {
			return err
		}

		defer file.Close()

		reader = file
	} else {
		logx.Fprintf(logger, "reading input from <stdin>\n")
	}

	forced := cmd.String("input.codec")

	sniffed, reader2, err := codec.Sniff(reader, input, forced, cmd.Bool("sniff"))
	if err != nil {
		return err
	}

	reader = reader2

	logx.Fprintf(logger, "sniffed codec: %s (%q, forced=%v)\n", sniffed.Codec, sniffed, forced != "")

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

	splitX, splitY, doSplit, err := parseSplit(cmd.String("split"))
	if err != nil {
		return err
	}

	doPlay := cmd.Bool("play")

	if doSplit {
		if doPlay {
			return errors.New("--split cannot be used with --play")
		}

		if output == "-" {
			return errors.New("--split requires file output (not stdout)")
		}
	}

	localOpts := *common

	var (
		oCodec codec.Codec
		oExt   string
	)

	if !doPlay {
		oCodec, oExt, err = codec.Detect(output, cmd.String("codec"))
		if err != nil {
			return err
		}

		localOpts.OutputExtension = oExt

		logx.Fprintf(logger, "output codec: %s (forced=%v)\n", oCodec, cmd.IsSet("codec"))

		if output != "-" {
			curExt := strings.TrimPrefix(filepath.Ext(output), ".")

			if mappedFromDir || curExt == "" || curExt != oExt {
				base := strings.TrimSuffix(output, filepath.Ext(output))
				output = base + "." + oExt
			}
		}

		if !doSplit {
			if output != "-" {
				if cmd.Bool("skip-existing") {
					if _, err := os.Stat(output); err == nil {
						logx.Fprintf(logger, "skipping %q (already exists)\n", filepath.ToSlash(output))

						return nil
					}
				}

				logx.Fprintf(logger, "opening output file %q\n", filepath.ToSlash(output))

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
				logx.Fprintf(logger, "writing output to <stdout>\n")
			}
		}
	}

	t0 := time.Now()

	animDecoder, hasAnimDecoder := sniffed.Codec.(codec.AnimatedDecoder)

	var (
		animEncoder    codec.AnimatedEncoder
		hasAnimEncoder bool
	)

	if oCodec != nil {
		animEncoder, hasAnimEncoder = oCodec.(codec.AnimatedEncoder)
	}

	frameIdx := cmd.Int("frame")
	timeMs := int(cmd.Duration("time").Milliseconds())

	var (
		anim *codec.Animation
		img  image.Image
	)

	if hasAnimDecoder {
		anim, err = animDecoder.DecodeAll(reader)
		if err != nil {
			return err
		} else if anim == nil {
			return errors.New("decoded animation is nil")
		} else if len(anim.Frames) == 0 {
			return errors.New("decoded animation has no frames")
		}

		if frameIdx >= 0 {
			if frameIdx >= len(anim.Frames) {
				return fmt.Errorf("frame index %d out of range (animation has %d frames)", frameIdx, len(anim.Frames))
			}

			img = anim.Frames[frameIdx]

			logx.Fprintf(logger, "selected frame %d from %d frame animation\n", frameIdx, len(anim.Frames))
		} else if cmd.IsSet("time") {
			var (
				cumulativeTime int
				selectedIdx    int
			)

			for i, delay := range anim.Delays {
				if cumulativeTime+delay > timeMs {
					selectedIdx = i

					break
				}

				cumulativeTime += delay
				selectedIdx = i
			}

			img = anim.Frames[selectedIdx]

			logx.Fprintf(logger, "selected frame %d at ~%dms from %d frame animation\n", selectedIdx, cumulativeTime, len(anim.Frames))
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

			if thumbnail := cmd.Uint("thumbnail"); thumbnail > 0 {
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

			err := animEncoder.EncodeAll(writer, anim, localOpts)
			if err != nil {
				return err
			}

			logx.Fprintf(logger, "encoded %d KiB in %s\n", (writer.n+1023)/1024, time.Since(t2).Truncate(time.Millisecond))

			return nil
		}

		if img == nil && len(anim.Frames) == 1 {
			img = anim.Frames[0]
		}
	}

	if img == nil {
		img, err = sniffed.Codec.Decode(reader)
		if err != nil {
			return err
		}
	}

	logx.Fprintf(logger, "decoded image: %dx%d %s in %s\n", img.Bounds().Dx(), img.Bounds().Dy(), colorModel(img), time.Since(t0).Truncate(time.Millisecond))

	if thumbnail := cmd.Uint("thumbnail"); thumbnail > 0 {
		t2 := time.Now()

		img = resize.Thumbnail(thumbnail, thumbnail, img)

		logx.Fprintf(logger, "resized image: %dx%d in %s\n", img.Bounds().Dx(), img.Bounds().Dy(), time.Since(t2).Truncate(time.Millisecond))
	}

	t1 := time.Now()

	var n int

	img, n, err = effects.ApplyAll(img)
	if err != nil {
		return err
	} else if n > 0 {
		logx.Fprintf(logger, "applied %d effect(s) in %s\n", n, time.Since(t1).Truncate(time.Millisecond))
	}

	if doPlay {
		logx.Fprintf(logger, "playing image...\n")

		return play.PlayImage(img)
	}

	t2 := time.Now()

	if doSplit {
		tiles, err := resize.Split(img, splitX, splitY)
		if err != nil {
			return err
		}

		for i, tile := range tiles {
			row := i / splitX
			col := i % splitX

			var tileOut string

			if hasTileTemplate(tileOut) {
				tileOut = formatTileTemplate(tileOut, row, col, i)
			} else {
				tileOut = splitOutputPath(output, row, col)
			}

			if err := os.MkdirAll(filepath.Dir(tileOut), 0755); err != nil {
				return err
			}

			file, err := os.OpenFile(tileOut, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}

			cw := &countWriter{w: file}

			encErr := oCodec.Encode(cw, tile, localOpts)

			closeErr := file.Close()
			if encErr != nil {
				return encErr
			}

			if closeErr != nil {
				return closeErr
			}

			logx.Fprintf(logger, "wrote tile r=%d c=%d -> %q\n", row, col, filepath.ToSlash(tileOut))
		}

		return nil
	}

	if err := oCodec.Encode(writer, img, localOpts); err != nil {
		return err
	}

	logx.Fprintf(logger, "encoded %d KiB in %s\n", (writer.n+1023)/1024, time.Since(t2).Truncate(time.Millisecond))

	return nil
}

func parseSplit(v string) (x, y int, ok bool, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, 0, false, nil
	}

	for _, sep := range []string{"x", "X", ":", ","} {
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
