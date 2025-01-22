package main

import (
	"bytes"
	"image/png"
	"log"
	"os"
	"testing"
)

var (
	TestFiles = []string{
		"test/image.avif",
		"test/image.bmp",
		"test/image.gif",
		"test/image.heic",
		"test/image.heif",
		"test/image.ico",
		"test/image.jpg",
		"test/image.png",
		"test/image.tif",
		"test/image.tiff",
		"test/image.webp",
		"test/image.jxl",
	}
)

func TestFFWebP(t *testing.T) {
	opts.Silent = true
	opts.Format = "png"

	encoder, _ := ResolveImageEncoder()

	for _, file := range TestFiles {
		log.Printf("Testing file: %s\n", file)

		in, err := os.OpenFile(file, os.O_RDONLY, 0)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", file, err)
		}

		defer in.Close()

		img, err := ReadImage(in)
		if err != nil {
			log.Fatalf("Failed to decode %s: %v", file, err)
		}

		before := img.Bounds()

		var result bytes.Buffer

		err = encoder(&result, img)
		if err != nil {
			log.Fatalf("Failed to encode png image: %v", err)
		}

		img, err = png.Decode(&result)
		if err != nil {
			log.Println(" - FAILED")
			log.Fatalf("Failed to decode PNG image: %v\n", err)
		}

		after := img.Bounds()

		if before.Max.X != after.Max.X || before.Max.Y != after.Max.Y {
			log.Println(" - FAILED")
			log.Fatalf("Invalid image (%dx%d != %dx%d) for file: %s\n", before.Max.X, before.Max.Y, after.Max.X, after.Max.Y, file)
		}

		log.Println(" - PASSED")
	}
}
