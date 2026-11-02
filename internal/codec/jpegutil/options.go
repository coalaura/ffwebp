package jpegutil

import (
	"fmt"
	"image"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/coalaura/jpeg/detect"
	"github.com/urfave/cli/v3"
)

func Enum(name, value, choices, usage string) cli.Flag {
	return &cli.StringFlag{
		Name:  name,
		Value: value,
		Usage: usage + " (" + choices + ")",
		Validator: func(value string) error {
			for choice := range strings.FieldsSeq(choices) {
				if value == choice {
					return nil
				}
			}

			return fmt.Errorf("--%s: expected %s, got %q", name, choices, value)
		},
	}
}

func OptionalBool(command *cli.Command, name string) *bool {
	if !command.IsSet(name) {
		return nil
	}

	return new(command.Bool(name))
}

func OptionalFloat(command *cli.Command, name string) *float32 {
	if !command.IsSet(name) {
		return nil
	}

	return new(float32(command.Float64(name)))
}

func Integers(value string, count int) ([]int, error) {
	values := make([]int, 0, count)

	for part := range strings.SplitSeq(value, ",") {
		number, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("expected %d comma-separated integers: %q", count, value)
		}

		values = append(values, number)
	}

	if len(values) != count {
		return nil, fmt.Errorf("expected %d comma-separated integers: %q", count, value)
	}

	return values, nil
}

func Floats(value string) ([]float64, error) {
	if value == "" {
		return nil, nil
	}

	values := make([]float64, 0, strings.Count(value, ",")+1)

	for part := range strings.SplitSeq(value, ",") {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid layer target %q: %w", part, err)
		}

		values = append(values, number)
	}

	return values, nil
}

func Precision(img image.Image) int {
	switch img.(type) {
	case *image.Gray16, *image.RGBA64, *image.NRGBA64, *image.Alpha16:
		return 16
	default:
		return 8
	}
}

func Sniff(reader io.ReaderAt, formats ...detect.Format) (int, []byte, error) {
	format, _, err := detect.Detect(io.NewSectionReader(reader, 0, detect.DefaultHeaderLimit))
	if err != nil {
		return 0, nil, nil
	}

	if slices.Contains(formats, format) {
		return 100, []byte(format.String()), nil
	}

	return 0, nil, nil
}

func Limits(flags []cli.Flag, prefix string) []cli.Flag {
	return append(flags,
		&cli.Int64Flag{Name: prefix + ".max-input-bytes", Usage: "Input byte limit: 0=64 MiB, maximum 512 MiB"},
		&cli.Uint64Flag{Name: prefix + ".max-pixels", Usage: "Decoded pixel limit: 0=16 million; native buffers also have library limits"},
	)
}
