package integration

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
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

type previewEntry struct {
	tag   uint16
	kind  uint16
	count uint32
	value uint32
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

			if compression > 1 {
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

func TestCLI_DNGPreview(t *testing.T) {
	preview, err := os.ReadFile(exampleJPEG)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := jpeg.Decode(bytes.NewReader(preview))
	if err != nil {
		t.Fatal(err)
	}

	orders := []binary.ByteOrder{binary.LittleEndian, binary.BigEndian}

	for _, order := range orders {
		t.Run(order.String(), func(t *testing.T) {
			directory := t.TempDir()

			input := filepath.Join(directory, "preview-without-extension")
			output := filepath.Join(directory, "output.png")

			writeBytes(t, input, dngPreview(preview, order))

			run(t, icmd.Command(executable, "-s", "-i", input, "-o", output))

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

func dngPreview(preview []byte, order binary.ByteOrder) []byte {
	entries := []previewEntry{
		{tag: 256, kind: 4, count: 1, value: 256},
		{tag: 257, kind: 4, count: 1, value: 256},
		{tag: 258, kind: 3, count: 1, value: 8},
		{tag: 259, kind: 3, count: 1, value: 7},
		{tag: 262, kind: 3, count: 1, value: 6},
		{tag: 273, kind: 4, count: 1, value: 0},
		{tag: 279, kind: 4, count: 1, value: uint32(len(preview))},
		{tag: 50706, kind: 1, count: 4, value: order.Uint32([]byte{1, 4, 0, 0})},
	}

	previewOffset := 8 + 2 + 12*len(entries) + 4
	entries[5].value = uint32(previewOffset)

	data := make([]byte, previewOffset+len(preview))
	copy(data, "II")

	if order == binary.BigEndian {
		copy(data, "MM")
	}

	order.PutUint16(data[2:4], 42)
	order.PutUint32(data[4:8], 8)
	order.PutUint16(data[8:10], uint16(len(entries)))

	for index, entry := range entries {
		offset := 10 + index*12

		order.PutUint16(data[offset:], entry.tag)
		order.PutUint16(data[offset+2:], entry.kind)
		order.PutUint32(data[offset+4:], entry.count)

		if entry.kind == 3 {
			order.PutUint16(data[offset+8:], uint16(entry.value))
		} else {
			order.PutUint32(data[offset+8:], entry.value)
		}
	}

	copy(data[previewOffset:], preview)

	return data
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

	if actual.Bounds() != expected.Bounds() {
		t.Fatalf("bounds: got %v, want %v", actual.Bounds(), expected.Bounds())
	}

	for row := range expected.Bounds().Dy() {
		for column := range expected.Bounds().Dx() {
			got := color.RGBA64Model.Convert(actual.At(column, row))
			want := color.RGBA64Model.Convert(expected.At(column, row))

			if got != want {
				t.Fatalf("pixel at (%d,%d): got %v, want %v", column, row, got, want)
			}
		}
	}
}
