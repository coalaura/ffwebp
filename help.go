package main

import (
	"fmt"
	"os"

	"github.com/coalaura/arguments"
)

func help() {
	if !opts.Help {
		return
	}

	println("  __  __              _")
	println(" / _|/ _|            | |")
	println("| |_| |___      _____| |__  _ __")
	println("|  _|  _\\ \\ /\\ / / _ \\ '_ \\| '_ \\")
	println("| | | |  \\ V  V /  __/ |_) | |_) |")
	println("|_| |_|   \\_/\\_/ \\___|_.__/| .__/")
	println("                           | |")
	fmt.Printf("                    %s |_|\n", Version)

	println("\nffwebp [options] <input> [output]\n")

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
