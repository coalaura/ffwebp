// integration/integration_test.go
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

// Skip encoding tests for these extensions (same semantics as your original)
var encodeOnly = map[string]bool{
	"heic": true,
	"heif": true,
	"psd":  true,
	"svg":  true,
}

const (
	fixturesDir = "images"       // integration/images/
	exampleJPEG = "example.jpeg" // integration/example.jpeg (256x256)
)

var ffwebpBin string // set in TestMain

func TestMain(m *testing.M) {
	// Build the CLI once with -tags=full into a temp dir, reusing it across tests.
	tmp, err := os.MkdirTemp("", "ffwebp-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mktemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	exe := "ffwebp"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	ffwebpBin = filepath.Join(tmp, exe)

	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Build from repo root so relative paths to ./cmd/ffwebp resolve.
	build := icmd.Command("go", "build", "-tags=full", "-o", ffwebpBin, "./cmd/ffwebp")
	build.Dir = root
	res := icmd.RunCmd(build)
	if res.ExitCode != 0 {
		fmt.Fprintln(os.Stderr, res.Combined())
		os.Exit(res.ExitCode)
	}

	os.Exit(m.Run())
}

func TestCLI_CodecDecodeAndRoundTrip(t *testing.T) {
	// Discover input samples: integration/images/image.<ext>
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

		// 1) <codec> -> jpeg
		t.Run("decode_"+ext, func(t *testing.T) {
			tmp := t.TempDir()
			out := filepath.Join(tmp, "decoded.jpeg")

			run(t, icmd.Command(ffwebpBin, "-s", "-i", inPath, "-o", out))

			w, h := jpegSize(t, out)
			if w == 0 || h == 0 {
				t.Fatalf("decoded JPEG has invalid size: %dx%d", w, h)
			}
		})

		// 2) jpeg -> <codec> -> jpeg (dimension stays 256x256), unless encodeOnly
		if encodeOnly[ext] {
			continue
		}

		t.Run("encode_"+ext, func(t *testing.T) {
			tmp := t.TempDir()
			mid := filepath.Join(tmp, "encoded."+ext)
			back := filepath.Join(tmp, "roundtrip.jpeg")

			run(t, icmd.Command(ffwebpBin, "-s", "-i", exampleJPEG, "-o", mid))
			run(t, icmd.Command(ffwebpBin, "-s", "-i", mid, "-o", back))

			w, h := jpegSize(t, back)
			if w != 256 || h != 256 {
				t.Fatalf("roundtrip dimension mismatch: got %dx%d, want 256x256", w, h)
			}
		})
	}
}

func run(t *testing.T, cmd icmd.Cmd) {
	t.Helper()
	res := icmd.RunCmd(cmd)
	// On failure, icmd prints the command line + stdout/stderr, which is handy.
	res.Assert(t, icmd.Success)
}

func jpegSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
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
	// Walk up to find go.mod so we can run `go build ./cmd/ffwebp` reliably.
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
