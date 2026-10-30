package integration

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/icmd"
)

type filenamePatternCase struct {
	output string
	want   string
}

type destinationConflictCase struct {
	name   string
	inputs []string
	output string
	flags  []string
	err    string
}

type splitPlanningCase struct {
	name   string
	split  string
	inputs int
	err    string
}

func TestCLI_OutputSafety(t *testing.T) {
	existingStates := []bool{false, true}

	for _, existing := range existingStates {
		name := "new"

		if existing {
			name = "existing"
		}

		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			output := filepath.Join(dir, "out.gif")

			sentinel := []byte("keep this existing file")

			if existing {
				writeBytes(t, output, sentinel)
			}

			// A valid flag that fails inside the encoder, after output creation.
			res := icmd.RunCmd(icmd.Command(executable, "-s", "-i", "fixtures/inputs/alpha.png", "-o", output, "--gif.colors", "1"))
			res.Assert(t, icmd.Expected{ExitCode: 1})

			actual, err := os.ReadFile(output)

			if existing {
				if err != nil || !bytes.Equal(actual, sentinel) {
					t.Fatalf("existing output changed: %q, %v", actual, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("failed encoder published an output: %v", err)
			}

			entries, err := filepath.Glob(filepath.Join(dir, ".ffwebp-*"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary output leaked: %v, %v", entries, err)
			}
		})
	}

	t.Run("skip-existing", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "out.png")
		sentinel := []byte("keep me")

		writeBytes(t, output, sentinel)

		run(t, icmd.Command(executable, "-s", "-i", "fixtures/inputs/alpha.png", "-o", output, "--skip-existing"))

		actual, err := os.ReadFile(output)
		if err != nil || !bytes.Equal(actual, sentinel) {
			t.Fatalf("skip-existing changed output: %q, %v", actual, err)
		}
	})
}

func TestCLI_FilenamePatterns(t *testing.T) {
	dir := t.TempDir()

	source := "fixtures/inputs/mars-rgb.png"
	expected := readPNG(t, source)

	patterns := []filenamePatternCase{
		{output: "single-%03d.png", want: "single-007.png"},
		{output: "single-%3d.png", want: "single-  7.png"},
		{output: "{/.}-{#}.png", want: "mars-rgb-7.png"},
		{output: "{/.}-{#:03d}.png", want: "mars-rgb-007.png"},
		{output: "{#:3d}-{#:-3d}-{#:+03d}.png", want: "  7-7  -+07.png"},
		{output: "literal-{unknown}.png", want: "literal-{unknown}.png"},
	}

	for _, pattern := range patterns {
		t.Run(pattern.output, func(t *testing.T) {
			// Template tokens contain literal forward slashes, including on Windows.
			output := filepath.ToSlash(dir) + "/" + pattern.output

			run(t, icmd.Command(executable, "-s", "-i", source, "--start-number", "7", output))

			assertPixels(t, readPNG(t, filepath.Join(dir, pattern.want)), expected)
		})
	}
}

func TestCLI_OutputPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits and umask are not supported on Windows")
	}

	output := filepath.Join(t.TempDir(), "out.png")
	command := icmd.Command("sh", "-c", `umask 077; exec "$@"`, "ffwebp-test", executable, "-s", "-i", "fixtures/inputs/alpha.png", "-o", output)

	run(t, command)
	assertPermissions(t, output, 0600)

	err := os.Chmod(output, 0640)
	if err != nil {
		t.Fatal(err)
	}

	run(t, command)
	assertPermissions(t, output, 0640)
}

func TestCLI_SamePath(t *testing.T) {
	extensions := []string{"png", "gif"}

	for _, extension := range extensions {
		t.Run(extension, func(t *testing.T) {
			source := "fixtures/inputs/alpha.png"

			if extension == "gif" {
				source = "fixtures/inputs/disposal.gif"
			}

			data, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}

			output := filepath.Join(t.TempDir(), "same."+extension)
			writeBytes(t, output, data)

			run(t, icmd.Command(executable, "-s", "-i", output, "-o", output))

			if extension == "png" {
				assertPixels(t, readPNG(t, output), readPNG(t, source))

				return
			}

			encoded, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}

			animation, err := gif.DecodeAll(bytes.NewReader(encoded))
			if err != nil {
				t.Fatal(err)
			}

			if len(animation.Image) != 4 {
				t.Fatalf("same-path animation has %d frames", len(animation.Image))
			}
		})
	}
}

func TestCLI_SkipAnimationAndTiles(t *testing.T) {
	directory := t.TempDir()
	animation := filepath.Join(directory, "out.gif")
	firstTile := filepath.Join(directory, "tile-0.png")
	secondTile := filepath.Join(directory, "tile-1.png")

	sentinel := []byte("preserve existing output")

	writeBytes(t, animation, sentinel)
	writeBytes(t, firstTile, sentinel)

	run(t, icmd.Command(executable, "-s", "-i", "fixtures/inputs/disposal.gif", "-o", animation, "--skip-existing"))
	run(t, icmd.Command(executable, "-s", "-i", "fixtures/inputs/alpha.png", "-o", filepath.Join(directory, "tile-{i}.png"), "--split", "2x1", "--skip-existing"))

	paths := []string{animation, firstTile}

	for _, path := range paths {
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, sentinel) {
			t.Fatalf("skipped output changed: %q, %v", actual, err)
		}
	}

	if readPNG(t, secondTile).Bounds() != image.Rect(0, 0, 32, 47) {
		t.Fatal("missing or incorrectly sized second tile")
	}

	// An absent destination is published too, for both static and animated paths.
	err := os.Remove(animation)
	if err != nil {
		t.Fatal(err)
	}

	run(t, icmd.Command(executable, "-s", "-i", "fixtures/inputs/disposal.gif", "-o", animation, "--skip-existing"))

	data, err := os.ReadFile(animation)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil || len(decoded.Image) != 4 {
		t.Fatalf("animation publication failed: %v", err)
	}
}

func TestCLI_PaddedSequenceOverflow(t *testing.T) {
	directory := t.TempDir()

	numbers := []int{1000, 10, 999, 1}
	colors := map[int]color.NRGBA{
		1:    {R: 255, A: 255},
		10:   {G: 255, A: 255},
		999:  {B: 255, A: 255},
		1000: {R: 255, G: 255, A: 255},
	}

	for _, number := range numbers {
		img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
		img.SetNRGBA(0, 0, colors[number])
		img.SetNRGBA(1, 0, colors[number])

		writePNG(t, filepath.Join(directory, fmt.Sprintf("%03d.png", number)), img)
	}

	output := filepath.Join(directory, "out-%03d-{i}.png")

	run(t, icmd.Command(executable, "-s", "-i", filepath.Join(directory, "%03d.png"), "-o", output, "--start-number", "7", "--split", "2x1"))

	ordered := []int{1, 10, 999, 1000}

	for index, number := range ordered {
		for tile := range 2 {
			sequence := index*2 + tile + 7

			actual := readPNG(t, filepath.Join(directory, fmt.Sprintf("out-%03d-%d.png", sequence, tile)))
			if actual.Bounds() != image.Rect(0, 0, 1, 1) || color.NRGBAModel.Convert(actual.At(0, 0)) != colors[number] {
				t.Fatalf("incorrect sequence order or tile at output %d", sequence)
			}
		}
	}
}

func TestCLI_DestinationConflicts(t *testing.T) {
	cases := []destinationConflictCase{
		{name: "directory", inputs: []string{"photo.jpg", "photo.png"}, output: "out", flags: []string{"--codec", "webp"}, err: "conflicting destination"},
		{name: "template", inputs: []string{"photo.jpg", "photo.png"}, output: "out/{/.}.webp", err: "conflicting destination"},
		{name: "split-batch", inputs: []string{"photo.jpg", "photo.png"}, output: "out/{/.}-{i}.webp", flags: []string{"--split", "2x1"}, err: "conflicting destination"},
		{name: "split-template", inputs: []string{"photo.png"}, output: "out/tile-{r}.png", flags: []string{"--split", "2x1"}, err: "conflicting destination"},
		{name: "other-input", inputs: []string{"001.png", "002.png"}, output: "%03d.png", flags: []string{"--start-number", "2"}, err: "would overwrite input"},
		{name: "split-input", inputs: []string{"tile.png", "tile_r0_c0.png"}, output: "{/.}_r{r}_c{c}.png", flags: []string{"--split", "1x1"}, err: "would overwrite input"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			sentinel := []byte("validation must precede decoding and writing")

			for _, input := range test.inputs {
				writeBytes(t, filepath.Join(directory, input), sentinel)
			}

			arguments := []string{"-s", "-i", filepath.Join(directory, "*.*"), "-o", filepath.ToSlash(directory) + "/" + test.output}
			arguments = append(arguments, test.flags...)

			result := icmd.RunCmd(icmd.Command(executable, arguments...))
			result.Assert(t, icmd.Expected{ExitCode: 1, Err: test.err})

			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != len(test.inputs) {
				t.Fatalf("validation created output: %v, %v", entries, err)
			}

			for _, input := range test.inputs {
				actual, err := os.ReadFile(filepath.Join(directory, input))
				if err != nil || !bytes.Equal(actual, sentinel) {
					t.Fatalf("validation changed input %q: %v", input, err)
				}
			}
		})
	}
}

func TestCLI_SplitPlanningLimit(t *testing.T) {
	cases := []splitPlanningCase{
		{name: "excessive", split: "1000x1000", inputs: 1, err: "split requests 1000000 tiles; supported limit is 10000"},
		{name: "aggregate", split: "100x60", inputs: 2, err: "split requests 12000 tiles; supported limit is 10000"},
		{name: "multiplication", split: "2147483647x2147483647", inputs: 5, err: "split requests 23058430070662103045 tiles; supported limit is 10000"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			sentinel := []byte("must fail before decoding or writing")
			paths := make([]string, 0, test.inputs+1)

			for index := range test.inputs {
				paths = append(paths, filepath.Join(directory, fmt.Sprintf("input-%d.png", index)))
			}

			paths = append(paths, filepath.Join(directory, "tile-1-0.png"))

			for _, path := range paths {
				writeBytes(t, path, sentinel)
			}

			result := icmd.RunCmd(icmd.Command(executable, "-s", "-i", filepath.Join(directory, "input-*.png"), "-o", filepath.Join(directory, "tile-{#}-{i}.png"), "--split", test.split))
			result.Assert(t, icmd.Expected{ExitCode: 1, Err: test.err})

			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != len(paths) {
				t.Fatalf("planning created output: %v, %v", entries, err)
			}

			for _, path := range paths {
				actual, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(actual, sentinel) {
					t.Fatalf("planning changed %q: %v", path, err)
				}
			}
		})
	}
}

func TestCLI_SplitDimensionsWithinLimit(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "tile-{i}.png")

	result := icmd.RunCmd(icmd.Command(executable, "-s", "-i", "fixtures/inputs/alpha.png", "-o", output, "--split", "100x100"))
	result.Assert(t, icmd.Expected{ExitCode: 1, Err: "x/y too large for image dimensions"})

	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid dimensions created output: %v, %v", entries, err)
	}
}

func assertPermissions(t *testing.T, path string, expected os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != expected {
		t.Fatalf("permissions: got %04o, want %04o", info.Mode().Perm(), expected)
	}
}
