package main

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/urfave/cli/v3"
)

// maxPlannedTiles bounds split filename storage and filesystem preflight work
// across the invocation, while allowing practical grids of up to 100x100 tiles.
const maxPlannedTiles = 10_000

type outputPlan struct {
	path      string
	codec     codec.Codec
	extension string
	tiles     []string
	splitX    int
	splitY    int
}

type pathIdentity struct {
	path       string
	key        string
	info       os.FileInfo
	job        int
	file       fileKey
	hasFileKey bool
}

type fileKey struct {
	device uint64
	inode  uint64
}

type identityIndex struct {
	files    map[fileKey][2]*pathIdentity
	fallback []*pathIdentity
}

func (index *identityIndex) add(identity *pathIdentity) {
	if identity.info == nil {
		return
	}

	if identity.hasFileKey {
		// Two distinct jobs suffice to detect aliases while allowing one's own input.
		pair := index.files[identity.file]
		if pair[0] == nil {
			pair[0] = identity
		} else if pair[0].job != identity.job {
			pair[1] = identity
		}

		index.files[identity.file] = pair
	} else {
		index.fallback = append(index.fallback, identity)
	}
}

func (index *identityIndex) conflict(identity *pathIdentity, skipOwnJob bool) *pathIdentity {
	if identity.info == nil {
		return nil
	}

	if identity.hasFileKey {
		for _, candidate := range index.files[identity.file] {
			if candidate != nil && (!skipOwnJob || candidate.job != identity.job) {
				return candidate
			}
		}
	} else {
		// Only existing files without a native identity need a full identity scan.
		for _, candidates := range index.files {
			for _, candidate := range candidates {
				if candidate != nil && (!skipOwnJob || candidate.job != identity.job) && os.SameFile(identity.info, candidate.info) {
					return candidate
				}
			}
		}
	}

	for _, candidate := range index.fallback {
		if (!skipOwnJob || candidate.job != identity.job) && os.SameFile(identity.info, candidate.info) {
			return candidate
		}
	}

	return nil
}

func planSplit(cmd *cli.Command, inputCount int) (int, int, error) {
	splitX, splitY, split, err := parseSplit(cmd.String("split"))
	if err != nil {
		return 0, 0, err
	}

	if !split {
		return 0, 0, nil
	}

	if cmd.Bool("play") {
		return 0, 0, errors.New("--split cannot be used with --play")
	}

	// Divide before multiplying, including the aggregate across all inputs.
	if splitX > maxPlannedTiles/splitY || splitX*splitY > maxPlannedTiles/inputCount {
		// The rejected count can exceed even uint64; compute it exactly only here.
		var (
			requested big.Int
			factor    big.Int
		)

		requested.SetInt64(int64(splitX))
		requested.Mul(&requested, factor.SetInt64(int64(splitY)))
		requested.Mul(&requested, factor.SetInt64(int64(inputCount)))

		return 0, 0, fmt.Errorf("split requests %s tiles; supported limit is %d tiles per invocation", requested.String(), maxPlannedTiles)
	}

	return splitX, splitY, nil
}

// splitX and splitY have already passed the invocation-wide planning limit.
func resolveOutput(input, output string, directory bool, cmd *cli.Command, splitX, splitY int) (outputPlan, error) {
	plan := outputPlan{path: output}
	split := splitX > 0

	if cmd.Bool("play") {
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

	var err error

	plan.codec, plan.extension, err = codec.Detect(output, cmd.String("codec"))
	if err != nil {
		return plan, err
	}

	if output != "-" {
		output = trimExt(output) + "." + plan.extension
	}

	plan.path = output

	if split {
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

			destinations = append(destinations, destination)
		}
	}

	slices.SortFunc(sources, comparePathKeys)
	slices.SortFunc(destinations, comparePathKeys)

	sourceFiles := identityIndex{files: make(map[fileKey][2]*pathIdentity, len(sources))}
	destinationFiles := identityIndex{files: make(map[fileKey][2]*pathIdentity, len(destinations))}

	for index := range sources {
		sourceFiles.add(&sources[index])
	}

	for index := range destinations {
		destination := &destinations[index]

		var previous *pathIdentity

		if index > 0 && destinations[index-1].key == destination.key {
			previous = &destinations[index-1]
		} else {
			previous = descendantPath(destinations, destination, false)
		}

		if previous == nil {
			previous = destinationFiles.conflict(destination, false)
		}

		if previous != nil {
			return fmt.Errorf("conflicting destination %q for inputs %q and %q (other output %q)", destination.path, inputs[previous.job], inputs[destination.job], previous.path)
		}

		source := matchingSource(sources, destination)
		if source == nil {
			source = sourceFiles.conflict(destination, true)
		}

		if source != nil {
			return fmt.Errorf("destination %q for input %q would overwrite input %q", destination.path, inputs[destination.job], inputs[source.job])
		}

		destinationFiles.add(destination)
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

	if identity.info != nil {
		identity.file, identity.hasFileKey = identifyFile(absolute, identity.info)
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

func comparePathKeys(first, second pathIdentity) int {
	return strings.Compare(first.key, second.key)
}

func searchPathKey(identity pathIdentity, key string) int {
	return strings.Compare(identity.key, key)
}

func matchingSource(sources []pathIdentity, destination *pathIdentity) *pathIdentity {
	start, _ := slices.BinarySearchFunc(sources, destination.key, searchPathKey)

	for index := start; index < len(sources) && sources[index].key == destination.key; index++ {
		if sources[index].job != destination.job {
			return &sources[index]
		}
	}

	return descendantPath(sources, destination, true)
}

func descendantPath(paths []pathIdentity, parent *pathIdentity, skipOwnJob bool) *pathIdentity {
	prefix := parent.key
	if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
		prefix += string(os.PathSeparator)
	}

	// Search the separator-terminated prefix: a lexical neighbor like "a.png-1"
	// can otherwise hide a descendant like "a.png/tile.png".
	start, _ := slices.BinarySearchFunc(paths, prefix, searchPathKey)

	for index := start; index < len(paths) && strings.HasPrefix(paths[index].key, prefix); index++ {
		candidate := &paths[index]
		if candidate != parent && (!skipOwnJob || candidate.job != parent.job) {
			return candidate
		}
	}

	return nil
}
