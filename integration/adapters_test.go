package integration

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/icmd"
)

type losslessCase struct {
	codec string
	flags []string
}

type svgSizeCase struct {
	name   string
	flags  []string
	width  int
	height int
}

type iconHeaderCase struct {
	name   string
	ext    string
	forced string
	kind   uint16
}

func TestCLI_IconHeaders(t *testing.T) {
	cases := []iconHeaderCase{
		{name: "ico", ext: "ico", kind: 1},
		{name: "cur", ext: "cur", kind: 2},
		{name: "forced-cur", ext: "ico", forced: "cur", kind: 2},
		{name: "forced-ico", ext: "cur", forced: "ico", kind: 1},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "input.png")
			output := filepath.Join(directory, "encoded."+test.ext)
			back := filepath.Join(directory, "decoded.png")
			expected := losslessImage()

			writePNG(t, input, expected)

			arguments := []string{"-s", "-i", input, "-o", output}

			if test.forced != "" {
				arguments = append(arguments, "--codec", test.forced)
				output = filepath.Join(directory, "encoded."+test.forced)
			}

			run(t, icmd.Command(executable, arguments...))

			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}

			if len(data) < 22 || binary.LittleEndian.Uint16(data[:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != test.kind || binary.LittleEndian.Uint16(data[4:6]) != 1 {
				t.Fatalf("invalid icon/cursor directory: %x", data)
			}

			if test.kind == 2 && binary.LittleEndian.Uint32(data[10:14]) != 0 {
				t.Fatal("CUR default hotspot is not (0, 0)")
			}

			run(t, icmd.Command(executable, "-s", "-i", output, "-o", back))
			assertPixels(t, readPNG(t, back), expected)
		})
	}
}

func TestCLI_Lossless(t *testing.T) {
	cases := []losslessCase{
		{codec: "avif", flags: []string{"--avif.chroma", "420", "--avif.quality-alpha", "1", "--avif.speed", "10"}},
		{codec: "jxl", flags: []string{"--jpegxl.effort", "3"}},
	}

	for _, test := range cases {
		t.Run(test.codec, func(t *testing.T) {
			directory := t.TempDir()

			input := filepath.Join(directory, "input.png")
			encoded := filepath.Join(directory, "encoded."+test.codec)
			output := filepath.Join(directory, "output.png")

			expected := losslessImage()
			writePNG(t, input, expected)

			arguments := []string{"-s", "-i", input, "-o", encoded, "--lossless"}
			arguments = append(arguments, test.flags...)

			run(t, icmd.Command(executable, arguments...))
			run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", output))

			assertPixels(t, readPNG(t, output), expected)
		})
	}
}

func TestCLI_HEIFLossless(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "input.png")
	encoded := filepath.Join(directory, "encoded.heif")
	output := filepath.Join(directory, "output.png")

	expected := image.NewNRGBA(image.Rect(0, 0, 33, 31))

	// Grayscale avoids RGB-to-YCbCr rounding while exercising odd sizes and alpha.
	for row := range expected.Bounds().Dy() {
		for column := range expected.Bounds().Dx() {
			level := uint8(column*37 + row*19)
			alpha := uint8(column*13 + row*17)
			expected.SetNRGBA(column, row, color.NRGBA{R: level, G: level, B: level, A: alpha})
		}
	}

	writePNG(t, input, expected)

	run(t, icmd.Command(executable, "-s", "-i", input, "-o", encoded, "--codec", "heif", "--lossless"))
	run(t, icmd.Command(executable, "-s", "--sniff", "-i", encoded, "-o", output))

	assertPixels(t, readPNG(t, output), expected)
}

func TestCLI_TIFFCompression(t *testing.T) {
	for compression := range 5 {
		t.Run(fmt.Sprint(compression), func(t *testing.T) {
			directory := t.TempDir()

			input := filepath.Join(directory, "input.png")
			encoded := filepath.Join(directory, "encoded.tiff")
			output := filepath.Join(directory, "output.png")

			expected := losslessImage()
			writePNG(t, input, expected)

			command := icmd.Command(executable, "-s", "-i", input, "-o", encoded, "--tiff.compression", fmt.Sprint(compression), "--tiff.predictor")

			if compression > 2 {
				result := icmd.RunCmd(command)
				result.Assert(t, icmd.Expected{ExitCode: 1, Err: "invalid tiff.compression"})

				return
			}

			run(t, command)
			run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", output))

			assertPixels(t, readPNG(t, output), expected)
		})
	}
}

func TestCLI_SVGDimensions(t *testing.T) {
	cases := []svgSizeCase{
		{name: "intrinsic", width: 80, height: 40},
		{name: "both", flags: []string{"--svg.width", "24", "--svg.height", "18"}, width: 24, height: 18},
		{name: "width", flags: []string{"--svg.width", "24"}, width: 24, height: 12},
		{name: "height", flags: []string{"--svg.height", "18"}, width: 36, height: 18},
		{name: "unchanged", flags: []string{"--svg.width", "80"}, width: 80, height: 40},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()

			input := filepath.Join(directory, "input.svg")
			output := filepath.Join(directory, "output.png")

			writeBytes(t, input, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="80" height="40"><rect width="80" height="40" fill="red"/></svg>`))

			arguments := []string{"-s", "-i", input, "-o", output}
			arguments = append(arguments, test.flags...)

			run(t, icmd.Command(executable, arguments...))

			actual := readPNG(t, output)
			if actual.Bounds() != image.Rect(0, 0, test.width, test.height) {
				t.Fatalf("unexpected bounds: %v", actual.Bounds())
			}

			if color.NRGBAModel.Convert(actual.At(test.width/2, test.height/2)) != (color.NRGBA{R: 255, A: 255}) {
				t.Fatal("rendered image is not red")
			}
		})
	}
}

func TestCLI_SVGLargeIntrinsicSize(t *testing.T) {
	directory := t.TempDir()

	input := filepath.Join(directory, "input.svg")
	output := filepath.Join(directory, "output.png")

	writeBytes(t, input, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1000000" height="500000" viewBox="0 0 80 40"><rect width="80" height="40" fill="red"/></svg>`))

	// The intrinsic raster would exceed the renderer's memory limit; explicit dimensions must avoid it.
	run(t, icmd.Command(executable, "-s", "-i", input, "-o", output, "--svg.width", "16", "--svg.height", "8"))

	actual := readPNG(t, output)
	if actual.Bounds() != image.Rect(0, 0, 16, 8) {
		t.Fatalf("unexpected bounds: %v", actual.Bounds())
	}

	if color.NRGBAModel.Convert(actual.At(8, 4)) != (color.NRGBA{R: 255, A: 255}) {
		t.Fatal("rendered image is not red")
	}
}

func TestCLI_AVIFAnimation(t *testing.T) {
	directory := t.TempDir()

	input := filepath.Join(directory, "input.gif")
	encoded := filepath.Join(directory, "animated-without-extension")
	output := filepath.Join(directory, "output.gif")

	palette := color.Palette{color.Black, color.White}

	first := image.NewPaletted(image.Rect(0, 0, 16, 16), palette)
	second := image.NewPaletted(first.Bounds(), palette)

	for index := range second.Pix {
		second.Pix[index] = 1
	}

	animation := &gif.GIF{
		Image:     []*image.Paletted{first, second},
		Delay:     []int{7, 13},
		LoopCount: 0,
	}

	var buffer bytes.Buffer

	err := gif.EncodeAll(&buffer, animation)
	if err != nil {
		t.Fatal(err)
	}

	writeBytes(t, input, buffer.Bytes())

	run(t, icmd.Command(executable, "-s", "-i", input, "-o", encoded+".avif", "--lossless", "--avif.speed", "10", "--avif.chroma", "420", "--avif.quality-alpha", "1"))

	err = os.Rename(encoded+".avif", encoded)
	if err != nil {
		t.Fatal(err)
	}

	run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", output))

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if len(actual.Image) != 2 || len(actual.Delay) != 2 || actual.Delay[0] != 7 || actual.Delay[1] != 13 {
		t.Fatalf("unexpected animation: %d frames, delays %v", len(actual.Image), actual.Delay)
	}

	assertPixels(t, actual.Image[0], first)
	assertPixels(t, actual.Image[1], second)

	selections := []string{"--frame", "--time"}

	for _, selection := range selections {
		t.Run(strings.TrimPrefix(selection, "--"), func(t *testing.T) {
			value := "1"

			if selection == "--time" {
				value = "70ms"
			}

			selected := filepath.Join(directory, "selected.png")

			run(t, icmd.Command(executable, "-s", "-i", encoded, "-o", selected, selection, value))
			assertPixels(t, readPNG(t, selected), second)
		})
	}
}

func losslessImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))

	for row := range 32 {
		for column := range 32 {
			pixel := color.NRGBA{R: uint8(column*37 + row*19), G: uint8(column*11 + row*53), B: uint8(column*71 + row*7), A: 255}

			if row >= 16 {
				pixel = color.NRGBA{A: uint8(column*13 + row*17)}
			}

			img.SetNRGBA(column, row, pixel)
		}
	}

	return img
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()

	var buffer bytes.Buffer

	err := png.Encode(&buffer, img)
	if err != nil {
		t.Fatal(err)
	}

	writeBytes(t, path, buffer.Bytes())
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()

	err := os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	return img
}

func assertPixels(t *testing.T, actual, expected image.Image) {
	t.Helper()

	assertPixelsWithin(t, actual, expected, 0)
}

func assertPixelsWithin(t *testing.T, actual, expected image.Image, tolerance int) {
	t.Helper()

	if actual.Bounds() != expected.Bounds() {
		t.Fatalf("bounds: got %v, want %v", actual.Bounds(), expected.Bounds())
	}

	var total uint64

	for row := range expected.Bounds().Dy() {
		for column := range expected.Bounds().Dx() {
			got := color.RGBA64Model.Convert(actual.At(column, row)).(color.RGBA64)
			want := color.RGBA64Model.Convert(expected.At(column, row)).(color.RGBA64)

			channels := [4][2]uint16{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}, {got.A, want.A}}

			for index, pair := range channels {
				delta := int(pair[0]) - int(pair[1])
				if delta < 0 {
					total += uint64(-delta)
				} else {
					total += uint64(delta)
				}

				if delta < -tolerance*257 || delta > tolerance*257 {
					t.Fatalf("pixel (%d,%d) channel %d: got %v, want %v (tolerance %d/255)", column, row, index, got, want, tolerance)
				}
			}
		}
	}

	if tolerance > 0 {
		samples := uint64(expected.Bounds().Dx() * expected.Bounds().Dy() * 4)
		if total > samples*257 {
			t.Fatalf("mean channel error %.3f/255 exceeds 1/255", float64(total)/float64(samples*257))
		}
	}
}
