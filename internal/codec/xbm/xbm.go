package xbm

import (
	"bytes"
	"image"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"

	decode "github.com/knieriem/g/image/xbm"
	"github.com/xyproto/xbm"
)

func init() {
	codec.Register(impl{})
}

type impl struct{}

func (impl) String() string {
	return "xbm"
}

func (impl) Extensions() []string {
	return []string{"xbm"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return flags
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 64)

	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return 0, nil, err
	}

	buf = buf[:n]

	if bytes.Contains(buf, []byte("#define")) && bytes.Contains(buf, []byte("static char")) {
		return 90, buf, nil
	}

	return 0, nil, nil
}

func (impl) Decode(r io.Reader) (image.Image, error) {
	return decode.Decode(r)
}

func (impl) Encode(w io.Writer, img image.Image, options opts.Common) error {
	return xbm.Encode(w, img)
}
