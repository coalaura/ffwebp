package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type sequenceMatch struct {
	path string
	n    int
}

func hasSeq(s string) bool {
	re := regexp.MustCompile(`%0?\d*d`)

	return re.FindStringIndex(s) != nil
}

func seqSpec(s string) (start, end, width int, zeroPad bool, ok bool) {
	re := regexp.MustCompile(`%0?(\d*)d`)

	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		return 0, 0, 0, false, false
	}

	start = loc[0]
	end = loc[1]

	if loc[2] != -1 && loc[3] != -1 {
		wstr := s[loc[2]:loc[3]]

		if wstr != "" {
			w, err := strconv.Atoi(wstr)
			if err == nil && w > 0 {
				width = w
			}
		}
	}

	zeroPad = width > 0

	return start, end, width, zeroPad, true
}

func formatSeq(pattern string, idx, startNum int) string {
	s, e, width, zero, ok := seqSpec(pattern)
	if !ok {
		return pattern
	}

	n := idx + startNum

	var num string

	if width > 0 && zero {
		num = fmt.Sprintf("%0*d", width, n)
	} else {
		num = fmt.Sprintf("%d", n)
	}

	return pattern[:s] + num + pattern[e:]
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

	s, e, width, _, ok := seqSpec(base)
	if !ok {
		return nil, nil
	}

	pre := regexp.QuoteMeta(base[:s])
	suf := regexp.QuoteMeta(base[e:])

	num := `\d+`

	if width > 0 {
		num = fmt.Sprintf(`\d{%d,}`, width)
	}

	re := regexp.MustCompile("^" + pre + "(" + num + ")" + suf + "$")

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

		if m == nil {
			continue
		}

		n, err := strconv.Atoi(m[1])
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

	return strings.Contains(s, "{/.}") || strings.Contains(s, "{/}") || strings.Contains(s, "{.}") || strings.Contains(s, "{}") || strings.Contains(s, "{//}") || strings.Contains(s, "{#}") || hasTileTemplate(s)
}

func trimExt(p string) string {
	return strings.TrimSuffix(p, filepath.Ext(p))
}

func formatTemplate(pattern, in string, idx, startNum int) string {
	if hasSeq(pattern) {
		pattern = formatSeq(pattern, idx, startNum)
	}

	dir := filepath.Dir(in)
	base := filepath.Base(in)

	// Special-case "-" (stdin)
	if in == "-" {
		dir = "."
		base = "stdin"
	}

	fullNoExt := trimExt(in)
	baseNoExt := trimExt(base)
	n := idx + startNum

	r := strings.NewReplacer(
		"{/.}", baseNoExt,
		"{//}", dir,
		"{/}", base,
		"{.}", fullNoExt,
		"{}", in,
		"{#}", strconv.Itoa(n),
	)

	return r.Replace(pattern)
}

func hasTileTemplate(s string) bool {
	return strings.Contains(s, "{r}") || strings.Contains(s, "{c}") || strings.Contains(s, "{i}")
}

func formatTileTemplate(pattern string, row, col, idx int) string {
	r := strings.NewReplacer(
		"{r}", strconv.Itoa(row),
		"{c}", strconv.Itoa(col),
		"{i}", strconv.Itoa(idx),
	)

	return r.Replace(pattern)
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
