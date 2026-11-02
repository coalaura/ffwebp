package jpegxl

import (
	"bytes"
	"fmt"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/opts"
	jxl "github.com/coalaura/jpeg/jpegxl"
)

func (impl) Transcode(reader io.Reader, input, output codec.Codec) ([]byte, bool, error) {
	recompress := command.Bool("jpegxl.recompress-jpeg")
	restore := command.Bool("jpegxl.restore-jpeg")

	if !recompress && !restore {
		return nil, false, nil
	}

	var result bytes.Buffer

	if recompress {
		if input.String() != "jpeg" || output.String() != "jpegxl" {
			return nil, true, fmt.Errorf("--jpegxl.recompress-jpeg requires JPEG input and JPEG XL output")
		}

		settings, err := encodeOptions(opts.Common{Quality: 85})
		if err != nil {
			return nil, true, err
		}

		err = jxl.RecompressJPEG(&result, reader, &settings)
		return result.Bytes(), true, err
	}

	if input.String() != "jpegxl" || output.String() != "jpeg" {
		return nil, true, fmt.Errorf("--jpegxl.restore-jpeg requires JPEG XL input and JPEG output")
	}

	settings := decodeOptions()

	err := jxl.RestoreJPEG(&result, reader, &settings)
	if err != nil {
		return nil, true, fmt.Errorf("restore original JPEG: %w; input must contain JPEG reconstruction data from recompression", err)
	}

	return result.Bytes(), true, nil
}

func validateTranscode() error {
	recompress := command.Bool("jpegxl.recompress-jpeg")
	restore := command.Bool("jpegxl.restore-jpeg")

	if !recompress && !restore {
		return nil
	}

	if recompress && restore {
		return fmt.Errorf("jpegxl: recompress-jpeg and restore-jpeg are mutually exclusive")
	}

	if recompress && !command.Bool("jpegxl.container") {
		return fmt.Errorf("jpegxl: JPEG reconstruction requires a container; remove --jpegxl.container=false")
	}

	for _, flag := range command.Flags {
		name := flag.Names()[0]
		if !command.IsSet(name) {
			continue
		}

		switch name {
		case "input", "output", "codec", "input.codec", "sniff", "silent", "threads", "start-number", "skip-existing", "jpegxl.recompress-jpeg", "jpegxl.restore-jpeg", "jpegxl.threads", "jpegxl.max-memory-bytes":
			continue
		case "jpegxl.effort", "jpegxl.decoding-speed", "jpegxl.codestream-level", "jpegxl.setting", "jpegxl.container":
			if recompress {
				continue
			}
		case "jpegxl.max-input-bytes", "jpegxl.max-pixels", "jpegxl.max-decoded-bytes", "jpegxl.max-frames":
			if restore {
				continue
			}
		}

		return fmt.Errorf("jpegxl: --%s is incompatible with byte-oriented JPEG reconstruction; remove it or use ordinary image conversion", name)
	}

	return nil
}
