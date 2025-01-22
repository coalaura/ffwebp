package main

import (
	"fmt"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"github.com/coalaura/arguments"
	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
	"golang.org/x/image/tiff"
)

type Option struct {
	Format string
	Name   string
	Value  interface{}
}

type OptionsTable struct {
	max     int
	entries []Option
	seen    map[string]bool
}

func NewOptionsTable() *OptionsTable {
	table := &OptionsTable{
		entries: make([]Option, 0),
		seen:    make(map[string]bool),
	}

	table.Add("format", "%s", opts.Format)

	if opts.NumColors != 0 {
		table.Add("colors", "%d", opts.NumColors)
	}

	return table
}

func (t *OptionsTable) Add(name, format string, value interface{}) {
	if t.seen[name] {
		return
	}

	t.seen[name] = true

	t.entries = append(t.entries, Option{
		Format: format,
		Name:   name,
		Value:  value,
	})

	if len(name) > t.max {
		t.max = len(name)
	}
}

func (t *OptionsTable) Print() {
	if opts.Silent {
		return
	}

	b := arguments.NewBuilder(true)

	b.Name()
	b.WriteString("Options:")

	for _, opt := range t.entries {
		b.WriteRune('\n')
		b.Mute()
		b.WriteString(" - ")
		b.Name()
		b.WriteString(opt.Name)
		b.WriteString(strings.Repeat(" ", t.max-len(opt.Name)))
		b.Mute()
		b.WriteString(": ")
		b.Value()
		b.WriteString(fmt.Sprintf(opt.Format, opt.Value))
	}

	println(b.String())
}

func (t *OptionsTable) AddWebPOptions(options webp.Options) {
	t.Add("lossless", "%v", options.Lossless)
	t.Add("quality", "%v", options.Quality)
	t.Add("method", "%v", options.Method)
	t.Add("exact", "%v", options.Exact)

	t.Print()
}

func (t *OptionsTable) AddJpegOptions(options *jpeg.Options) {
	t.Add("quality", "%v", options.Quality)

	t.Print()
}

func (t *OptionsTable) AddPNGOptions(encoder *png.Encoder) {
	t.Add("level", "%s", PNGCompressionLevelToString(encoder.CompressionLevel))

	t.Print()
}

func (t *OptionsTable) AddGifOptions(options *gif.Options) {
	t.Add("colors", "%v", options.NumColors)

	t.Print()
}

func (t *OptionsTable) AddTiffOptions(options *tiff.Options) {
	t.Add("compression", "%s", TiffCompressionTypeToString(options.Compression))

	t.Print()
}

func (t *OptionsTable) AddAvifOptions(options avif.Options) {
	t.Add("quality", "%v", options.Quality)
	t.Add("speed", "%v", options.Speed)
	t.Add("ratios", "%s", options.ChromaSubsampling.String())

	t.Print()
}

func (t *OptionsTable) AddJxlOptions(options jpegxl.Options) {
	t.Add("quality", "%v", options.Quality)
	t.Add("effort", "%v", options.Effort)

	t.Print()
}
