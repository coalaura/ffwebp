package main

import (
	"os"
)

func main() {
	parse()

	info("Reading input image...")

	in, err := os.OpenFile(opts.Input, os.O_RDONLY, 0)
	if err != nil {
		fatalf("Failed to open input file: %s", err)
	}

	defer in.Close()

	var out *os.File

	if opts.Output == "" {
		opts.Silent = true

		out = os.Stdout
	} else {
		out, err = os.OpenFile(opts.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			fatalf("Failed to open output file: %s", err)
		}

		defer out.Close()
	}

	info("Decoding input image...")

	img, err := ReadImage(in)
	if err != nil {
		fatalf("Failed to read image: %s", err)
	}

	info("Using output format: %s", opts.Format)

	// Write image
	info("Encoding output image...")

	err = WriteImage(out, img, opts.Format)
	if err != nil {
		fatalf("Failed to write image: %s", err)
	}
}
