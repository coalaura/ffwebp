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

func decodeInput(input string, output codec.Codec, cmd *cli.Command, logger io.Writer) (img image.Image, animation *codec.Animation, native func(io.Writer) error, err error) {
	var reader io.Reader = os.Stdin

	if input != "-" {
		logx.Fprintf(logger, "opening input file %q\n", filepath.ToSlash(input))

		file, openErr := os.Open(input)
		if openErr != nil {
			return nil, nil, nil, openErr
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
		return nil, nil, nil, err
	}

	logx.Fprintf(logger, "sniffed codec: %s (%q, forced=%v)\n", sniffed.Codec, sniffed, forced != "")

	for _, registered := range codec.All() {
		if processor, ok := registered.(codec.InputProcessor); ok {
			err := processor.ValidateRoute(sniffed.Codec, output)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	for _, registered := range codec.All() {
		if transcoder, ok := registered.(codec.ByteTranscoder); ok {
			data, handled, err := transcoder.Transcode(reader, sniffed.Codec, output)
			if err != nil {
				return nil, nil, nil, err
			}

			if handled {
				return nil, nil, func(writer io.Writer) error {
					_, err := writer.Write(data)
					return err
				}, nil
			}
		}
	}

	if processor, ok := sniffed.Codec.(codec.InputProcessor); ok {
		img, native, err = processor.DecodeInput(reader, output, logger)
		return img, nil, native, err
	}

	if decoder, ok := sniffed.Codec.(codec.AnimatedDecoder); ok {
		animation, err := decoder.DecodeAll(reader)
		if err != nil {
			return nil, nil, nil, err
		}

		if animation == nil || len(animation.Frames) == 0 {
			return nil, nil, nil, errors.New("decoded animation has no frames")
		}

		return nil, animation, nil, nil
	}

	img, err = sniffed.Codec.Decode(reader)
	if err != nil {
		return nil, nil, nil, err
	}

	if img == nil || img.Bounds().Empty() {
		return nil, nil, nil, errors.New("decoded image is empty")
	}

	return img, nil, nil, nil
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
