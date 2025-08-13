package integration

import (
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/icmd"
)

var (
	executable = "ffwebp"
	encodeOnly = map[string]bool{
		"heic": true,
		"heif": true,
		"psd":  true,
		"svg":  true,
	}
)

const (
	fixturesDir = "images"
	exampleJPEG = "example.jpeg"
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ffwebp-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	defer os.RemoveAll(tmp)

	if runtime.GOOS == "windows" {
		executable += ".exe"
	}

	executable = filepath.Join(tmp, executable)

	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	build := icmd.Command("go", "build", "-tags=full", "-o", executable, "./cmd/ffwebp")

	build.Dir = root

	res := icmd.RunCmd(build)
	if res.ExitCode != 0 {
		fmt.Fprintln(os.Stderr, res.Combined())
		os.Exit(res.ExitCode)
	}

	os.Exit(m.Run())
}

func TestCLI_CodecDecodeAndRoundTrip(t *testing.T) {
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read %s: %v", fixturesDir, err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		inPath := filepath.Join(fixturesDir, e.Name())
		ext := strings.TrimPrefix(filepath.Ext(e.Name()), ".")

		// test going from codec to jpeg
		t.Run("decode_"+ext, func(t *testing.T) {
			tmp := t.TempDir()

			out := filepath.Join(tmp, "decoded.jpeg")

			run(t, icmd.Command(executable, "-s", "-i", inPath, "-o", out))

			w, h := jpegSize(t, out)
			if w == 0 || h == 0 {
				t.Fatalf("decoded JPEG has no size: %dx%d", w, h)
			}
		})

		// test going from jpeg to codec then back to jpeg
		if encodeOnly[ext] {
			continue
		}

		t.Run("encode_"+ext, func(t *testing.T) {
			tmp := t.TempDir()

			mid := filepath.Join(tmp, "encoded."+ext)
			back := filepath.Join(tmp, "roundtrip.jpeg")

			run(t, icmd.Command(executable, "-s", "-i", exampleJPEG, "-o", mid))
			run(t, icmd.Command(executable, "-s", "-i", mid, "-o", back))

			w, h := jpegSize(t, back)
			if w != 256 || h != 256 {
				t.Fatalf("dimension mismatch: got %dx%d, want 256x256", w, h)
			}
		})
	}
}

func run(t *testing.T, cmd icmd.Cmd) {
	t.Helper()

	res := icmd.RunCmd(cmd)

	res.Assert(t, icmd.Success)
}

func jpegSize(t *testing.T, path string) (int, int) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	defer f.Close()

	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode jpeg config %s: %v", path, err)
	}

	return cfg.Width, cfg.Height
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)

		if parent == dir {
			return "", fmt.Errorf("could not locate go.mod starting from %s", dir)
		}

		dir = parent
	}
}
