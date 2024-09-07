package main

import (
	"os"
)

func main() {
	help()

	silent = arguments.GetBool("s", "silent", false)

	// Read input file
	input := arguments.GetString("i", "input")

	var in *os.File

	if input == "" {
		in = os.Stdin
	} else {
		var err error

		in, err = os.OpenFile(input, os.O_RDONLY, 0)
		if err != nil {
			fatalf(1, "Failed to open input file: %s", err)
		}
	}

	// Read image
	if in == os.Stdin {
		info("Decoding input from stdin...")
	} else {
		info("Decoding input image...")
	}

	img, err := ReadImage(in)
	if err != nil {
		fatalf(4, "Failed to read image: %s", err)
	}

	// Read output format
	format := arguments.GetString("f", "format")

	// Read output file
	output := arguments.GetString("", "")

	var out *os.File

	if output == "" {
		if format == "" {
			format = "webp"
		}

		out = os.Stdout
		silent = true
	} else {
		var err error

		if format == "" {
			format = OutputFormatFromPath(output)
		}

		out, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			fatalf(2, "Failed to open output file: %s", err)
		}
	}

	if !IsValidOutputFormat(format) {
		fatalf(3, "Invalid output format: %s", format)
	}

	info("Using output format: %s", format)

	// Write image
	info("Encoding output image...")

	err = WriteImage(out, img, format)
	if err != nil {
		fatalf(5, "Failed to write image: %s", err)
	}
}
