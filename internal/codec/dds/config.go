package dds

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/coalaura/dds"
	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/urfave/cli/v3"
)

type configuration struct {
	scope           string
	decode          dds.DecodeOptions
	encode          dds.Options
	compression     dds.CompressionOptions
	conversion      dds.ConversionOptions
	mipmaps         dds.MipmapOptions
	mips            string
	cube            int
	face            dds.CubeFace
	selectors       bool
	cubeSelection   bool
	imageOperations bool
	lossless        bool
	quality         bool
	explicit        map[string]bool
}

var codecFlags = []cli.Flag{
	&cli.StringFlag{Name: "dds.scope", Value: "auto", Usage: "DDS: auto|image|texture; auto preserves complete DDS-to-DDS textures; selectors choose image"},
	&cli.IntFlag{Name: "dds.mip", Usage: "DDS: zero-based mip selection (image scope)"},
	&cli.IntFlag{Name: "dds.item", Usage: "DDS: zero-based array item or cube face selection (image scope)"},
	&cli.IntFlag{Name: "dds.slice", Usage: "DDS: zero-based depth slice selection (image scope)"},
	&cli.IntFlag{Name: "dds.cube", Usage: "DDS: zero-based cube in a cube array; conflicts with item/slice"},
	&cli.StringFlag{Name: "dds.face", Value: "px", Usage: "DDS: cube face px|nx|py|ny|pz|nz; conflicts with item/slice"},
	&cli.StringFlag{Name: "dds.interpret", Usage: "DDS: explicit compatible typed DXGI format for typeless input"},
	&cli.StringFlag{Name: "dds.precision", Value: "auto", Usage: "DDS: image extraction precision: auto|8|16; auto preserves native precision"},
	&cli.StringFlag{Name: "dds.display", Value: "strict", Usage: "DDS: image extraction: strict|clamp; clamp is NOT tone mapping or normal reconstruction"},
	&cli.StringFlag{Name: "dds.input-alpha", Value: "auto", Usage: "DDS: sample interpretation: auto|straight|premultiplied (does not multiply samples)"},
	&cli.StringFlag{Name: "dds.premultiplication", Value: "linear", Usage: "DDS: premultiplied-sRGB input convention: linear|encoded (not a gamma toggle)"},
	&cli.Int64Flag{Name: "dds.max-bytes", Usage: "DDS: input/native-payload loading budget: 0=512 MiB, or 148..2147483647; not a process memory cap"},
	&cli.StringFlag{Name: "dds.load", Usage: "DDS: comma-separated compatibility flags; strict by default (see tokens below)"},
	&cli.StringFlag{Name: "dds.decode-filter", Usage: "DDS: comma-separated image conversion controls (see tokens below)"},
	&cli.StringFlag{Name: "dds.format", Value: "auto", Usage: "DDS: auto|FORMAT; native auto preserves storage, image auto uses precision-preserving uncompressed RGBA"},
	&cli.StringFlag{Name: "dds.header", Value: "default", Usage: "DDS: default|auto|legacy|dx10; auto/legacy can lose alpha intent; legacy rejects unrepresentable textures"},
	&cli.StringFlag{Name: "dds.alpha", Value: "auto", Usage: "DDS: output association: auto|straight|premultiplied; converts samples"},
	&cli.StringFlag{Name: "dds.source-color", Value: "default", Usage: "DDS: image encoding only: default|linear|srgb; default follows destination transfer function"},
	&cli.StringFlag{Name: "dds.mipmaps", Value: "keep", Usage: "DDS: keep|none|full|N; N includes base; keep adds none to images, full/N replaces the native chain"},
	&cli.StringFlag{Name: "dds.mipmap-filter", Usage: "DDS: comma-separated mip filtering controls; separate-alpha is native-only"},
	&cli.StringFlag{Name: "dds.convert-filter", Usage: "DDS: comma-separated uncompressed conversion controls"},
	&cli.StringFlag{Name: "dds.bc7-effort", Value: "default", Usage: "DDS: default|quick|exhaustive; BC7 search effort, never bitrate or --quality"},
	&cli.Float64Flag{Name: "dds.alpha-threshold", Value: 0.5, Usage: "DDS: 0..1, including explicit zero; BC1 compression or supported uncompressed conversion"},
	&cli.StringFlag{Name: "dds.compression", Usage: "DDS: comma-separated BC compression controls; dithering/uniform require BC1-3"},
}

func (impl) Configure(command *cli.Command) error {
	settings := configuration{explicit: make(map[string]bool, len(codecFlags))}

	for _, flag := range codecFlags {
		name := flag.Names()[0]
		settings.explicit[strings.TrimPrefix(name, "dds.")] = command.IsSet(name)
	}

	settings.lossless = command.Bool("lossless")
	settings.quality = command.IsSet("quality")
	settings.imageOperations = command.Bool("play") || command.IsSet("join") || command.IsSet("split") || command.IsSet("thumbnail") || command.String("effects") != "" || command.IsSet("frame") || command.IsSet("time")
	settings.scope = strings.ToLower(command.String("dds.scope"))

	if settings.scope != "auto" && settings.scope != "image" && settings.scope != "texture" {
		return fmt.Errorf("--dds.scope must be auto, image or texture")
	}

	var err error

	settings.decode.Surface.Mip, err = selectionValue(command, "mip")
	if err != nil {
		return err
	}

	settings.decode.Surface.Item, err = selectionValue(command, "item")
	if err != nil {
		return err
	}

	settings.decode.Surface.Slice, err = selectionValue(command, "slice")
	if err != nil {
		return err
	}

	settings.cube, err = selectionValue(command, "cube")
	if err != nil {
		return err
	}

	face, err := enumValue(command, "face", "px nx py ny pz nz")
	if err != nil {
		return err
	}

	settings.face = dds.CubeFace(face)
	settings.cubeSelection = settings.explicit["cube"] || settings.explicit["face"]
	settings.selectors = settings.cubeSelection || settings.explicit["mip"] || settings.explicit["item"] || settings.explicit["slice"]

	if settings.cubeSelection && (settings.explicit["item"] || settings.explicit["slice"]) {
		return fmt.Errorf("dds: cube/face conflict with item/slice")
	}

	if settings.selectors && settings.scope == "texture" {
		return fmt.Errorf("dds: surface selectors require image scope")
	}

	settings.encode.Format, err = parseFormat(command.String("dds.format"), true)
	if err != nil {
		return err
	}

	if settings.explicit["interpret"] {
		settings.decode.Interpretation, err = parseFormat(command.String("dds.interpret"), false)
		if err != nil {
			return err
		}
	}

	precision, err := enumValue(command, "precision", "auto 8 16")
	if err != nil {
		return err
	}

	settings.decode.Precision = dds.ImagePrecision(precision)

	display, err := enumValue(command, "display", "strict clamp")
	if err != nil {
		return err
	}

	settings.decode.Display = dds.DisplayMode(display)

	alpha, err := enumValue(command, "input-alpha", "auto straight premultiplied")
	if err != nil {
		return err
	}

	settings.decode.AlphaInterpretation = dds.AlphaMode(alpha)

	alpha, err = enumValue(command, "alpha", "auto straight premultiplied")
	if err != nil {
		return err
	}

	settings.encode.AlphaMode = dds.AlphaMode(alpha)

	premultiplication, err := enumValue(command, "premultiplication", "linear encoded")
	if err != nil {
		return err
	}

	settings.decode.Premultiplication = dds.SRGBPremultiplication(premultiplication)

	header, err := enumValue(command, "header", "default legacy auto dx10")
	if err != nil {
		return err
	}

	settings.encode.Header = dds.HeaderMode(header)

	colorSpace, err := enumValue(command, "source-color", "default linear srgb")
	if err != nil {
		return err
	}

	settings.encode.SourceColorSpace = dds.SourceColorSpace(colorSpace)

	effort, err := enumValue(command, "bc7-effort", "default quick exhaustive")
	if err != nil {
		return err
	}

	settings.compression.BC7Effort = dds.BC7Effort(effort)

	settings.decode.Load.MaxBytes = command.Int64("dds.max-bytes")
	if settings.decode.Load.MaxBytes < 0 || settings.decode.Load.MaxBytes > math.MaxInt32 || (settings.decode.Load.MaxBytes != 0 && settings.decode.Load.MaxBytes < 148) {
		return fmt.Errorf("--dds.max-bytes must be 0 (default) or 148..2147483647")
	}

	load, err := parseBits(command.String("dds.load"), loadNames, false)
	if err != nil {
		return fmt.Errorf("--dds.load: %w", err)
	}

	settings.decode.Load.Flags = dds.LoadFlags(load)

	compression, err := parseBits(command.String("dds.compression"), compressionNames, false)
	if err != nil {
		return fmt.Errorf("--dds.compression: %w", err)
	}

	settings.compression.Flags = dds.CompressionFlags(compression)

	settings.decode.Filter, err = filterValue(command, "decode-filter", false)
	if err != nil {
		return err
	}

	settings.conversion.Filter, err = filterValue(command, "convert-filter", false)
	if err != nil {
		return err
	}

	settings.mipmaps.Filter, err = filterValue(command, "mipmap-filter", true)
	if err != nil {
		return err
	}

	if settings.explicit["alpha-threshold"] {
		threshold := command.Float64("dds.alpha-threshold")
		if math.IsNaN(threshold) || threshold < 0 || threshold > 1 {
			return fmt.Errorf("--dds.alpha-threshold must be in [0,1]")
		}

		settings.compression.AlphaThreshold = new(float32(threshold))
		settings.conversion.AlphaThreshold = settings.compression.AlphaThreshold
	}

	settings.mips = strings.ToLower(command.String("dds.mipmaps"))

	switch settings.mips {
	case "keep":
	case "none":
		settings.mipmaps.Levels = 1
	case "full":
	default:
		settings.mipmaps.Levels, err = strconv.Atoi(settings.mips)
		if err != nil || settings.mipmaps.Levels < 1 || settings.mipmaps.Levels > 16 {
			return fmt.Errorf("--dds.mipmaps must be keep, none, full or 1..16 (including base)")
		}
	}

	if settings.explicit["mipmap-filter"] && (settings.mips == "keep" || settings.mipmaps.Levels == 1) {
		return fmt.Errorf("--dds.mipmap-filter requires generated mip levels")
	}

	config = settings

	return nil
}

func (impl) ValidateRoute(input, output codec.Codec) error {
	settings := &config

	ddsInput := input != nil && input.String() == "dds"
	ddsOutput := output != nil && output.String() == "dds"

	native := settings.native(ddsInput, ddsOutput)

	if settings.scope == "texture" && (!ddsInput || !ddsOutput) {
		return fmt.Errorf("--dds.scope texture requires DDS input and output")
	}

	if native && settings.imageOperations {
		return fmt.Errorf("dds: image operations (thumbnail, effects, join/split, playback, frame/time) require --dds.scope image; native textures are never flattened implicitly")
	}

	if ddsOutput && settings.quality {
		return fmt.Errorf("dds: --quality has no DDS bitrate meaning; use --dds.format and --dds.bc7-effort (BC storage is lossy and fixed-size)")
	}

	for name, explicit := range settings.explicit {
		if !explicit {
			continue
		}

		applicable := false

		switch name {
		case "scope":
			applicable = ddsInput || ddsOutput
		case "precision", "display", "decode-filter":
			applicable = ddsInput && !native
		case "source-color":
			applicable = ddsOutput && !native
		case "mip", "item", "slice", "cube", "face", "interpret", "input-alpha", "premultiplication", "max-bytes", "load":
			applicable = ddsInput
		case "format", "header", "alpha", "mipmaps", "mipmap-filter", "convert-filter", "bc7-effort", "alpha-threshold", "compression":
			applicable = ddsOutput
		}

		if !applicable {
			return fmt.Errorf("--dds.%s cannot affect this operation", name)
		}
	}

	return nil
}

func (settings *configuration) native(input, output bool) bool {
	return settings.scope == "texture" || (settings.scope == "auto" && input && output && !settings.selectors)
}

func selectionValue(command *cli.Command, name string) (int, error) {
	value := command.Int("dds." + name)
	if value < 0 || value > 65535 {
		return 0, fmt.Errorf("--dds.%s must be a zero-based index in 0..65535", name)
	}

	return value, nil
}

func enumValue(command *cli.Command, name, choices string) (uint32, error) {
	value := strings.ToLower(command.String("dds." + name))

	for index, choice := range strings.Fields(choices) {
		if value == choice {
			return uint32(index), nil
		}
	}

	return 0, fmt.Errorf("--dds.%s must be one of: %s", name, choices)
}

func filterValue(command *cli.Command, name string, mipmaps bool) (dds.FilterFlags, error) {
	value, err := parseBits(command.String("dds."+name), filterNames, true)
	if err != nil {
		return 0, fmt.Errorf("--dds.%s: %w", name, err)
	}

	mipmapOnly := dds.FilterWrapU | dds.FilterWrapV | dds.FilterWrapW | dds.FilterMirrorU | dds.FilterMirrorV | dds.FilterMirrorW | dds.FilterSeparateAlpha | filterModes
	if !mipmaps && dds.FilterFlags(value)&mipmapOnly != 0 {
		return 0, fmt.Errorf("--dds.%s: addressing, filter mode and separate-alpha apply only to mipmaps", name)
	}

	return dds.FilterFlags(value), nil
}
