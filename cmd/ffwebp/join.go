package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/coalaura/ffwebp/internal/resize"
	"github.com/urfave/cli/v3"
)

func validateJoinFlags(cmd *cli.Command) error {
	if cmd.IsSet("join") {
		if cmd.IsSet("split") {
			return errors.New("--join cannot be used with --split")
		}

		return nil
	}

	flags := []string{"join-background", "join-mode", "join-align", "join-cell"}

	for _, flag := range flags {
		if cmd.IsSet(flag) {
			return fmt.Errorf("--%s requires --join", flag)
		}
	}

	return nil
}

func processJoin(inputs []string, output string, cmd *cli.Command, common *opts.Common) error {
	options, err := parseJoinOptions(cmd, len(inputs))
	if err != nil {
		return err
	}

	plan, err := resolveJoinOutput(inputs, output, cmd)
	if err != nil {
		return err
	}

	if !cmd.Bool("play") {
		skipped, err := skipOutput(plan.path, cmd.Bool("skip-existing"), nil)
		if err != nil || skipped {
			return err
		}

		logx.Printf("output codec: %s (forced=%v)\n", plan.codec, cmd.IsSet("codec"))

		if joinOpaqueOutput(&plan, cmd) && options.Background.A < 255 {
			background := options.Background
			alpha := uint32(background.A)

			options.Background = color.NRGBA{
				R: uint8((uint32(background.R)*alpha + 255*(255-alpha) + 127) / 255),
				G: uint8((uint32(background.G)*alpha + 255*(255-alpha) + 127) / 255),
				B: uint8((uint32(background.B)*alpha + 255*(255-alpha) + 127) / 255),
				A: 255,
			}

			logx.Printf("output has no alpha; compositing join background over white\n")
		}
	}

	empty := options.Columns*options.Rows - len(inputs)

	logx.Printf("joining %d images into %dx%d grid; %d empty cell(s)\n", len(inputs), options.Columns, options.Rows, empty)

	if options.CellWidth > 0 {
		logx.Printf("join cells: %dx%d; mode=%s align=%s\n", options.CellWidth, options.CellHeight, options.Mode, cmd.String("join-align"))
	} else {
		logx.Printf("join cells: widest image per column, tallest per row; mode=%s align=%s\n", options.Mode, cmd.String("join-align"))
	}

	images := make([]image.Image, len(inputs))

	for index, input := range inputs {
		logx.Printf("join input %d: %q -> row=%d column=%d (zero-based)\n", index+1, filepath.ToSlash(input), index/options.Columns, index%options.Columns)

		img, animation, err := decodeInput(input, cmd, nil)
		if err != nil {
			return fmt.Errorf("join input %q: %w", input, err)
		}

		if animation != nil {
			img, err = selectAnimationFrame(animation, cmd, nil)
			if err != nil {
				return fmt.Errorf("join input %q: %w", input, err)
			}
		}

		images[index] = img
	}

	joined, err := resize.Join(images, options)
	if err != nil {
		return err
	}

	logx.Printf("joined canvas: %dx%d\n", joined.Bounds().Dx(), joined.Bounds().Dy())

	return processImage(joined, &plan, cmd, common, nil)
}

func parseJoinOptions(cmd *cli.Command, inputCount int) (resize.JoinOptions, error) {
	options := resize.JoinOptions{Mode: cmd.String("join-mode")}

	columns, rows, err := parseJoinDimensions(cmd.String("join"), true)
	if err != nil {
		return options, fmt.Errorf("--join: %w", err)
	}

	if inputCount == 0 {
		return options, errors.New("--join requires at least one input")
	}

	if columns == 0 {
		columns = (inputCount-1)/rows + 1
	} else if rows == 0 {
		rows = (inputCount-1)/columns + 1
	}

	if columns > resize.MaxJoinCells/rows {
		return options, fmt.Errorf("--join supports at most %d cells", resize.MaxJoinCells)
	}

	if inputCount > columns*rows {
		return options, fmt.Errorf("--join %dx%d has %d cells for %d images; use a larger grid or an auto dimension", columns, rows, columns*rows, inputCount)
	}

	options.Columns = columns
	options.Rows = rows

	if cmd.IsSet("join-cell") {
		options.CellWidth, options.CellHeight, err = parseJoinDimensions(cmd.String("join-cell"), false)
		if err != nil {
			return options, fmt.Errorf("--join-cell: %w", err)
		}
	}

	switch options.Mode {
	case "original", "fit", "cover", "stretch":
	default:
		return options, errors.New("--join-mode must be original, fit, cover or stretch")
	}

	options.AlignX, options.AlignY, err = parseJoinAlignment(cmd.String("join-align"))
	if err != nil {
		return options, err
	}

	options.Background, err = parseJoinBackground(cmd.String("join-background"))

	return options, err
}

func resolveJoinOutput(inputs []string, output string, cmd *cli.Command) (outputPlan, error) {
	if hasSeq(output) || hasTemplate(output) || isGlob(output) {
		return outputPlan{}, errors.New("--join requires one output filename or stdout, not an output pattern")
	}

	if output != "-" {
		info, err := os.Stat(output)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return outputPlan{}, err
		}

		if (info != nil && info.IsDir()) || output == "" || os.IsPathSeparator(output[len(output)-1]) {
			return outputPlan{}, errors.New("--join requires one output filename, not a directory")
		}
	}

	plan, err := resolveOutput(inputs[0], output, false, cmd, 0, 0)
	if err != nil || cmd.Bool("play") || output == "-" {
		return plan, err
	}

	destination, err := identifyPath(plan.path, -1)
	if err != nil {
		return plan, err
	}

	for index, input := range inputs {
		if input == "-" {
			continue
		}

		source, err := identifyPath(input, index)
		if err != nil {
			return plan, err
		}

		sameFile := source.info != nil && destination.info != nil && os.SameFile(source.info, destination.info)
		containsSource := strings.HasPrefix(source.key, destination.key+string(os.PathSeparator))

		if source.key == destination.key || sameFile || containsSource {
			return plan, fmt.Errorf("join destination %q would overwrite input %q", plan.path, input)
		}
	}

	return plan, nil
}

func parseJoinDimensions(value string, allowAuto bool) (int, int, error) {
	separator := strings.IndexAny(value, "xX:,")
	if separator < 0 {
		return 0, 0, errors.New("expected COLSxROWS or WIDTHxHEIGHT (e.g. 2x2)")
	}

	first, err := parseJoinDimension(value[:separator], allowAuto)
	if err != nil {
		return 0, 0, err
	}

	second, err := parseJoinDimension(value[separator+1:], allowAuto)
	if err != nil {
		return 0, 0, err
	}

	if first == 0 && second == 0 {
		return 0, 0, errors.New("only one dimension can be auto")
	}

	return first, second, nil
}

func parseJoinDimension(value string, allowAuto bool) (int, error) {
	value = strings.TrimSpace(value)
	if allowAuto && strings.EqualFold(value, "auto") {
		return 0, nil
	}

	dimension, err := strconv.Atoi(value)
	if err != nil || dimension <= 0 {
		return 0, fmt.Errorf("invalid dimension %q; expected a positive integer", value)
	}

	return dimension, nil
}

func parseJoinAlignment(value string) (int, int, error) {
	switch value {
	case "top-left":
		return 0, 0, nil
	case "top":
		return 1, 0, nil
	case "top-right":
		return 2, 0, nil
	case "left":
		return 0, 1, nil
	case "center":
		return 1, 1, nil
	case "right":
		return 2, 1, nil
	case "bottom-left":
		return 0, 2, nil
	case "bottom":
		return 1, 2, nil
	case "bottom-right":
		return 2, 2, nil
	}

	return 0, 0, fmt.Errorf("invalid --join-align %q; expected center, top, bottom, left, right or a corner such as top-left", value)
}

func parseJoinBackground(value string) (color.NRGBA, error) {
	switch strings.ToLower(value) {
	case "transparent":
		return color.NRGBA{}, nil
	case "white":
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}, nil
	case "black":
		return color.NRGBA{A: 255}, nil
	}

	invalid := fmt.Errorf("invalid --join-background %q; expected transparent, white, black, #RGB, #RGBA, #RRGGBB or #RRGGBBAA", value)

	if !strings.HasPrefix(value, "#") {
		return color.NRGBA{}, invalid
	}

	hex := value[1:]

	number, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.NRGBA{}, invalid
	}

	switch len(hex) {
	case 3:
		return color.NRGBA{R: uint8(number>>8) * 17, G: uint8(number>>4&15) * 17, B: uint8(number&15) * 17, A: 255}, nil
	case 4:
		return color.NRGBA{R: uint8(number>>12) * 17, G: uint8(number>>8&15) * 17, B: uint8(number>>4&15) * 17, A: uint8(number&15) * 17}, nil
	case 6:
		return color.NRGBA{R: uint8(number >> 16), G: uint8(number >> 8), B: uint8(number), A: 255}, nil
	case 8:
		return color.NRGBA{R: uint8(number >> 24), G: uint8(number >> 16), B: uint8(number >> 8), A: uint8(number)}, nil
	}

	return color.NRGBA{}, invalid
}

func joinOpaqueOutput(plan *outputPlan, cmd *cli.Command) bool {
	switch plan.codec.String() {
	case "jpeg", "bmp", "pcx", "xbm":
		return true
	case "pnm":
		format := strings.ToLower(cmd.String("pnm.format"))
		if format == "" {
			format = plan.extension
		}

		if format == "pbm" || format == "pgm" || format == "ppm" {
			return true
		}

		tuple := strings.ToUpper(cmd.String("pnm.pam-tupletype"))

		return tuple != "" && !strings.HasSuffix(tuple, "_ALPHA")
	}

	return false
}
