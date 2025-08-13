package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

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
			if w, err := strconv.Atoi(wstr); err == nil && w > 0 {
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

	n := idx + startNum - 1

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

	sort.Strings(matches)

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

	num := `\\d+`

	if width > 0 {
		num = fmt.Sprintf(`\\d{%d}`, width)
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

	type match struct {
		path string
		n    int
	}

	var out []match

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

		out = append(out, match{path: full, n: n})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].n < out[j].n })

	result := make([]string, len(out))

	for i := range out {
		result[i] = out[i].path
	}

	return result, nil
}
