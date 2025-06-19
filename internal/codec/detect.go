package codec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func Sniff(reader io.Reader) (Codec, io.Reader, error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	ra := bytes.NewReader(buf)

	var (
		guess Codec
		best  int
	)

	for _, codec := range codecs {
		confidence, err := codec.Sniff(ra)
		if err != nil {
			return nil, nil, err
		}

		if confidence > best {
			best = confidence
			guess = codec
		}
	}

	if guess == nil {
		return nil, nil, errors.New("unknown format")
	}

	return guess, bytes.NewReader(buf), nil
}

func Detect(output, override string) (Codec, error) {
	if override != "" {
		codec, ok := codecs[override]
		if !ok {
			return nil, fmt.Errorf("unsupported output codec: %q", override)
		}

		return codec, nil
	}

	if output == "-" {
		return nil, errors.New("missing codec for output")
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(output), "."))
	if ext == "" {
		return nil, fmt.Errorf("output filename %q has no extension", output)
	}

	for _, codec := range codecs {
		for _, alias := range codec.Extensions() {
			if ext == strings.ToLower(alias) {
				return codec, nil
			}
		}
	}

	return nil, fmt.Errorf("unsupported or unknown file extension: %q", ext)
}
