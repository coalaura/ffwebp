package main

import (
	"os"

	"github.com/coalaura/arguments"
)

func header() {
	println("  __  __              _")
	println(" / _|/ _|_      _____| |__  _ __")
	println("| |_| |_\\ \\ /\\ / / _ \\ '_ \\| '_ \\")
	println("|  _|  _|\\ V  V /  __/ |_) | |_) |")
	println("|_| |_|   \\_/\\_/ \\___|_.__/| .__/")
	print("                           |_| ")
	println(Version)

	println()
}

func help() {
	if !opts.Help {
		return
	}

	println("ffwebp [options] <input> [output]\n")

	arguments.ShowHelp(true)

	b := arguments.NewBuilder(true)

	b.WriteRune('\n')
	b.Mute()
	b.WriteString(" - ")
	b.Name()
	b.WriteString("Input formats")
	b.Mute()
	b.WriteString(":  ")
	values(b, InputFormats)

	b.WriteRune('\n')
	b.Mute()
	b.WriteString(" - ")
	b.Name()
	b.WriteString("Output formats")
	b.Mute()
	b.WriteString(": ")
	values(b, OutputFormats)

	println(b.String())

	os.Exit(0)
}

func values(b *arguments.Builder, v []string) {
	for i, value := range v {
		if i > 0 {
			b.Mute()
			b.WriteString(", ")
		}

		b.Value()
		b.WriteString(value)
	}

	b.Reset()
}
