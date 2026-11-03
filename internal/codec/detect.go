package codec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coalaura/jpeg/detect"
)

// Covers the classic encoder's 4 MiB metadata budget plus normal JPEG headers.
const jpegSniffLimit = 5 << 20

type Sniffed struct {
	Header     []byte
	Confidence int
	Codec      Codec
}

type candidate struct {
	codec      Codec
	confidence int
	header     []byte
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

		if codec == nil {
			return nil, nil, fmt.Errorf("unsupported input codec: %q", force)
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

	// Budgeted formats must reach their loaders before the whole-input sniffer.
	var prefix [4]byte

	count, err := io.ReadFull(reader, prefix[:])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, nil, err
	}

	reader = io.MultiReader(bytes.NewReader(prefix[:count]), reader)

	if count == 4 && string(prefix[:]) == "DDS " {
		dds, ok := Get("dds")
		if ok {
			return &Sniffed{Header: prefix[:], Confidence: 100, Codec: dds}, reader, nil
		}
	}

	limit := detect.DefaultHeaderLimit

	if count >= 2 && prefix[0] == 0xff && prefix[1] == 0xd8 {
		limit = jpegHeaderLimit()
	}

	format, replay, detectionErr := detect.DetectLimit(reader, limit)
	reader = replay

	if detectionErr == nil {
		name := ""

		switch format {
		case detect.JPEG:
			name = "jpeg"
		case detect.JPEGLS:
			name = "jpegls"
		case detect.JP2, detect.J2K:
			name = "jpeg2000"
		case detect.JXLCodestream, detect.JXLContainer:
			name = "jpegxl"
		}

		registered, ok := Get(name)
		if !ok {
			return nil, nil, fmt.Errorf("input %s requires the %s codec build tag", format, name)
		}

		return &Sniffed{Header: []byte(format.String()), Confidence: 100, Codec: registered}, reader, nil
	}

	if errors.Is(detectionErr, detect.ErrHeaderLimit) || errors.Is(detectionErr, detect.ErrMalformed) {
		return nil, nil, fmt.Errorf("JPEG-family detection: %w; use --input.codec for a known format within its input limit", detectionErr)
	}

	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	ra := bytes.NewReader(buf)

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
	ext := strings.ToLower(override)

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
		if requireEncode && !codec.CanEncode() {
			return nil, fmt.Errorf("decode-only output codec: %q", ext)
		}

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

func jpegHeaderLimit() int {
	limit := int64(0)
	names := []string{"jpeg", "jpegls"}

	for _, name := range names {
		if bounded, ok := codecs[name].(InputLimiter); ok {
			budget := bounded.InputLimit()
			if budget <= 0 {
				budget = jpegSniffLimit
			}

			limit = max(limit, budget)
		}
	}

	if limit == 0 {
		return jpegSniffLimit
	}

	// The SOI marker is shared: neither codec's smaller budget may exclude
	// valid input for the other before its frame marker identifies it.
	return int(min(limit, jpegSniffLimit))
}
