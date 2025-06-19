package main

import (
	"errors"
	"image"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/urfave/cli/v3"
)

func resize(img image.Image, cmd *cli.Command) (image.Image, error) {
	options := strings.ToLower(cmd.String("resize"))

	index := strings.Index(options, "x")
	if index == -1 {
		return img, nil
	}

	var (
		width  int
		height int
	)

	wRaw := options[:index]
	if wRaw != "" {
		w64, err := strconv.ParseInt(wRaw, 10, 64)
		if err != nil {
			return nil, err
		}

		width = int(max(0, w64))
	}

	hRaw := options[index:]
	if hRaw != "" {
		h64, err := strconv.ParseInt(hRaw, 10, 64)
		if err != nil {
			return nil, err
		}

		height = int(max(0, h64))
	}

	if width == 0 && height == 0 {
		return nil, errors.New("at least one size needs to be specified for resizing")
	}

	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	return resized, nil
}
