package main

import (
	"runtime"
	"sort"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/effects"
	"github.com/coalaura/ffwebp/internal/logx"
)

func banner() {
	codecs := codec.All()

	names := make([]string, len(codecs))

	for i, c := range codecs {
		names[i] = c.String()
	}

	sort.Strings(names)

	if effects.HasEffects() {
		names = append(names, "effects")
	}

	build := strings.Join(names, ",")

	logx.Printf("ffwebp version %s\n", Version)
	logx.Printf(
		"  built with %s %s %s\n",
		runtime.Compiler,
		runtime.Version(),
		runtime.GOARCH,
	)
	logx.Printf(
		"  configuration: -tags %s\n",
		build,
	)
}
