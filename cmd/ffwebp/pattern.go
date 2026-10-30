package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const maxPatternWidth = 1000

type sequenceMatch struct {
	path string
	n    int
}

type outputPatternValues struct {
	inputIndex  int
	startNumber int
	sequence    int
	row         int
	column      int
	tile        int
	split       bool
}

var (
	sequencePattern     = regexp.MustCompile(`%[-+ #0]*[0-9]*(?:\.[0-9]+)?d`)
	numberFormatPattern = regexp.MustCompile(`^[-+ #0]*([1-9][0-9]*)?(?:\.([0-9]+))?d$`)
)

func hasSeq(s string) bool {
	return sequencePattern.MatchString(s)
}

func isGlob(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '*', '?', '[':
			return true
		}
	}

	return false
}

func expandGlob(pat string) ([]string, error) {
	matches, err := filepath.Glob(pat)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(matches, compareNaturalPaths)

	return matches, nil
}

func expandSeq(pat string) ([]string, error) {
	dir := filepath.Dir(pat)
	if dir == "." || dir == "" {
		dir = ""
	}

	base := filepath.Base(pat)

	location := sequencePattern.FindStringIndex(base)
	if location == nil {
		return nil, nil
	}

	width, err := numberFormatWidth(base[location[0]+1 : location[1]])
	if err != nil {
		return nil, err
	}

	pre := regexp.QuoteMeta(base[:location[0]])
	suf := regexp.QuoteMeta(base[location[1]:])

	re := regexp.MustCompile("^" + pre + `([ +\-0-9]+)` + suf + "$")

	scanDir := filepath.Dir(pat)

	if scanDir == "." || scanDir == "" {
		scanDir = "."
	}

	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil, err
	}

	var out []sequenceMatch

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		m := re.FindStringSubmatch(name)

		if m == nil || len(m[1]) < width {
			continue
		}

		n, err := strconv.Atoi(strings.TrimSpace(m[1]))
		if err != nil {
			continue
		}

		full := name

		if dir != "" {
			full = filepath.Join(filepath.Dir(pat), name)
		}

		out = append(out, sequenceMatch{path: full, n: n})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].n < out[j].n
	})

	result := make([]string, len(out))

	for i := range out {
		result[i] = out[i].path
	}

	return result, nil
}

func hasTemplate(s string) bool {
	if !strings.Contains(s, "{") {
		return false
	}

	return strings.Contains(s, "{/.}") || strings.Contains(s, "{/}") || strings.Contains(s, "{.}") || strings.Contains(s, "{}") || strings.Contains(s, "{//}") || hasNumericTemplate(s, "#rci")
}

func trimExt(p string) string {
	return strings.TrimSuffix(p, filepath.Ext(p))
}

func hasNumericTemplate(pattern, keys string) bool {
	for len(pattern) > 0 {
		start := strings.IndexByte(pattern, '{')
		if start < 0 {
			return false
		}

		pattern = pattern[start+1:]

		end := strings.IndexByte(pattern, '}')
		if end < 0 {
			return false
		}

		token := pattern[:end]
		if isNumericToken(token, keys) {
			return true
		}

		pattern = pattern[end+1:]
	}

	return false
}

func formatOutputPattern(pattern, input string, values outputPatternValues) (string, error) {
	if !strings.ContainsAny(pattern, "%{") {
		return pattern, nil
	}

	directory := filepath.Dir(input)
	base := filepath.Base(input)

	if input == "-" {
		directory = "."
		base = "stdin"
	}

	var output strings.Builder

	output.Grow(len(pattern))

	// Expand the original pattern once; substituted filenames are literal data.
	for index := 0; index < len(pattern); {
		if pattern[index] == '%' {
			location := sequencePattern.FindStringIndex(pattern[index:])
			if location != nil && location[0] == 0 {
				end := index + location[1]

				err := appendPatternNumber(&output, pattern[index+1:end], values.sequence, values.startNumber)
				if err != nil {
					return "", err
				}

				index = end

				continue
			}
		}

		if pattern[index] != '{' {
			output.WriteByte(pattern[index])
			index++

			continue
		}

		end := strings.IndexByte(pattern[index:], '}')
		if end < 0 {
			output.WriteString(pattern[index:])

			break
		}

		end += index
		token := pattern[index+1 : end]

		switch token {
		case "":
			output.WriteString(input)
		case ".":
			output.WriteString(trimExt(input))
		case "/":
			output.WriteString(base)
		case "/.":
			output.WriteString(trimExt(base))
		case "//":
			output.WriteString(directory)
		default:
			if isNumericToken(token, "#rci") {
				if token[0] != '#' && !values.split {
					return "", fmt.Errorf("output placeholder {%s} requires --split", token)
				}

				err := appendNumericToken(&output, token, values)
				if err != nil {
					return "", err
				}
			} else {
				output.WriteString(pattern[index : end+1])
			}
		}

		index = end + 1
	}

	return output.String(), nil
}

func isNumericToken(token, keys string) bool {
	return len(token) > 0 && strings.IndexByte(keys, token[0]) >= 0 && (len(token) == 1 || token[1] == ':')
}

func appendNumericToken(output *strings.Builder, token string, values outputPatternValues) error {
	format := "d"

	if len(token) > 1 {
		format = token[2:]
	}

	var (
		number int
		start  int
	)

	switch token[0] {
	case '#':
		number = values.inputIndex
		start = values.startNumber
	case 'r':
		number = values.row
	case 'c':
		number = values.column
	case 'i':
		number = values.tile
	}

	return appendPatternNumber(output, format, number, start)
}

func appendPatternNumber(output *strings.Builder, format string, number, start int) error {
	_, err := numberFormatWidth(format)
	if err != nil {
		return err
	}

	if start > math.MaxInt-number {
		return fmt.Errorf("output sequence exceeds the maximum integer; use a smaller --start-number")
	}

	fmt.Fprintf(output, "%"+format, number+start)

	return nil
}

func numberFormatWidth(format string) (int, error) {
	parts := numberFormatPattern.FindStringSubmatch(format)
	if parts == nil {
		return 0, fmt.Errorf("invalid numeric format %q: expected a printf decimal format such as d, 2d or 02d", format)
	}

	var width int

	for index, part := range parts[1:] {
		if part == "" {
			continue
		}

		number, err := strconv.Atoi(part)
		if err != nil || number > maxPatternWidth {
			return 0, fmt.Errorf("numeric format width and precision must not exceed %d", maxPatternWidth)
		}

		if index == 0 {
			width = number
		}
	}

	return width, nil
}

func expandInputs(patterns []string) ([]string, error) {
	inputs := make([]string, 0, len(patterns))

	var stdin bool

	for _, pattern := range patterns {
		if pattern == "-" {
			if stdin {
				return nil, fmt.Errorf("stdin can only be selected once")
			}

			stdin = true
		}

		var (
			matches []string
			err     error
		)

		switch {
		case hasSeq(pattern):
			matches, err = expandSeq(pattern)
		case isGlob(pattern):
			matches, err = expandGlob(pattern)
		default:
			inputs = append(inputs, pattern)

			continue
		}

		if err != nil {
			return nil, err
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("no inputs match pattern: %s", filepath.ToSlash(pattern))
		}

		inputs = append(inputs, matches...)
	}

	return inputs, nil
}

func compareNaturalPaths(first, second string) int {
	left := first
	right := second

	for len(left) > 0 && len(right) > 0 {
		if left[0] >= '0' && left[0] <= '9' && right[0] >= '0' && right[0] <= '9' {
			leftEnd := digitRunEnd(left)
			rightEnd := digitRunEnd(right)

			leftDigits := strings.TrimLeft(left[:leftEnd], "0")
			rightDigits := strings.TrimLeft(right[:rightEnd], "0")

			// Compare digit counts before contents, without integer overflow.
			if len(leftDigits) != len(rightDigits) {
				return len(leftDigits) - len(rightDigits)
			}

			comparison := strings.Compare(leftDigits, rightDigits)
			if comparison != 0 {
				return comparison
			}

			left = left[leftEnd:]
			right = right[rightEnd:]

			continue
		}

		if left[0] != right[0] {
			return int(left[0]) - int(right[0])
		}

		left = left[1:]
		right = right[1:]
	}

	if len(left) != len(right) {
		return len(left) - len(right)
	}

	return strings.Compare(first, second)
}

func digitRunEnd(value string) int {
	var index int

	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}

	return index
}
