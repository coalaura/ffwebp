package xbm

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image/color"
	"io"
	"strconv"
	"strings"
	"text/scanner"

	"golang.org/x/image/colornames"
)

// xpm v0.0.3 understands only None and #RRGGBB. Normalize the palette, leaving
// the pixel data untouched. The dependency's reader-only API requires buffering.
func normalizePalette(reader io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var (
		source    scanner.Scanner
		scanError error
	)

	source.Init(bytes.NewReader(data))
	source.Mode = scanner.ScanStrings | scanner.ScanComments | scanner.SkipComments

	source.Error = func(_ *scanner.Scanner, message string) {
		scanError = fmt.Errorf("xpm: %s", message)
	}

	header, _, _, err := nextString(&source)
	if err != nil {
		return nil, err
	}

	var (
		values     [4]int
		fieldIndex int
	)

	for field := range strings.FieldsSeq(header) {
		if fieldIndex == len(values) {
			break
		}

		values[fieldIndex], err = strconv.Atoi(field)
		if err != nil || values[fieldIndex] <= 0 {
			return nil, fmt.Errorf("xpm: invalid header")
		}

		fieldIndex++
	}

	if fieldIndex != len(values) || values[2] > len(data) || values[3] > len(data) {
		return nil, fmt.Errorf("xpm: invalid header")
	}

	colors := values[2]
	charsPerPixel := values[3]

	prefix := make([]byte, 0, source.Pos().Offset)
	normalized := make([]byte, 0, charsPerPixel+10)

	var copied int

	for range colors {
		line, start, end, err := nextString(&source)
		if err != nil {
			return nil, err
		}

		if len(line) < charsPerPixel {
			return nil, fmt.Errorf("xpm: short palette entry")
		}

		definition := strings.TrimSpace(line[charsPerPixel:])
		if len(definition) < 2 || definition[0] != 'c' || (definition[1] != ' ' && definition[1] != '\t') {
			return nil, fmt.Errorf("xpm: invalid palette entry %q", line)
		}

		value := strings.TrimSpace(definition[1:])

		pixel, err := paletteColor(value)
		if err != nil {
			return nil, err
		}

		// Reuse this small buffer across palette entries; the image rows are never copied here.
		normalized = append(normalized[:0], line[:charsPerPixel]...)
		normalized = append(normalized, " c "...)

		if pixel.A == 0 {
			normalized = append(normalized, "None"...)
		} else {
			normalized = append(normalized, '#')
			channels := [3]byte{pixel.R, pixel.G, pixel.B}
			normalized = hex.AppendEncode(normalized, channels[:])
		}

		prefix = append(prefix, data[copied:start]...)
		prefix = strconv.AppendQuote(prefix, string(normalized))

		copied = end
	}

	if scanError != nil {
		return nil, scanError
	}

	return io.MultiReader(bytes.NewReader(prefix), bytes.NewReader(data[copied:])), nil
}

func nextString(source *scanner.Scanner) (string, int, int, error) {
	for {
		token := source.Scan()
		if token == scanner.EOF {
			return "", 0, 0, fmt.Errorf("xpm: missing header or palette entry")
		}

		if token != scanner.String {
			continue
		}

		value, err := strconv.Unquote(source.TokenText())

		return value, source.Offset, source.Pos().Offset, err
	}
}

func paletteColor(value string) (color.RGBA, error) {
	if strings.EqualFold(value, "None") {
		return color.RGBA{}, nil
	}

	if strings.HasPrefix(value, "#") {
		var channels [3]byte

		if len(value) != 7 {
			return color.RGBA{}, fmt.Errorf("xpm: unsupported color %q", value)
		}

		_, err := hex.Decode(channels[:], []byte(value[1:]))
		if err != nil {
			return color.RGBA{}, fmt.Errorf("xpm: invalid color %q: %w", value, err)
		}

		return color.RGBA{R: channels[0], G: channels[1], B: channels[2], A: 255}, nil
	}

	name := strings.ToLower(strings.ReplaceAll(value, " ", ""))

	// These X11 names differ from the CSS colors in colornames.Map.
	switch name {
	case "gray", "grey":
		return color.RGBA{R: 190, G: 190, B: 190, A: 255}, nil
	case "green":
		return color.RGBA{G: 255, A: 255}, nil
	case "maroon":
		return color.RGBA{R: 176, G: 48, B: 96, A: 255}, nil
	case "purple":
		return color.RGBA{R: 160, G: 32, B: 240, A: 255}, nil
	}

	if pixel, ok := colornames.Map[name]; ok {
		return pixel, nil
	}

	return color.RGBA{}, fmt.Errorf("xpm: unsupported color %q", value)
}
