package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/icmd"
)

type filenamePatternCase struct {
	output string
	want   string
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
		{output: "{/.}-{#}.png", want: "mars-rgb-7.png"},
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
