package main

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/logx"
)

func codecList() []string {
	codecs := codec.All()

	names := make([]string, len(codecs))

	for i, c := range codecs {
		names[i] = c.String()

		if !c.CanEncode() {
			names[i] += "*"
		}
	}

	sort.Strings(names)

	return names
}

func tags() string {
	var (
		codec   = "none"
		feature = "none"
	)

	if effects.HasEffects() {
		feature = "effects"
	}

	codecs := codecList()

	if len(codecs) > 0 {
		codec = strings.Join(codecs, " ")
	}

	return fmt.Sprintf("[codecs: %s] [features: %s]", codec, feature)
}

func banner() {
	tags := codecList()

	if effects.HasEffects() {
		tags = append(tags, "effects")
	}

	if len(tags) == 0 {
		tags = []string{"none"}
	}

	logx.Printf("ffwebp version %s\n", Version)
	logx.Printf(
		"  built with %s %s %s\n",
		runtime.Compiler,
		runtime.Version(),
		runtime.GOARCH,
	)
	logx.Printf("  %s\n", strings.Join(tags, ","))
}
