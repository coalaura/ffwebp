package png

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/gen2brain/pngn"
	"github.com/urfave/cli/v3"
)

type impl struct{}

var compression int

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "png"
}

func (impl) Extensions() []string {
	return []string{"png", "apng"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags, &cli.IntFlag{
		Name:        "png.compression",
		Usage:       "PNG: compression level (0=none, 1=default, 2=speed, 3=best)",
		Value:       1,
		Destination: &compression,
		Validator: func(value int) error {
			if value < 0 || value > 3 {
				return fmt.Errorf("invalid png.compression: %d", value)
			}

			return nil
		},
	})
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	magic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	buf := make([]byte, len(magic))

	_, err := reader.ReadAt(buf, 0)
	if err != nil {
		return 0, nil, err
	}

	if bytes.Equal(buf, magic) {
		return 100, magic, nil
	}

	return 0, nil, nil
}

func (adapter impl) Decode(reader io.Reader) (image.Image, error) {
	animation, err := adapter.DecodeAll(reader)
	if err != nil {
		return nil, err
	}

	return &codec.ProfiledImage{Image: animation.Frames[0], ICC: animation.ICC, Channels: animation.Channels}, nil
}

func (impl) DecodeAll(reader io.Reader) (*codec.Animation, error) {
	var prefix bytes.Buffer

	animated, err := pngn.Animated(io.TeeReader(reader, &prefix))
	if err != nil {
		return nil, err
	}

	// Replay the consumed header even for stdin; collect metadata with the sole pixel decode.
	metadata := new(pngn.Metadata)

	animation, err := pngn.DecodeAll(io.MultiReader(bytes.NewReader(prefix.Bytes()), reader), &pngn.Options{Metadata: metadata})
	if err != nil {
		return nil, err
	}

	channels := 4

	header := prefix.Bytes()
	if len(header) > 25 {
		switch header[25] {
		case 0:
			channels = 1
		case 4:
			channels = 2
		}
	}

	var profile []byte

	if metadata.ICCProfile != nil {
		profile = metadata.ICCProfile.Profile
	}

	if channels == 1 {
		for index, frame := range animation.Image {
			if opaque, ok := frame.(interface{ Opaque() bool }); ok && !opaque.Opaque() {
				channels = 2

				break
			}

			animation.Image[index], err = codec.GrayImage(frame, false)
			if err != nil {
				return nil, err
			}
		}
	}

	loops := animation.LoopCount

	// pngn reads/writes acTL.num_plays directly, not GIF's repeat count.
	if loops == 1 {
		loops = -1
	} else if loops > 1 {
		loops--
	}

	// DecodeAll already composites/disposes frames and excludes a separate default PNG image.
	return &codec.Animation{
		Frames:       animation.Image,
		DelaySeconds: animation.Delay,
		LoopCount:    loops,
		Animated:     animated,
		ICC:          profile,
		Channels:     channels,
	}, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	err := codec.ValidateICC(codec.ImageChannels(img), common.ICC)
	if err != nil {
		return err
	}

	logx.Printf("png: compression=%d\n", compression)

	settings := pngn.EncodeOptions{
		CompressionLevel: compressionLevel(compression),
		Metadata:         profileMetadata(common.ICC),
	}

	return pngn.Encode(writer, img, &settings)
}

func (adapter impl) EncodeAll(writer io.Writer, animation *codec.Animation, options opts.Common) error {
	if animation == nil || len(animation.Frames) == 0 {
		return fmt.Errorf("png: animation has no frames")
	}

	for index, frame := range animation.Frames {
		if frame == nil {
			return fmt.Errorf("png: frame %d is nil", index)
		}

		err := codec.ValidateICC(codec.ImageChannels(frame), options.ICC)
		if err != nil {
			return err
		}
	}

	if len(animation.Frames) == 1 && !animation.Animated {
		return adapter.Encode(writer, animation.Frames[0], options)
	}

	delays, err := animationDelays(animation)
	if err != nil {
		return err
	}

	loops := animation.LoopCount
	if loops < -1 || int64(loops) >= math.MaxUint32 {
		return fmt.Errorf("png: unsupported loop count %d", loops)
	}

	if loops == -1 {
		loops = 1
	} else if loops > 0 {
		loops++
	}

	settings := pngn.EncodeOptions{
		CompressionLevel: compressionLevel(compression),
		Metadata:         profileMetadata(options.ICC),
	}

	output := pngn.PNG{
		Image:     animation.Frames,
		Delay:     delays,
		LoopCount: loops,
	}

	logx.Printf("png: frames=%d compression=%d\n", len(output.Image), compression)

	return pngn.EncodeAll(writer, &output, &settings)
}

func profileMetadata(profile []byte) *pngn.Metadata {
	if len(profile) == 0 {
		return nil
	}

	return &pngn.Metadata{ICCProfile: &pngn.ICCProfile{Name: "ICC", Profile: profile}}
}

func compressionLevel(level int) pngn.CompressionLevel {
	switch level {
	case 0:
		return pngn.NoCompression
	case 1:
		return pngn.DefaultCompression
	case 2:
		return pngn.BestSpeed
	case 3:
		return pngn.BestCompression
	default:
		return pngn.DefaultCompression
	}
}

func animationDelays(animation *codec.Animation) ([]float64, error) {
	delays := make([]float64, len(animation.Frames))
	exact := true

	for index := range delays {
		delays[index] = animation.SecondsAt(index)
		exact = representableDelay(delays[index]) && exact
	}

	if exact {
		return delays, nil
	}

	// Quantize cumulative boundaries only when the encoder cannot retain the source fractions.
	milliseconds, err := animation.MillisecondDelays()
	if err != nil {
		return nil, err
	}

	for index, delay := range milliseconds {
		delays[index] = float64(delay) / 1000
		if !representableDelay(delays[index]) {
			return nil, fmt.Errorf("png: frame %d delay cannot be represented by the PNG encoder", index)
		}
	}

	return delays, nil
}

func representableDelay(seconds float64) bool {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return false
	}

	// pngn v0.1.3 tries these denominators, then rounds/clamps to 65535ms.
	denominators := [...]float64{1000, 100, 60, 30, 25, 24, 10, 1}

	for _, denominator := range denominators {
		numerator := seconds * denominator
		if numerator <= math.MaxUint16 && numerator == math.Trunc(numerator) {
			return true
		}
	}

	return false
}
