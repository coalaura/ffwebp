package main

import (
	"os"
)

func main() {
	header()
	parse()

	encoder, err := ResolveImageEncoder()
	must(err)

	debug("Reading input image...")

	in, err := os.OpenFile(opts.Input, os.O_RDONLY, 0)
	must(err)

	defer in.Close()

	var out *os.File

	if opts.Output == "" {
		opts.Silent = true

		out = os.Stdout
	} else {
		out, err = os.OpenFile(opts.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		must(err)

		defer out.Close()
	}

	debug("Decoding input image...")

	img, err := ReadImage(in)
	must(err)

	// Write image
	debug("Encoding output image...")

	img = Quantize(img)

	err = encoder(out, img)
	must(err)

	debug("Completed.")
}
