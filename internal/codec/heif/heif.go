package heif

import (
	"bytes"
	"errors"
	"image"
	"io"

	"github.com/gen2brain/h265/heic"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "heif"
}

func (impl) Extensions() []string {
	return []string{"heic", "heif"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 32)

	n, err := reader.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, nil, err
	}

	buf = buf[:n]

	if len(buf) < 12 {
		return 0, nil, nil
	}

	if !bytes.Equal(buf[4:8], []byte("ftyp")) {
		return 0, nil, nil
	}

	brands := [][]byte{
		[]byte("heic"),
		[]byte("heix"),
		[]byte("hevc"),
		[]byte("hevx"),
		[]byte("mif1"),
		[]byte("msf1"),
	}

	for _, b := range brands {
		if bytes.Contains(buf[8:], b) {
			return 90, buf[:n], nil
		}
	}

	return 0, nil, nil
}

func (impl) Decode(reader io.Reader) (image.Image, error) {
	return heic.Decode(reader)
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	settings := heic.EncodeOptions{
		Quality:  options.Quality,
		Lossless: options.Lossless,
		Chroma:   heic.Chroma420,
	}

	if options.Lossless {
		// PCM preserves samples, but subsampling would still discard color detail.
		settings.Chroma = heic.Chroma444
	}

	logx.Printf("heif: quality=%d lossless=%t\n", settings.Quality, settings.Lossless)

	return heic.Encode(writer, img, settings)
}
