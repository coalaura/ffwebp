package tga

import (
    "encoding/binary"
    "image"
    "io"

    "github.com/ftrvxmtrx/tga"

    "github.com/coalaura/ffwebp/internal/codec"
    "github.com/coalaura/ffwebp/internal/opts"
    "github.com/urfave/cli/v3"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "tga"
}

func (impl) Extensions() []string {
	return []string{"tga"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
    // Validate full 18-byte TGA header to reduce false positives.
    // Ref: https://www.fileformat.info/format/tga/egff.htm
    hdr := make([]byte, 18)
    if _, err := reader.ReadAt(hdr, 0); err != nil && err != io.EOF {
        return 0, nil, err
    }
    if len(hdr) < 18 {
        return 0, nil, nil
    }

    idLength := hdr[0]
    colorMapType := hdr[1]
    imageType := hdr[2]

    if colorMapType > 1 {
        return 0, nil, nil
    }

    switch imageType {
    case 1, 2, 3, 9, 10, 11:
        // valid image types
    default:
        // Exclude type 0 (no image data) to avoid matching random files like ISO BMFF.
        return 0, nil, nil
    }

    // Width/height must be > 0
    width := binary.LittleEndian.Uint16(hdr[12:14])
    height := binary.LittleEndian.Uint16(hdr[14:16])
    if width == 0 || height == 0 {
        return 0, nil, nil
    }

    // Pixel depth must be one of common values
    bpp := hdr[16]
    switch bpp {
    case 8, 15, 16, 24, 32:
        // ok
    default:
        return 0, nil, nil
    }

    // If color map is present, validate that the length is non-zero
    if colorMapType == 1 {
        colorMapLength := binary.LittleEndian.Uint16(hdr[5:7])
        if colorMapLength == 0 {
            return 0, nil, nil
        }
    }

    // Basic sanity: idLength must not push us past file start (not strictly necessary for sniff)
    _ = idLength

    header := make([]byte, 18)
    copy(header, hdr)
    return 100, header, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return tga.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, _ opts.Common) error {
	return tga.Encode(writer, img)
}
