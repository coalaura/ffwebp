package codec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
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

func Sniff(reader io.Reader, input, force string, ignoreExtension bool) (*Sniffed, io.Reader, error) {
	if force != "" {
		codec, err := FindCodec(strings.ToLower(force), false)
		if err != nil {
			return nil, nil, err
		}

		return &Sniffed{
			Header:     []byte(force),
			Confidence: 100,
			Codec:      codec,
		}, reader, nil
	}

	var (
		hintedExt   string
		hintedCodec Codec
	)

	if !ignoreExtension {
		hintedExt = strings.ToLower(strings.TrimPrefix(filepath.Ext(input), "."))

		if hintedExt != "" {
			hintedCodec, _ = FindCodec(hintedExt, false)
		}
	}

	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	ra := bytes.NewReader(buf)

	type candidate struct {
		codec      Codec
		confidence int
		header     []byte
	}

	var (
		best int
		list []candidate
	)

	for _, codec := range codecs {
		ra.Seek(0, 0)

		confidence, header, err := codec.Sniff(ra)
		if err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}

			return nil, nil, err
		}

		fmt.Println(codec.String(), confidence)

		if confidence <= 0 {
			continue
		}

		list = append(list, candidate{
			codec:      codec,
			confidence: confidence,
			header:     header,
		})

		if confidence > best {
			best = confidence
		}
	}

	if len(list) == 0 || best <= 0 {
		return nil, nil, errors.New("unknown input format")
	}

	var top []candidate

	for _, cand := range list {
		if cand.confidence == best {
			top = append(top, cand)
		}
	}

	if hintedCodec != nil {
		for _, cand := range top {
			if cand.codec != hintedCodec {
				continue
			}

			return &Sniffed{
				Header:     cand.header,
				Confidence: cand.confidence,
				Codec:      cand.codec,
			}, bytes.NewReader(buf), nil
		}
	}

	sort.Slice(top, func(i, j int) bool {
		return top[i].codec.String() < top[j].codec.String()
	})

	chosen := top[0]

	return &Sniffed{
		Header:     chosen.header,
		Confidence: chosen.confidence,
		Codec:      chosen.codec,
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

	codec, err := FindCodec(ext, true)
	if err != nil {
		return nil, "", err
	}

	if codec == nil {
		return nil, "", fmt.Errorf("unsupported output codec: %q", ext)
	}

	return codec, ext, nil
}

func FindCodec(ext string, requireEncode bool) (Codec, error) {
	codec, ok := codecs[ext]
	if ok {
		return codec, nil
	}

	for _, codec := range codecs {
		for _, alias := range codec.Extensions() {
			if ext == alias {
				if requireEncode && !codec.CanEncode() {
					return nil, fmt.Errorf("decode-only output codec: %q", ext)
				}

				return codec, nil
			}
		}
	}

	return nil, nil
}
