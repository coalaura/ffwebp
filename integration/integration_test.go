package integration

import (
	"errors"
	"fmt"
	"image/jpeg"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// skip encoding tests for these extensions
var encodeOnly = map[string]bool{
	"heic": true,
	"heif": true,
	"psd":  true,
	"svg":  true,
}

func TestFFWebP(t *testing.T) {
	// build a fresh executable with "full" tags
	executable, err := buildExecutable()
	require.NoError(t, err)

	defer os.Remove(executable)

	// resolve all test files
	files, err := listFiles("images")
	require.NoError(t, err)

	// test all extensions
	for _, path := range files {
		ext := strings.TrimLeft(filepath.Ext(path), ".")

		decoded := "decoded.jpeg"
		encoded := fmt.Sprintf("encoded.%s", ext)

		// test if we can convert from the codec to jpeg
		t.Run(fmt.Sprintf("decode %s", ext), func(t *testing.T) {
			defer os.Remove(decoded)

			err = runCommand(executable, "-i", path, "-o", decoded)
			require.NoError(t, err)

			err = validateJPEG(decoded, 0)
			require.NoError(t, err)
		})

		if encodeOnly[ext] {
			continue
		}

		// test if we can convert from jpeg to the codec and then back to jpeg
		t.Run(fmt.Sprintf("encode %s", ext), func(t *testing.T) {
			defer os.Remove(encoded)
			defer os.Remove(decoded)

			err = runCommand(executable, "-i", "example.jpeg", "-o", encoded)
			require.NoError(t, err)

			err = runCommand(executable, "-i", encoded, "-o", decoded)
			require.NoError(t, err)

			err = validateJPEG(decoded, 256)
			require.NoError(t, err)
		})
	}
}

func buildExecutable() (string, error) {
	if runtime.GOOS == "windows" {
		err := runCommand("go", "build", "-tags", "full", "-o", "ffwebp.exe", "..\\cmd\\ffwebp")
		if err != nil {
			return "", err
		}

		return "./ffwebp.exe", nil
	}

	err := runCommand("go", "build", "-tags", "full", "-o", "ffwebp", "../cmd/ffwebp")
	if err != nil {
		return "", err
	}

	err = runCommand("chmod", "+x", "ffwebp")
	if err != nil {
		return "", err
	}

	return "./ffwebp", nil
}

func listFiles(directory string) ([]string, error) {
	var files []string

	err := filepath.Walk(directory, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		files = append(files, path)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return errors.New(string(out))
		}

		return err
	}

	return nil
}

func validateJPEG(path string, requireSize int) error {
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return err
	}

	bounds := img.Bounds()

	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return fmt.Errorf("invalid dimensions: %dx%d", bounds.Dx(), bounds.Dy())
	}

	if requireSize != 0 && (bounds.Dx() != requireSize || bounds.Dy() != requireSize) {
		return fmt.Errorf("mismatched size: %dx%dx", bounds.Dx(), bounds.Dy())
	}

	return nil
}
