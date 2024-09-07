package main

import (
	"bytes"
	"image/png"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
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
	exe, err := filepath.Abs("bin/ffwebp.exe")
	if err != nil {
		log.Fatalf("Failed to get absolute path for ffwebp.exe: %v\n", err)
	}

	for _, file := range TestFiles {
		log.Printf("Testing file: %s\n", file)

		cmd := exec.Command(exe, "-i", file, "-f", "png", "-s")

		// Capture the output (which is expected to be a PNG image)
		var stdout bytes.Buffer

		cmd.Stdout = &stdout

		// Run the command
		err := cmd.Run()
		if err != nil {
			out := strings.TrimSpace(stdout.String())

			log.Println(" - FAILED")
			log.Fatalf("Test failed for file: %s (%s)\n", file, out)
		}

		// Decode the captured stdout output as a PNG image
		img, err := png.Decode(&stdout)
		if err != nil {
			log.Println(" - FAILED")
			log.Fatalf("Failed to decode PNG image: %v\n", err)
		}

		if img == nil {
			log.Println(" - FAILED")
			log.Fatalf("No image data returned for file: %s\n", file)
		}

		log.Println(" - PASSED")
	}
}
