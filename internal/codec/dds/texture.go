package dds

import (
	"fmt"
	"io"

	"github.com/coalaura/dds"
	"github.com/coalaura/ffwebp/internal/logx"
)

func (settings *configuration) processTexture(texture *dds.Texture, logger io.Writer) (*dds.Texture, error) {
	var err error

	if settings.decode.Interpretation != dds.FormatUNKNOWN {
		texture, err = texture.Interpret(settings.decode.Interpretation)
		if err != nil {
			return nil, err
		}
	}

	metadata := texture.Metadata()

	if settings.explicit["input-alpha"] && !metadata.Format.Info().Alpha {
		return nil, fmt.Errorf("--dds.input-alpha requires an input alpha channel")
	}

	if settings.decode.AlphaInterpretation != dds.AlphaUnknown && settings.decode.AlphaInterpretation != metadata.AlphaMode() {
		texture, err = interpretAlpha(texture, settings.decode.AlphaInterpretation)
		if err != nil {
			return nil, err
		}

		metadata = texture.Metadata()
	}

	if settings.explicit["premultiplication"] && (!metadata.IsSRGB() || metadata.AlphaMode() != dds.AlphaPremultiplied) {
		return nil, fmt.Errorf("--dds.premultiplication requires premultiplied sRGB input")
	}

	target := settings.encode.Format
	if target == dds.FormatUNKNOWN {
		target = metadata.Format
	}

	alpha := settings.encode.AlphaMode
	if alpha == dds.AlphaUnknown {
		alpha = metadata.AlphaMode()
	}

	// Drop association before discarding alpha, just as the image encoder does.
	dropAlpha := target != metadata.Format && !target.Info().Alpha && metadata.Format.Info().Alpha
	if dropAlpha && alpha == dds.AlphaPremultiplied && settings.encode.AlphaMode == dds.AlphaUnknown {
		alpha = dds.AlphaStraight
	}

	generate := settings.mips != "keep" && settings.mipmaps.Levels != 1
	convert := target != metadata.Format
	associate := alpha != metadata.AlphaMode()
	recompress := target.Info().Compressed && (settings.explicit["compression"] || settings.explicit["bc7-effort"] || settings.explicit["alpha-threshold"])

	process := generate || convert || associate || recompress

	if !target.Info().Compressed && !convert && !metadata.Format.Info().Compressed && (settings.explicit["convert-filter"] || settings.explicit["alpha-threshold"]) {
		return nil, fmt.Errorf("dds: convert-filter/alpha-threshold require a different uncompressed output format")
	}

	err = settings.outputOptions(target, process)
	if err != nil {
		return nil, err
	}

	if settings.encode.AlphaMode == dds.AlphaPremultiplied && !target.Info().Alpha {
		return nil, fmt.Errorf("dds: premultiplied output requires an alpha channel")
	}

	if settings.lossless && process {
		normalizeAssociation := metadata.IsSRGB() && metadata.AlphaMode() == dds.AlphaPremultiplied && settings.decode.Premultiplication == dds.SRGBPremultipliedEncoded

		if target.Info().Compressed || associate || settings.conversion.Filter != 0 || settings.conversion.AlphaThreshold != nil || settings.mipmaps.Filter != 0 || normalizeAssociation {
			return nil, fmt.Errorf("dds: --lossless prohibits BC encoding/recompression and lossy precision, range or alpha changes; untouched BC payloads can be preserved")
		}
	}

	if settings.mips != "keep" && settings.mipmaps.Levels == 1 {
		texture, err = texture.GenerateMipmaps(&settings.mipmaps)
		if err != nil {
			return nil, err
		}
	}

	if process && metadata.Format.Info().Compressed {
		texture, err = texture.Decompress(dds.FormatUNKNOWN)
		if err != nil {
			return nil, err
		}
	}

	if settings.lossless && process && !losslessFormat(texture.Metadata().Format, target) {
		return nil, fmt.Errorf("dds: --lossless prohibits lossy precision/range conversion; retain the native or precision-preserving decompressed format")
	}

	// Encoded association must be undone before format-driven transfers or mips.
	// The override describes existing samples; it must never multiply them again.
	encodedAssociation := metadata.IsSRGB() && metadata.AlphaMode() == dds.AlphaPremultiplied && settings.decode.Premultiplication == dds.SRGBPremultipliedEncoded
	if process && encodedAssociation {
		texture, err = texture.WithAlphaMode(dds.AlphaStraight, true)
		if err != nil {
			return nil, err
		}
	}

	if process && alpha != texture.Metadata().AlphaMode() {
		texture, err = texture.WithAlphaMode(alpha, false)
		if err != nil {
			return nil, err
		}
	}

	if generate {
		texture, err = texture.GenerateMipmaps(&settings.mipmaps)
		if err != nil {
			return nil, err
		}
	}

	if process {
		if target.Info().Compressed {
			texture, err = texture.Compress(target, &settings.compression)
		} else if texture.Metadata().Format != target || settings.explicit["convert-filter"] || settings.explicit["alpha-threshold"] {
			texture, err = texture.Convert(target, &settings.conversion)
		}

		if err != nil {
			return nil, err
		}
	}

	if dropAlpha {
		texture, err = interpretAlpha(texture, dds.AlphaOpaque)
		if err != nil {
			return nil, err
		}
	}

	metadata = texture.Metadata()

	logx.Fprintf(logger, "dds: native %dx%dx%d items=%d mips=%d format=%d; processed=%t (unchanged BC storage is preservation, not lossless BC encoding)\n", metadata.Width, metadata.Height, metadata.Depth, metadata.ArraySize, metadata.MipLevels, metadata.Format, process)

	return texture, nil
}

func interpretAlpha(texture *dds.Texture, alpha dds.AlphaMode) (*dds.Texture, error) {
	metadata := texture.Metadata()
	metadata.MiscFlags2 = metadata.MiscFlags2&^7 | uint32(alpha)

	surfaces := make([]dds.Subresource, 0, texture.SubresourceCount())

	// The public immutable API exposes native copies, not a metadata-only setter.
	// Rebuild only for an explicit interpretation override; never convert samples.
	for item := range metadata.ArraySize {
		for mip := range metadata.MipLevels {
			for slice := range max(1, metadata.Depth>>mip) {
				surface, err := texture.Surface(dds.Selection{Mip: mip, Item: item, Slice: slice})
				if err != nil {
					return nil, err
				}

				surfaces = append(surfaces, surface)
			}
		}
	}

	return dds.NewTexture(metadata, surfaces)
}
