package main

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/urfave/cli/v3"
)

func decodeInput(input string, cmd *cli.Command, logger io.Writer) (img image.Image, animation *codec.Animation, err error) {
	var reader io.Reader = os.Stdin

	if input != "-" {
		logx.Fprintf(logger, "opening input file %q\n", filepath.ToSlash(input))

		file, openErr := os.Open(input)
		if openErr != nil {
			return nil, nil, openErr
		}

		defer func() {
			closeErr := file.Close()

			if err == nil {
				err = closeErr
			}
		}()

		reader = file
	} else {
		logx.Fprintf(logger, "reading input from <stdin>\n")
	}

	forced := cmd.String("input.codec")

	sniffed, reader, err := codec.Sniff(reader, input, forced, cmd.Bool("sniff"))
	if err != nil {
		return nil, nil, err
	}

	logx.Fprintf(logger, "sniffed codec: %s (%q, forced=%v)\n", sniffed.Codec, sniffed, forced != "")

	if decoder, ok := sniffed.Codec.(codec.AnimatedDecoder); ok {
		animation, err := decoder.DecodeAll(reader)
		if err != nil {
			return nil, nil, err
		}

		if animation == nil || len(animation.Frames) == 0 {
			return nil, nil, errors.New("decoded animation has no frames")
		}

		return nil, animation, nil
	}

	img, err = sniffed.Codec.Decode(reader)
	if err != nil {
		return nil, nil, err
	}

	if img == nil || img.Bounds().Empty() {
		return nil, nil, errors.New("decoded image is empty")
	}

	return img, nil, nil
}

func selectAnimationFrame(animation *codec.Animation, cmd *cli.Command, logger io.Writer) (image.Image, error) {
	index := cmd.Int("frame")

	if index >= 0 {
		if index >= len(animation.Frames) {
			return nil, fmt.Errorf("frame index %d out of range (animation has %d frames)", index, len(animation.Frames))
		}
	} else {
		index = 0

		if cmd.IsSet("time") {
			target := cmd.Duration("time").Milliseconds()
			elapsed := int64(0)

			for frame, delay := range animation.Delays {
				if frame >= len(animation.Frames) {
					break
				}

				index = frame
				elapsed += int64(delay)

				if elapsed > target {
					break
				}
			}
		}
	}

	logx.Fprintf(logger, "selected frame %d from %d frame animation\n", index, len(animation.Frames))

	return animation.Frames[index], nil
}
