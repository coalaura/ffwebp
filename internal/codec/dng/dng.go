package dng

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"

	// pure-go DNG preview extractor
	"github.com/mdouchement/dng"
)

const tagDNGVersion = 0xC612

type impl struct{}

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "dng"
}

func (impl) Extensions() []string {
	return []string{"dng"}
}

func (impl) CanEncode() bool {
	return false
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	header := make([]byte, 8)

	count, err := reader.ReadAt(header, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, nil, err
	}

	if count < len(header) {
		return 0, nil, nil
	}

	isLE := bytes.Equal(header[0:2], []byte{'I', 'I'})
	isBE := bytes.Equal(header[0:2], []byte{'M', 'M'})

	if !isLE && !isBE {
		return 0, nil, nil
	}

	var ord binary.ByteOrder

	if isLE {
		ord = binary.LittleEndian
	} else {
		ord = binary.BigEndian
	}

	if ord.Uint16(header[2:4]) != 42 {
		return 0, nil, nil
	}

	ifdOffset := ord.Uint32(header[4:8])
	if ifdOffset < 8 {
		return 0, nil, nil
	}

	entries, err := readU16(reader, ord, int64(ifdOffset))
	if err != nil {
		return 0, nil, nil
	}

	for index := range int(entries) {
		offset := int64(ifdOffset) + 2 + int64(index)*12

		tag, err := readU16(reader, ord, offset)
		if err != nil {
			return 0, nil, nil
		}

		if tag == tagDNGVersion {
			return 110, header, nil
		}
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	logx.Printf("dng: extracting embedded JPEG preview\n")

	img, err := dng.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("dng: embedded JPEG preview: %w", err)
	}

	return img, nil
}

func (impl) Encode(w io.Writer, img image.Image, _ opts.Common) error {
	return errors.New("dng: encode not supported")
}

func readU16(reader io.ReaderAt, ord binary.ByteOrder, off int64) (uint16, error) {
	var buffer [2]byte

	_, err := reader.ReadAt(buffer[:], off)
	if err != nil {
		return 0, err
	}

	return ord.Uint16(buffer[:]), nil
}
