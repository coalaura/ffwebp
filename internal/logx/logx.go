package logx

import (
	"fmt"
	"image"
	"io"
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

func Fprintf(writer io.Writer, format string, a ...any) {
	if !enabled.Load() {
		return
	}

	if writer == nil {
		writer = os.Stderr
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

	fmt.Fprintf(writer, format, a...)
}

func Printf(format string, a ...any) {
	Fprintf(os.Stderr, format, a...)
}

func Print(message string) {
	if !enabled.Load() {
		return
	}

	fmt.Fprint(os.Stderr, message)
}

func Errorf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f, a...)

	os.Exit(1)
}
