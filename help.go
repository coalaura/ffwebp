package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func help() {
	if !arguments.GetBool("h", "help", false) {
		return
	}

	info("  __  __              _")
	info(" / _|/ _|            | |")
	info("| |_| |___      _____| |__  _ __")
	info("|  _|  _\\ \\ /\\ / / _ \\ '_ \\| '_ \\")
	info("| | | |  \\ V  V /  __/ |_) | |_) |")
	info("|_| |_|   \\_/\\_/ \\___|_.__/| .__/")
	info("                           | |")
	info("                    %s |_|", Version)

	info("\nffwebp -i <input> [output] [options]")

	var max int

	for name := range options {
		if len(name) > max {
			max = len(name)
		}
	}

	var formatted []string

	for name, help := range options {
		formatted = append(formatted, fmt.Sprintf(" - %-*s: %s", max, name, help))
	}

	sort.Strings(formatted)

	info(strings.Join(formatted, "\n"))

	info("\nInput formats:  %s", strings.Join(InputFormats, ", "))
	info("Output formats: %s", strings.Join(OutputFormats, ", "))

	os.Exit(0)
}
