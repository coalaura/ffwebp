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

func Sniff(reader io.Reader, input string, ignoreExtension bool) (*Sniffed, io.Reader, error) {
	if !ignoreExtension {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(input), "."))
		if ext != "" {
			codec, _ := FindCodec(ext)
			if codec != nil {
				return &Sniffed{
					Header:     []byte("." + ext),
					Confidence: 100,
					Codec:      codec,
				}, reader, nil
			}
		}
	}

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
		return nil, nil, errors.New("unknown input format")
	}

	return &Sniffed{
		Header:     magic,
		Confidence: best,
		Codec:      guess,
	}, bytes.NewReader(buf), nil
}

func Detect(output, override string) (Codec, string, error) {
	ext := override

	if ext == "" {
		ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(output), "."))
		if ext == "" {
			return nil, "", fmt.Errorf("output filename %q has no extension", output)
		}
	}

	codec, err := FindCodec(ext)
	if err != nil {
		return nil, "", err
	}

	if codec == nil {
		return nil, "", fmt.Errorf("unsupported output codec: %q", ext)
	}

	return codec, ext, nil
}

func FindCodec(ext string) (Codec, error) {
	codec, ok := codecs[ext]
	if ok {
		return codec, nil
	}

	for _, codec := range codecs {
		for _, alias := range codec.Extensions() {
			if ext == alias {
				if !codec.CanEncode() {
					return nil, fmt.Errorf("decode-only output codec: %q", ext)
				}

				return codec, nil
			}
		}
	}

	return nil, nil
}
