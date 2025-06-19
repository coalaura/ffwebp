package logx

import (
	"fmt"
	"image"
	"os"
	"sync/atomic"
	"time"
)

var enabled atomic.Bool

func init() {
	enabled.Store(true)
}

func SetSilent() {
	enabled.Store(false)
}

func Printf(format string, a ...any) {
	if !enabled.Load() {
		return
	}

	for i, v := range a {
		switch r := v.(type) {
		case time.Time:
			a[i] = time.Since(r)
		case image.Image:
			b := r.Bounds()

			a[i] = fmt.Sprintf("%dx%dx", b.Dx(), b.Dy())
		default:
			a[i] = v
		}
	}

	fmt.Fprintf(os.Stderr, format, a...)
}

func PrintKV(codec, key string, val any) {
	if !enabled.Load() {
		return
	}

	Printf("%s: %s=%v\n", codec, key, val)
}

func Errorf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f, a...)

	os.Exit(1)
}
