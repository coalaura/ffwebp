package dng

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"

	// pure-go DNG preview extractor
	"github.com/mdouchement/dng"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

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
	header := make([]byte, 16)
	if _, err := reader.ReadAt(header, 0); err != nil && err != io.EOF {
		return 0, nil, err
	}

	if len(header) < 8 {
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

	sig := ord.Uint16(header[2:4])

	const (
		tiffClassic   = 42
		tiffBig       = 43
		tagDNGVersion = 0xC612
	)

	switch sig {
	case tiffClassic:
		ifd0, err := readU32(reader, ord, 4)
		if err != nil || ifd0 == 0 {
			return 0, nil, nil
		}

		n, err := readU16(reader, ord, int64(ifd0))
		if err != nil {
			return 0, nil, nil
		}

		for i := 0; i < int(n); i++ {
			off := int64(ifd0) + 2 + int64(i)*12

			tag, err := readU16(reader, ord, off)
			if err != nil {
				return 0, nil, nil
			}

			if uint16(tag) == uint16(tagDNGVersion) {
				return 110, header[:8], nil
			}
		}
	case tiffBig:
		ifd0, err := readU64(reader, ord, 8)
		if err != nil || ifd0 == 0 {
			return 0, nil, nil
		}

		var bcnt [8]byte

		if _, err := reader.ReadAt(bcnt[:], int64(ifd0)); err != nil {
			return 0, nil, nil
		}

		n := ord.Uint64(bcnt[:])
		max := n

		if max > 1024 {
			max = 1024
		}

		for i := uint64(0); i < max; i++ {
			off := int64(ifd0) + 8 + int64(i)*20

			tag, err := readU16(reader, ord, off)
			if err != nil {
				return 0, nil, nil
			}

			if uint16(tag) == uint16(tagDNGVersion) {
				return 110, header[:8], nil
			}
		}
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	return dng.Decode(r)
}

func (impl) Encode(w io.Writer, img image.Image, _ opts.Common) error {
	return errors.New("dng: encode not supported")
}

func readU16(reader io.ReaderAt, ord binary.ByteOrder, off int64) (uint16, error) {
	var b [2]byte

	if _, err := reader.ReadAt(b[:], off); err != nil {
		return 0, err
	}

	return ord.Uint16(b[:]), nil
}

func readU32(reader io.ReaderAt, ord binary.ByteOrder, off int64) (uint32, error) {
	var b [4]byte

	if _, err := reader.ReadAt(b[:], off); err != nil {
		return 0, err
	}

	return ord.Uint32(b[:]), nil
}

func readU64(reader io.ReaderAt, ord binary.ByteOrder, off int64) (uint64, error) {
	var b [8]byte

	if _, err := reader.ReadAt(b[:], off); err != nil {
		return 0, err
	}

	return ord.Uint64(b[:]), nil
}
