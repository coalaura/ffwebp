package dds

import (
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/coalaura/dds"
	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/coalaura/ffwebp/internal/logx"
	"github.com/coalaura/ffwebp/internal/opts"
	"github.com/urfave/cli/v3"
)

type impl struct{}

// Configure runs once before workers start; jobs only read these settings.
var config configuration

func init() {
	codec.Register(impl{})
}

func (impl) String() string {
	return "dds"
}

func (impl) Extensions() []string {
	return []string{"dds"}
}

func (impl) CanEncode() bool {
	return true
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	return append(flags, codecFlags...)
}

func (impl) Sniff(reader io.ReaderAt) (int, []byte, error) {
	var header [4]byte

	_, err := reader.ReadAt(header[:], 0)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, nil, nil
		}

		return 0, nil, err
	}

	if string(header[:]) == "DDS " {
		return 100, header[:], nil
	}

	return 0, nil, nil
}

func (adapter impl) Decode(reader io.Reader) (image.Image, error) {
	img, _, err := adapter.DecodeInput(reader, nil, nil)
	return img, err
}

func (impl) DecodeInput(reader io.Reader, output codec.Codec, logger io.Writer) (image.Image, func(io.Writer) error, error) {
	settings := &config

	texture, err := dds.Load(reader, &settings.decode.Load)
	if err != nil {
		return nil, nil, fmt.Errorf("dds load: %w", err)
	}

	native := settings.native(true, output != nil && output.String() == "dds")
	if native {
		texture, err = settings.processTexture(texture, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("dds texture: %w", err)
		}

		return nil, func(writer io.Writer) error {
			return settings.outputError(texture.Save(writer, &dds.SaveOptions{Header: settings.encode.Header}))
		}, nil
	}

	options := settings.decode
	metadata := texture.Metadata()

	if settings.cubeSelection {
		if !metadata.IsCube() {
			return nil, nil, fmt.Errorf("dds: cube/face selection requires a cube texture")
		}

		options.Surface = dds.CubeSelection(settings.cube, settings.face, options.Surface.Mip)
	}

	format := metadata.Format

	if options.Interpretation != dds.FormatUNKNOWN {
		format = options.Interpretation
	}

	alpha := metadata.AlphaMode()

	if options.AlphaInterpretation != dds.AlphaUnknown {
		alpha = options.AlphaInterpretation
	}

	if settings.explicit["premultiplication"] && (!format.Info().SRGB || alpha != dds.AlphaPremultiplied) {
		return nil, nil, fmt.Errorf("--dds.premultiplication requires premultiplied sRGB input")
	}

	if settings.explicit["input-alpha"] && !format.Info().Alpha {
		return nil, nil, fmt.Errorf("--dds.input-alpha requires an input alpha channel")
	}

	if settings.lossless {
		info := format.Info()
		linearAssociation := info.SRGB && alpha == dds.AlphaPremultiplied && options.Premultiplication == dds.SRGBPremultipliedLinear

		if (options.Precision == dds.Precision8 && info.BitsPerColor > 8) || info.DataType != dds.TypeUNorm || format == dds.FormatR10G10B10_XR_BIAS_A2_UNORM || options.Display == dds.DisplayClamp || options.Filter != 0 || linearAssociation {
			return nil, nil, fmt.Errorf("dds: --lossless prohibits precision/range reduction or filtered image extraction; use texture scope for native preservation")
		}
	}

	if texture.SubresourceCount() > 1 {
		logx.Fprintf(logger, "dds: selected mip=%d item=%d slice=%d from %d subresources (not animation frames)\n", options.Surface.Mip, options.Surface.Item, options.Surface.Slice, texture.SubresourceCount())
	}

	img, err := texture.Image(&options)
	if err != nil {
		return nil, nil, fmt.Errorf("dds image: %w (see --dds.interpret, --dds.display clamp, --help dds)", err)
	}

	return img, nil, nil
}

func (impl) Encode(writer io.Writer, img image.Image, common opts.Common) error {
	settings := &config
	options := settings.encode

	sourceFormat, sourceAlpha := imageStorage(img)

	format := options.Format
	if format == dds.FormatUNKNOWN {
		format = sourceFormat
	}

	err := settings.outputOptions(format, true)
	if err != nil {
		return err
	}

	if settings.mipmaps.Filter&dds.FilterSeparateAlpha != 0 {
		return fmt.Errorf("--dds.mipmap-filter separate-alpha is native-only; image encoding requires associated-alpha filtering")
	}

	if settings.mips != "keep" {
		options.Mipmaps = &settings.mipmaps
	}

	if format.Info().Compressed {
		options.Compression = &settings.compression
	} else if settings.explicit["convert-filter"] || settings.explicit["alpha-threshold"] {
		options.Conversion = &settings.conversion
	}

	if common.Lossless {
		transfer := options.SourceColorSpace == dds.SourceColorSRGB && !format.Info().SRGB || options.SourceColorSpace == dds.SourceColorLinear && format.Info().SRGB
		association := options.AlphaMode != dds.AlphaUnknown && options.AlphaMode != sourceAlpha || format.Info().SRGB && sourceAlpha == dds.AlphaPremultiplied
		generate := options.Mipmaps != nil && options.Mipmaps.Levels != 1 && max(img.Bounds().Dx(), img.Bounds().Dy()) > 1

		if !losslessFormat(sourceFormat, format) || transfer || association || generate || settings.conversion.Filter != 0 || settings.conversion.AlphaThreshold != nil {
			return fmt.Errorf("dds: --lossless requires precision-preserving uncompressed storage without color/alpha conversion; use --dds.format auto")
		}
	}

	return settings.outputError(dds.Encode(writer, img, &options))
}

func (settings *configuration) outputError(err error) error {
	if err != nil && settings.encode.Header == dds.HeaderLegacy {
		return fmt.Errorf("dds: --dds.header legacy requires representable legacy storage and topology: %w", err)
	}

	return err
}

func (settings *configuration) outputOptions(format dds.Format, processing bool) error {
	info := format.Info()

	compressionOptions := settings.explicit["compression"] || settings.explicit["bc7-effort"]
	conversionOptions := settings.explicit["convert-filter"]

	if (compressionOptions && (!info.Compressed || !processing)) || (conversionOptions && (info.Compressed || !processing)) || (settings.explicit["alpha-threshold"] && !processing) {
		return fmt.Errorf("dds: compression/convert-filter/alpha-threshold options require the corresponding encoding operation")
	}

	bc7 := format == dds.FormatBC7_UNORM || format == dds.FormatBC7_UNORM_SRGB
	if settings.explicit["bc7-effort"] && !bc7 {
		return fmt.Errorf("--dds.bc7-effort requires BC7 encoding")
	}

	bc123 := format >= dds.FormatBC1_UNORM && format <= dds.FormatBC3_UNORM_SRGB
	if settings.compression.Flags&(dds.CompressRGBDither|dds.CompressAlphaDither|dds.CompressUniform) != 0 && !bc123 {
		return fmt.Errorf("dds: compression dithering and uniform weighting require BC1-3")
	}

	bc1 := format == dds.FormatBC1_UNORM || format == dds.FormatBC1_UNORM_SRGB
	if settings.explicit["alpha-threshold"] && !bc1 && format != dds.FormatB5G5R5A1_UNORM {
		return fmt.Errorf("--dds.alpha-threshold requires BC1 or B5G5R5A1_UNORM output")
	}

	return nil
}

func imageStorage(img image.Image) (dds.Format, dds.AlphaMode) {
	format := dds.FormatR16G16B16A16_UNORM
	alpha := dds.AlphaStraight

	switch img.(type) {
	case *image.NRGBA, *image.Gray, *image.Alpha, *image.YCbCr, *image.CMYK:
		format = dds.FormatR8G8B8A8_UNORM
	case *image.RGBA:
		format = dds.FormatR8G8B8A8_UNORM
		alpha = dds.AlphaPremultiplied
	case *image.RGBA64:
		alpha = dds.AlphaPremultiplied
	}

	return format, alpha
}

func losslessFormat(source, target dds.Format) bool {
	if source == target {
		return !target.Info().Compressed
	}

	if source == dds.FormatR8G8B8A8_UNORM || source == dds.FormatB8G8R8A8_UNORM {
		return target == dds.FormatR16G16B16A16_UNORM || target == dds.FormatR8G8B8A8_UNORM || target == dds.FormatB8G8R8A8_UNORM
	}

	return false
}
