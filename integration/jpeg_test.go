package integration

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/icmd"
)

type jpegEncodingCase struct {
	name  string
	flags []string
	alpha bool
}

type jpegInvalidCase struct {
	name  string
	input string
	ext   string
	flags []string
	error string
}

type jpegLimitCase struct {
	codec string
	input string
}

func TestCLI_JPEGDetection(t *testing.T) {
	data := jpegBytes(t, "fixtures/inputs/jpeg-family.jls")

	var reference string

	for _, fixture := range fixtures(t) {
		if fixture.File == "inputs/jpeg-family.jls" {
			reference = filepath.Join(fixturesDir, fixture.Reference[0])
		}
	}

	modes := []string{"stdin", "misleading-extension", "forced-input"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			output := filepath.Join(directory, "out.png")
			input := "-"

			if mode != "stdin" {
				input = filepath.Join(directory, "wrong.jpg")
				writeBytes(t, input, data)
			}

			arguments := []string{"-s", "-i", input, "-o", output}

			if mode == "forced-input" {
				arguments = append(arguments, "--input.codec", "jpegls")
			}

			result := icmd.RunCmd(icmd.Command(executable, arguments...), icmd.WithStdin(bytes.NewReader(data)))
			result.Assert(t, icmd.Success)

			assertNativePixels(t, readPNG(t, output), readPNG(t, reference), 0)
		})
	}
}

func TestCLI_JPEGContainers(t *testing.T) {
	extensions := []string{"jp2", "j2k", "j2c"}

	for _, extension := range extensions {
		t.Run(extension, func(t *testing.T) {
			directory := t.TempDir()
			output := filepath.Join(directory, "out.jp2")

			run(t, icmd.Command(executable, "-s", "-i", "fixtures/inputs/mars-rgb.png", "-o", output, "-c", extension))

			encoded := filepath.Join(directory, "out."+extension)
			magic := []byte{0xff, 0x4f, 0xff, 0x51}

			if extension == "jp2" {
				magic = []byte("\x00\x00\x00\x0cjP  \r\n\x87\n")
			}

			if !bytes.HasPrefix(jpegBytes(t, encoded), magic) {
				t.Fatalf("forced %s produced the wrong container", extension)
			}
		})
	}

	containers := []bool{false, true}

	for _, container := range containers {
		t.Run(fmt.Sprintf("jxl-boxed-%t", container), func(t *testing.T) {
			directory := t.TempDir()
			encoded := filepath.Join(directory, "out.jxl")
			output := filepath.Join(directory, "out.png")

			arguments := []string{"-s", "-i", "fixtures/inputs/mars-rgb.png", "-o", encoded, "--lossless", "--jpegxl.effort", "3"}
			magic := []byte("\x00\x00\x00\x0cJXL \r\n\x87\n")

			if !container {
				arguments = append(arguments, "--jpegxl.container=false")
				magic = []byte{0xff, 0x0a}
			}

			run(t, icmd.Command(executable, arguments...))

			data := jpegBytes(t, encoded)
			if !bytes.HasPrefix(data, magic) {
				t.Fatal("JXL container choice was not honored")
			}

			result := icmd.RunCmd(icmd.Command(executable, "-s", "-i", "-", "-o", output), icmd.WithStdin(bytes.NewReader(data)))
			result.Assert(t, icmd.Success)

			assertPixels(t, readPNG(t, output), readPNG(t, "fixtures/inputs/mars-rgb.png"))
		})
	}
}

func TestCLI_JPEGPrecision(t *testing.T) {
	cases := []jpegEncodingCase{
		{name: "jpeg", flags: []string{"--lossless"}},
		{name: "jp2"},
		{name: "jls"},
		{name: "jxl", flags: []string{"--lossless", "--jpegxl.effort", "3"}},
		{name: "jp2", alpha: true},
		{name: "jls", flags: []string{"--jpegls.rgba"}, alpha: true},
		{name: "jxl", flags: []string{"--lossless", "--jpegxl.effort", "3"}, alpha: true},
	}

	for _, test := range cases {
		t.Run(fmt.Sprintf("%s-alpha-%t", test.name, test.alpha), func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "input.png")
			encoded := filepath.Join(directory, "encoded."+test.name)
			output := filepath.Join(directory, "output.png")

			expected := jpegPrecisionImage(test.alpha)
			writePNG(t, input, expected)

			arguments := []string{"-s", "-i", input, "-o", encoded}
			arguments = append(arguments, test.flags...)

			run(t, icmd.Command(executable, arguments...))

			arguments = []string{"-s", "-i", encoded, "-o", output}

			if test.name == "jls" && test.alpha {
				arguments = append(arguments, "--jpegls.rgba")
			}

			run(t, icmd.Command(executable, arguments...))
			assertNativePixels(t, readPNG(t, output), expected, 0)
		})
	}

	t.Run("near-native-units", func(t *testing.T) {
		directory := t.TempDir()
		input := filepath.Join(directory, "input.png")
		encoded := filepath.Join(directory, "near.jls")
		output := filepath.Join(directory, "output.png")

		expected := jpegPrecisionImage(false)
		writePNG(t, input, expected)

		run(t, icmd.Command(executable, "-s", "-i", input, "-o", encoded, "--jpegls.near", "7"))
		run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", output))

		assertNativePixels(t, readPNG(t, output), expected, 7)
	})
}

func TestCLI_JPEGQuality(t *testing.T) {
	codecs := []string{"jpg", "jp2", "jls", "jxl"}

	for _, codec := range codecs {
		t.Run(codec, func(t *testing.T) {
			directory := t.TempDir()
			qualities := []string{"default", "85", "0"}
			encoded := make([][]byte, len(qualities))

			for index, quality := range qualities {
				output := filepath.Join(directory, quality+"."+codec)
				arguments := []string{"-s", "-i", "fixtures/inputs/mars-rgb.png", "-o", output}

				if quality != "default" {
					arguments = append(arguments, "-q", quality)
				}

				run(t, icmd.Command(executable, arguments...))
				encoded[index] = jpegBytes(t, output)
			}

			if !bytes.Equal(encoded[0], encoded[1]) {
				t.Fatal("omitted quality no longer defaults to 85")
			}

			independent := codec == "jp2" || codec == "jls"
			if bytes.Equal(encoded[0], encoded[2]) != independent {
				t.Fatalf("incorrect quality-zero behavior for %s", codec)
			}
		})
	}
}

func TestCLI_JPEGGray(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.png")
	encoded := filepath.Join(directory, "gray.jpg")
	output := filepath.Join(directory, "output.png")

	source := jpegPrecisionImage(false)
	expected := image.NewGray16(source.Bounds())

	for y := range source.Bounds().Dy() {
		for x := range source.Bounds().Dx() {
			expected.Set(x, y, source.At(x, y))
		}
	}

	writePNG(t, input, source)

	run(t, icmd.Command(executable, "-s", "-i", input, "-o", encoded, "--lossless", "--jpeg.subsampling", "gray"))
	run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", output))

	assertNativePixels(t, readPNG(t, output), expected, 0)
}

func TestCLI_JPEGOpaqueJoin(t *testing.T) {
	extensions := []string{"jpg", "j2k", "jls"}

	for _, extension := range extensions {
		t.Run(extension, func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "input.png")
			encoded := filepath.Join(directory, "joined."+extension)
			output := filepath.Join(directory, "output.png")

			source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
			expected := image.NewNRGBA(source.Bounds())

			for y := range 2 {
				for x := range 2 {
					source.SetNRGBA(x, y, color.NRGBA{R: 255, A: 128})
					expected.SetNRGBA(x, y, color.NRGBA{R: 255, G: 127, B: 127, A: 255})
				}
			}

			writePNG(t, input, source)
			run(t, icmd.Command(executable, "-s", "-i", input, "-o", encoded, "--join", "1x1", "--lossless"))
			arguments := []string{"-s", "-i", encoded, "-o", output}

			if extension == "j2k" {
				arguments = append(arguments, "--jpeg2000.raw-color", "rgb")
			}

			run(t, icmd.Command(executable, arguments...))
			assertNativePixels(t, readPNG(t, output), expected, 0)
		})
	}
}

func TestCLI_JPEGBoundedStdin(t *testing.T) {
	cases := []jpegLimitCase{
		{codec: "jpeg", input: exampleJPEG},
		{codec: "jpeg2000", input: "fixtures/inputs/jpeg-family.jp2"},
		{codec: "jpegls", input: "fixtures/inputs/jpeg-family.jls"},
		{codec: "jpegxl", input: "fixtures/inputs/alpha.jxl"},
	}

	for _, test := range cases {
		t.Run(test.codec, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			output := filepath.Join(t.TempDir(), "output.png")
			command := exec.CommandContext(ctx, executable, "-s", "-i", "-", "-o", output, "--"+test.codec+".max-input-bytes", "32")

			var stderr bytes.Buffer

			command.Stderr = &stderr

			stdin, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}

			defer stdin.Close()

			err = command.Start()
			if err != nil {
				t.Fatal(err)
			}

			// Leave stdin open: a whole-stream sniff would wait for EOF instead of
			// returning the codec's configured input-limit error.
			stdin.Write(jpegBytes(t, test.input))

			err = command.Wait()
			if ctx.Err() != nil || err == nil || !strings.Contains(stderr.String(), "limit") {
				t.Fatalf("expected an input-limit error before EOF: %v, deadline %v, stderr %s", err, ctx.Err(), &stderr)
			}
		})
	}
}

func TestCLI_JPEGInvalidOptions(t *testing.T) {
	cases := []jpegInvalidCase{
		{name: "obsolete", ext: "jpg", flags: []string{"--jpeg.adaptive-quantization"}, error: "no equivalent"},
		{name: "jpeg-point", ext: "jpg", flags: []string{"--lossless", "--jpeg.point-transform", "1"}, error: "zero point transform"},
		{name: "jpeg-progressive", ext: "jpg", flags: []string{"--lossless", "--jpeg.progressive"}, error: "progressive"},
		{name: "jpeg-restarts", ext: "jpg", flags: []string{"--jpeg.restart-rows", "1", "--jpeg.restart-interval", "1"}, error: "restart"},
		{name: "jp2-targets", ext: "jp2", flags: []string{"--jpeg2000.ratios", "4,1", "--jpeg2000.psnr", "30,0"}, error: "mutually exclusive"},
		{name: "jp2-wavelets", ext: "jp2", flags: []string{"--lossless", "--jpeg2000.irreversible"}, error: "lossless"},
		{name: "raw-color", input: "fixtures/inputs/jpeg-family.j2k", ext: "png", error: "raw-color"},
		{name: "jls-near", ext: "jls", flags: []string{"--lossless", "--jpegls.near", "1"}, error: "NEAR=0"},
		{name: "jls-transform", ext: "jls", flags: []string{"--jpegls.near", "1", "--jpegls.transform", "hp1"}, error: "transform"},
		{name: "jxl-quality-distance", ext: "jxl", flags: []string{"-q", "0", "--jpegxl.distance", "0"}, error: "mutually exclusive"},
		{name: "jxl-distance-lossless", ext: "jxl", flags: []string{"--lossless", "--jpegxl.distance", "1"}, error: "conflicts"},
		{name: "jxl-strict-raw", ext: "jxl", flags: []string{"--jpegxl.container=false", "--jpegxl.codestream-level", "10"}, error: "container"},
		{name: "jxl-setting", ext: "jxl", flags: []string{"--jpegxl.setting", "unknown=0"}, error: "unknown setting"},
		{name: "jxl-missing-reconstruction", input: "fixtures/inputs/alpha.jxl", ext: "jpg", flags: []string{"--jpegxl.restore-jpeg"}, error: "reconstruction"},
		{name: "reconstruction-pixels", input: exampleJPEG, ext: "jxl", flags: []string{"--jpegxl.recompress-jpeg", "--frame", "0"}, error: "frame"},
		{name: "jpeg-limit", input: exampleJPEG, ext: "png", flags: []string{"--jpeg.max-input-bytes", "32"}, error: "limit"},
		{name: "jp2-limit", input: "fixtures/inputs/jpeg-family.jp2", ext: "png", flags: []string{"--jpeg2000.max-input-bytes", "32"}, error: "limit"},
		{name: "jls-limit", input: "fixtures/inputs/jpeg-family.jls", ext: "png", flags: []string{"--jpegls.max-input-bytes", "32"}, error: "limit"},
		{name: "jxl-limit", input: "fixtures/inputs/alpha.jxl", ext: "png", flags: []string{"--jpegxl.max-input-bytes", "32"}, error: "limit"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := test.input
			if input == "" {
				input = "fixtures/inputs/mars-rgb.png"
			}

			output := filepath.Join(t.TempDir(), "out."+test.ext)

			arguments := []string{"-s", "-i", input, "-o", output}
			arguments = append(arguments, test.flags...)

			result := icmd.RunCmd(icmd.Command(executable, arguments...))
			result.Assert(t, icmd.Expected{ExitCode: 1, Err: test.error})

			_, err := os.Stat(output)
			if !os.IsNotExist(err) {
				t.Fatalf("invalid options published output: %v", err)
			}
		})
	}
}

func TestCLI_JPEGXLAnimation(t *testing.T) {
	inputs := []string{"single-play.gif", "disposal.gif", "animation.webp"}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			var expected fixture

			for _, candidate := range fixtures(t) {
				if candidate.File == "inputs/"+input {
					expected = candidate
				}
			}

			directory := t.TempDir()
			encoded := filepath.Join(directory, "out.jxl")
			output := filepath.Join(directory, "out.gif")

			run(t, icmd.Command(executable, "-s", "-i", filepath.Join(fixturesDir, expected.File), "-o", encoded, "--lossless", "--jpegxl.effort", "3"))
			run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", output))

			animation, err := gif.DecodeAll(bytes.NewReader(jpegBytes(t, output)))
			if err != nil {
				t.Fatal(err)
			}

			for index := range animation.Delay {
				animation.Delay[index] *= 10
			}

			if animation.LoopCount != expected.Loops || !reflect.DeepEqual(animation.Delay, expected.Delays) {
				t.Fatalf("got delays %v loops %d; want %v loops %d", animation.Delay, animation.LoopCount, expected.Delays, expected.Loops)
			}
		})
	}
}

func TestCLI_JPEGReconstruction(t *testing.T) {
	directory := t.TempDir()
	encoded := filepath.Join(directory, "original.jxl")
	output := filepath.Join(directory, "restored.jpg")

	run(t, icmd.Command(executable, "-s", "-i", exampleJPEG, "-o", encoded, "--jpegxl.recompress-jpeg", "--jpegxl.effort", "3"))

	result := icmd.RunCmd(icmd.Command(executable, "-s", "-i", "-", "-o", output, "--jpegxl.restore-jpeg"), icmd.WithStdin(bytes.NewReader(jpegBytes(t, encoded))))
	result.Assert(t, icmd.Success)

	if !bytes.Equal(jpegBytes(t, output), jpegBytes(t, exampleJPEG)) {
		t.Fatal("JPEG reconstruction changed the original bytes")
	}
}

func jpegBytes(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func jpegPrecisionImage(alpha bool) *image.NRGBA64 {
	img := image.NewNRGBA64(image.Rect(0, 0, 9, 7))

	for y := range 7 {
		for x := range 9 {
			value := uint16(x*997 + y*541 + 1001)
			pixel := color.NRGBA64{R: value, G: value * 3, B: value * 5, A: 65535}

			if alpha {
				pixel.A = value * 2

				if x == 0 && y == 0 {
					pixel.A = 0
				}
			}

			img.SetNRGBA64(x, y, pixel)
		}
	}

	return img
}

func assertNativePixels(t *testing.T, actual, expected image.Image, tolerance int) {
	t.Helper()

	if actual.Bounds() != expected.Bounds() {
		t.Fatalf("bounds: got %v, want %v", actual.Bounds(), expected.Bounds())
	}

	for y := expected.Bounds().Min.Y; y < expected.Bounds().Max.Y; y++ {
		for x := expected.Bounds().Min.X; x < expected.Bounds().Max.X; x++ {
			got := color.NRGBA64Model.Convert(actual.At(x, y)).(color.NRGBA64)
			want := color.NRGBA64Model.Convert(expected.At(x, y)).(color.NRGBA64)

			actualChannels := []uint16{got.R, got.G, got.B, got.A}
			expectedChannels := []uint16{want.R, want.G, want.B, want.A}

			for channel := range actualChannels {
				difference := int(actualChannels[channel]) - int(expectedChannels[channel])
				if difference < -tolerance || difference > tolerance {
					t.Fatalf("pixel (%d,%d) channel %d: got %d, want %d within %d native units", x, y, channel, actualChannels[channel], expectedChannels[channel], tolerance)
				}
			}
		}
	}
}
