package webp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"math"

	"github.com/coalaura/webp"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var (
	method     int
	exact      bool
	autoFilter bool
)

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "webp"
}

func (impl) Extensions() []string {
	return []string{"webp"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.IntFlag{
			Name:        "webp.method",
			Usage:       "WebP: quality/speed trade-off (0=fast, 6=slower-better)",
			Value:       4,
			Destination: &method,
			Validator: func(v int) error {
				if v < 0 || v > 6 {
					return fmt.Errorf("invalid webp.method: %d (must be 0-6)", v)
				}

				return nil
			},
		},
		&cli.BoolFlag{
			Name:        "webp.exact",
			Usage:       "WebP: preserve exact RGB values in transparent areas",
			Value:       false,
			Destination: &exact,
		},
		&cli.BoolFlag{
			Name:        "webp.autofilter",
			Usage:       "WebP: enable auto-adjustment of filter strength",
			Value:       false,
			Destination: &autoFilter,
		},
	)
}

func (impl) Sniff(rd io.ReaderAt) (int, []byte, error) {
	buf := make([]byte, 12)

	_, err := rd.ReadAt(buf, 0)
	if err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf[0:4], []byte("RIFF")) && bytes.Equal(buf[8:12], []byte("WEBP")) {
		return 100, buf[:12], nil
	}

	return 0, nil, nil
}

func (adapter impl) Decode(reader io.Reader) (image.Image, error) {
	animation, err := adapter.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	return &codec.ProfiledImage{Image: animation.Frames[0], ICC: animation.ICC, Channels: 4}, nil
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	_, _, _, animated, _, err := webp.GetInfo(data)
	if err != nil {
		return nil, err
	}

	profile, err := webp.GetMetadata(data, "ICCP")
	if err != nil && err.Error() != "webp: metadata not found" {
		return nil, err
	}

	anim, err := webp.DecodeAll(bytes.NewReader(data), &webp.DecodeOptions{
		UseThreads: true,
	})

	if err != nil {
		return nil, err
	}

	if anim == nil {
		return nil, fmt.Errorf("webp: decoded animation is nil")
	}

	for index, frame := range anim.Image {
		anim.Image[index] = straightAlpha(frame)
	}

	loops := anim.LoopCount
	if loops == 1 {
		loops = -1
	} else if loops > 1 {
		loops--
	}

	return &codec.Animation{
		Frames:     anim.Image,
		Delays:     anim.Delay,
		LoopCount:  loops,
		Background: anim.Background,
		Animated:   animated,
		ICC:        profile,
		Channels:   4,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, options opts.Common) error {
	logx.Printf("webp: quality=%d lossless=%t method=%d exact=%t autofilter=%t\n", options.Quality, options.Lossless, method, exact, autoFilter)

	settings := &webp.Options{
		Quality:    float32(options.Quality),
		Lossless:   options.Lossless,
		AutoFilter: autoFilter,
		Method:     method,
		Exact:      exact,
	}

	if len(options.ICC) == 0 {
		return webp.Encode(writer, img, settings)
	}

	var buffer bytes.Buffer

	err := webp.Encode(&buffer, img, settings)
	if err != nil {
		return err
	}

	return writeProfile(writer, buffer.Bytes(), options.ICC)
}

func (impl) EncodeAll(writer io.Writer, anim *codec.Animation, options opts.Common) error {
	var frames int

	if anim != nil {
		frames = len(anim.Frames)
	}

	logx.Printf("webp: frames=%d quality=%d lossless=%t method=%d exact=%t autofilter=%t\n", frames, options.Quality, options.Lossless, method, exact, autoFilter)

	if frames == 0 {
		return fmt.Errorf("webp: animation has no frames")
	}

	if frames == 1 {
		return fmt.Errorf("webp: encoder collapses one-frame animations to stills; use APNG or JPEG XL to preserve playback")
	}

	loops := anim.LoopCount
	if loops < -1 || loops >= math.MaxUint16 {
		return fmt.Errorf("webp: unsupported loop count %d", loops)
	}

	if loops == -1 {
		loops = 1
	} else if loops > 0 {
		loops++
	}

	delays, err := anim.MillisecondDelays()
	if err != nil {
		return err
	}

	total := int64(0)

	for index, delay := range delays {
		if delay == 0 {
			return fmt.Errorf("webp: encoder cannot preserve zero-duration frame %d; use APNG or GIF", index)
		}

		total += int64(delay)
		if delay > 0xffffff || total > math.MaxInt32 {
			return fmt.Errorf("webp: frame %d exceeds the encoder's duration range", index)
		}
	}

	webpAnim := &webp.Animation{
		Image:      anim.Frames,
		Delay:      delays,
		LoopCount:  loops,
		Background: anim.Background,
	}

	var buffer bytes.Buffer

	err = webp.EncodeAll(&buffer, webpAnim, &webp.Options{
		Quality:    float32(options.Quality),
		Lossless:   options.Lossless,
		AutoFilter: autoFilter,
		Method:     method,
		Exact:      exact,
		UseThreads: true,
	})

	if err != nil {
		return err
	}

	// libwebp can merge identical frames or collapse the entire animation.
	// Check the muxed timeline before publishing any bytes, without decoding pixels again.
	err = validateTimeline(buffer.Bytes(), delays)
	if err != nil {
		return err
	}

	return writeProfile(writer, buffer.Bytes(), options.ICC)
}

func writeProfile(writer io.Writer, data, profile []byte) error {
	if len(profile) != 0 {
		err := codec.ValidateICC(4, profile)
		if err != nil {
			return err
		}

		data, err = webp.SetMetadata(data, profile, "ICCP")
		if err != nil {
			return err
		}
	}

	_, err := writer.Write(data)
	return err
}

func validateTimeline(data []byte, delays []int) error {
	frame := 0

	for offset := 12; offset+8 <= len(data); {
		size := int64(binary.LittleEndian.Uint32(data[offset+4:]))

		end := int64(offset) + 8 + size
		if end > int64(len(data)) {
			return fmt.Errorf("webp: invalid encoded chunk")
		}

		if string(data[offset:offset+4]) == "ANMF" {
			if size < 16 || frame >= len(delays) {
				return fmt.Errorf("webp: invalid encoded frame")
			}

			header := data[offset+8:]

			delay := int(header[12]) | int(header[13])<<8 | int(header[14])<<16
			if delay != delays[frame] {
				return fmt.Errorf("webp: encoder merged or retimed frames; use APNG or JPEG XL to preserve the timeline")
			}

			frame++
		}

		offset = int(end + size%2)
	}

	if frame != len(delays) {
		return fmt.Errorf("webp: encoder collapsed animation frames; use APNG or JPEG XL to preserve the timeline")
	}

	return nil
}

func straightAlpha(img image.Image) image.Image {
	if rgba, ok := img.(*image.RGBA); ok {
		// webp v1.6.6 wraps straight MODE_RGBA bytes as premultiplied RGBA.
		// Reuse the owned pixel buffer with the correct color model, without conversion.
		return &image.NRGBA{Pix: rgba.Pix, Stride: rgba.Stride, Rect: rgba.Rect}
	}

	return img
}
