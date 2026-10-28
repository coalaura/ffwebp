package ico

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"io"

	"github.com/sergeymakinen/go-ico"
	"github.com/sergeymakinen/go-ico/cur"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "ico"
}

func (impl) Extensions() []string {
	return []string{"ico", "cur"}
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magicICO := []byte{0x00, 0x00, 0x01, 0x00}
	magicCUR := []byte{0x00, 0x00, 0x02, 0x00}

	buf := make([]byte, 22)

	_, err := reader.ReadAt(buf, 0)
	if errors.Is(err, io.EOF) {
		return 0, nil, nil
	}

	if err != nil {
		return 0, nil, err
	}

	if !bytes.Equal(buf[:4], magicICO) && !bytes.Equal(buf[:4], magicCUR) {
		return 0, nil, nil
	}

	// Uncompressed TGA shares the CUR prefix; require a plausible directory entry.
	count := binary.LittleEndian.Uint16(buf[4:6])
	size := binary.LittleEndian.Uint32(buf[14:18])
	offset := binary.LittleEndian.Uint32(buf[18:22])

	if count == 0 || buf[9] != 0 || size == 0 || offset < 6+16*uint32(count) {
		return 0, nil, nil
	}

	return 100, buf, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(data) < 6 {
		return nil, io.ErrUnexpectedEOF
	}

	preferAlpha(data)

	reader := bytes.NewReader(data)

	if binary.LittleEndian.Uint32(data[:4]) == 0x00020000 {
		return cur.Decode(reader)
	}

	images, err := ico.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	var (
		best     image.Image
		bestArea int
	)

	for _, img := range images {
		bounds := img.Bounds()
		area := bounds.Dx() * bounds.Dy()

		if area > bestArea {
			best = img
			bestArea = area
		}
	}

	if best == nil {
		return nil, io.EOF
	}

	return best, nil
}

func (impl) Encode(w io.Writer, img image.Image, _ opts.Common) error {
	return ico.Encode(w, img)
}
