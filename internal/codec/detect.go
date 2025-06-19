package codec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type Sniffed struct {
	Header     []byte
	Confidence int
	Codec      Codec
}

func (s *Sniffed) String() string {
	var builder strings.Builder

	for _, b := range s.Header {
		if b >= 32 && b <= 126 {
			builder.WriteByte(b)
		} else {
			builder.WriteRune('.')
		}
	}

	return builder.String()
}

func Sniff(reader io.Reader) (*Sniffed, io.Reader, error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	ra := bytes.NewReader(buf)

	var (
		best  int
		magic []byte
		guess Codec
	)

	for _, codec := range codecs {
		confidence, header, err := codec.Sniff(ra)
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}

			return nil, nil, err
		}

		if confidence > best {
			best = confidence
			magic = header
			guess = codec
		}
	}

	if guess == nil {
		return nil, nil, errors.New("unknown format")
	}

	return &Sniffed{
		Header:     magic,
		Confidence: best,
		Codec:      guess,
	}, bytes.NewReader(buf), nil
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
