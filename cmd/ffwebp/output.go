package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/urfave/cli/v3"
)

type outputPlan struct {
	path      string
	codec     codec.Codec
	extension string
	tiles     []string
	splitX    int
	splitY    int
}

type pathIdentity struct {
	path string
	key  string
	info os.FileInfo
	job  int
}

func resolveOutput(input, output string, directory bool, cmd *cli.Command) (outputPlan, error) {
	plan := outputPlan{path: output}

	splitX, splitY, split, err := parseSplit(cmd.String("split"))
	if err != nil {
		return plan, err
	}

	if cmd.Bool("play") {
		if split {
			return plan, errors.New("--split cannot be used with --play")
		}

		return plan, nil
	}

	if split && output == "-" {
		return plan, errors.New("--split requires file output (not stdout)")
	}

	if output != "-" {
		info, statErr := os.Stat(output)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return plan, statErr
		}

		trailingSeparator := len(output) > 0 && os.IsPathSeparator(output[len(output)-1])

		directory = directory || (info != nil && info.IsDir()) || trailingSeparator
		if directory {
			output = filepath.Join(output, filepath.Base(input))
		}
	}

	plan.codec, plan.extension, err = codec.Detect(output, cmd.String("codec"))
	if err != nil {
		return plan, err
	}

	if output != "-" {
		output = trimExt(output) + "." + plan.extension
	}

	plan.path = output

	if split {
		if splitX > int(^uint(0)>>1)/splitY {
			return plan, errors.New("split tile count is too large")
		}

		plan.splitX = splitX
		plan.splitY = splitY
		plan.tiles = make([]string, 0, splitX*splitY)

		for row := range splitY {
			for column := range splitX {
				tile := splitOutputPath(output, row, column)

				if hasTileTemplate(output) {
					tile = formatTileTemplate(output, row, column, len(plan.tiles))
				}

				plan.tiles = append(plan.tiles, tile)
			}
		}
	}

	return plan, nil
}

func validateOutputs(inputs []string, plans []outputPlan) error {
	sources := make([]pathIdentity, 0, len(inputs))
	count := 0

	for index, input := range inputs {
		count += max(1, len(plans[index].tiles))

		if input == "-" {
			continue
		}

		identity, err := identifyPath(input, index)
		if err != nil {
			return fmt.Errorf("input %q: %w", input, err)
		}

		sources = append(sources, identity)
	}

	destinations := make([]pathIdentity, 0, count)

	for index := range plans {
		plan := &plans[index]

		paths := plan.tiles
		if len(paths) == 0 {
			paths = []string{plan.path}
		}

		for _, path := range paths {
			if path == "-" {
				continue
			}

			destination, err := identifyPath(path, index)
			if err != nil {
				return fmt.Errorf("output %q for input %q: %w", path, inputs[index], err)
			}

			for _, previous := range destinations {
				if samePath(destination, previous) || nestedPath(destination.key, previous.key) || nestedPath(previous.key, destination.key) {
					return fmt.Errorf("conflicting destination %q for inputs %q and %q (other output %q)", path, inputs[previous.job], inputs[index], previous.path)
				}
			}

			for _, source := range sources {
				if source.job != index && (samePath(destination, source) || nestedPath(destination.key, source.key)) {
					return fmt.Errorf("destination %q for input %q would overwrite input %q", path, inputs[index], inputs[source.job])
				}
			}

			destinations = append(destinations, destination)
		}
	}

	return nil
}

func identifyPath(path string, job int) (pathIdentity, error) {
	identity := pathIdentity{path: path, job: job}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return identity, err
	}

	identity.info, err = os.Stat(absolute)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return identity, err
	}

	identity.key, err = resolvePathAliases(absolute)
	if err != nil {
		return identity, err
	}

	// Windows path spelling is case-insensitive. Fold Darwin conservatively too:
	// its default filesystems are case-insensitive, including absent leaf names.
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		identity.key = strings.ToLower(identity.key)
	}

	return identity, nil
}

func resolvePathAliases(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}

	resolved, err = resolvePathAliases(parent)
	if err != nil {
		return "", err
	}

	return filepath.Join(resolved, filepath.Base(path)), nil
}

func samePath(first, second pathIdentity) bool {
	return first.key == second.key || (first.info != nil && second.info != nil && os.SameFile(first.info, second.info))
}

func nestedPath(parent, child string) bool {
	return strings.HasPrefix(child, parent+string(os.PathSeparator))
}
