package dds

import (
	"fmt"
	"strings"

	"github.com/coalaura/dds"
)

const (
	filterModes       = dds.FilterPoint | dds.FilterLinear | dds.FilterCubic | dds.FilterBox | dds.FilterTriangle
	mipmapOnlyFilters = dds.FilterWrapU | dds.FilterWrapV | dds.FilterWrapW | dds.FilterMirrorU | dds.FilterMirrorV | dds.FilterMirrorW | dds.FilterSeparateAlpha | filterModes
)

type namedBits struct {
	name  string
	value uint32
}

type namedFormat struct {
	name  string
	value dds.Format
}

var (
	// DXGI's contiguous range, followed by its sparse extension values below.
	formatNames = strings.Fields(`UNKNOWN R32G32B32A32_TYPELESS R32G32B32A32_FLOAT R32G32B32A32_UINT R32G32B32A32_SINT R32G32B32_TYPELESS R32G32B32_FLOAT R32G32B32_UINT R32G32B32_SINT R16G16B16A16_TYPELESS R16G16B16A16_FLOAT R16G16B16A16_UNORM R16G16B16A16_UINT R16G16B16A16_SNORM R16G16B16A16_SINT R32G32_TYPELESS R32G32_FLOAT R32G32_UINT R32G32_SINT R32G8X24_TYPELESS D32_FLOAT_S8X24_UINT R32_FLOAT_X8X24_TYPELESS X32_TYPELESS_G8X24_UINT R10G10B10A2_TYPELESS R10G10B10A2_UNORM R10G10B10A2_UINT R11G11B10_FLOAT R8G8B8A8_TYPELESS R8G8B8A8_UNORM R8G8B8A8_UNORM_SRGB R8G8B8A8_UINT R8G8B8A8_SNORM R8G8B8A8_SINT R16G16_TYPELESS R16G16_FLOAT R16G16_UNORM R16G16_UINT R16G16_SNORM R16G16_SINT R32_TYPELESS D32_FLOAT R32_FLOAT R32_UINT R32_SINT R24G8_TYPELESS D24_UNORM_S8_UINT R24_UNORM_X8_TYPELESS X24_TYPELESS_G8_UINT R8G8_TYPELESS R8G8_UNORM R8G8_UINT R8G8_SNORM R8G8_SINT R16_TYPELESS R16_FLOAT D16_UNORM R16_UNORM R16_UINT R16_SNORM R16_SINT R8_TYPELESS R8_UNORM R8_UINT R8_SNORM R8_SINT A8_UNORM R1_UNORM R9G9B9E5_SHAREDEXP R8G8_B8G8_UNORM G8R8_G8B8_UNORM BC1_TYPELESS BC1_UNORM BC1_UNORM_SRGB BC2_TYPELESS BC2_UNORM BC2_UNORM_SRGB BC3_TYPELESS BC3_UNORM BC3_UNORM_SRGB BC4_TYPELESS BC4_UNORM BC4_SNORM BC5_TYPELESS BC5_UNORM BC5_SNORM B5G6R5_UNORM B5G5R5A1_UNORM B8G8R8A8_UNORM B8G8R8X8_UNORM R10G10B10_XR_BIAS_A2_UNORM B8G8R8A8_TYPELESS B8G8R8A8_UNORM_SRGB B8G8R8X8_TYPELESS B8G8R8X8_UNORM_SRGB BC6H_TYPELESS BC6H_UF16 BC6H_SF16 BC7_TYPELESS BC7_UNORM BC7_UNORM_SRGB AYUV Y410 Y416 NV12 P010 P016 420_OPAQUE YUY2 Y210 Y216 NV11 AI44 IA44 P8 A8P8 B4G4R4A4_UNORM`)

	extraFormats = []namedFormat{
		{"P208", dds.FormatP208},
		{"V208", dds.FormatV208},
		{"V408", dds.FormatV408},
		{"SAMPLER_FEEDBACK_MIN_MIP_OPAQUE", dds.FormatSAMPLER_FEEDBACK_MIN_MIP_OPAQUE},
		{"SAMPLER_FEEDBACK_MIP_REGION_USED_OPAQUE", dds.FormatSAMPLER_FEEDBACK_MIP_REGION_USED_OPAQUE},
		{"A4B4G4R4_UNORM", dds.FormatA4B4G4R4_UNORM},
	}

	formatAliases = []namedFormat{
		{"rgba8", dds.FormatR8G8B8A8_UNORM},
		{"rgba8-srgb", dds.FormatR8G8B8A8_UNORM_SRGB},
		{"rgba8-snorm", dds.FormatR8G8B8A8_SNORM},
		{"rgba16", dds.FormatR16G16B16A16_UNORM},
		{"rgba16-float", dds.FormatR16G16B16A16_FLOAT},
		{"rgba16-snorm", dds.FormatR16G16B16A16_SNORM},
		{"bgra8", dds.FormatB8G8R8A8_UNORM},
		{"bgra8-srgb", dds.FormatB8G8R8A8_UNORM_SRGB},
		{"bc1", dds.FormatBC1_UNORM},
		{"bc1-srgb", dds.FormatBC1_UNORM_SRGB},
		{"bc2", dds.FormatBC2_UNORM},
		{"bc2-srgb", dds.FormatBC2_UNORM_SRGB},
		{"bc3", dds.FormatBC3_UNORM},
		{"bc3-srgb", dds.FormatBC3_UNORM_SRGB},
		{"bc4", dds.FormatBC4_UNORM},
		{"bc4-snorm", dds.FormatBC4_SNORM},
		{"bc5", dds.FormatBC5_UNORM},
		{"bc5-snorm", dds.FormatBC5_SNORM},
		{"bc6", dds.FormatBC6H_UF16},
		{"bc6h", dds.FormatBC6H_UF16},
		{"bc6-signed", dds.FormatBC6H_SF16},
		{"bc6h-signed", dds.FormatBC6H_SF16},
		{"bc7", dds.FormatBC7_UNORM},
		{"bc7-srgb", dds.FormatBC7_UNORM_SRGB},
		{"dxt1", dds.FormatBC1_UNORM},
		{"dxt3", dds.FormatBC2_UNORM},
		{"dxt5", dds.FormatBC3_UNORM},
		{"ati1", dds.FormatBC4_UNORM},
		{"ati2", dds.FormatBC5_UNORM},
	}

	loadNames = []namedBits{
		{"legacy-dword", uint32(dds.LoadLegacyDWORD)},
		{"no-legacy-expansion", uint32(dds.LoadNoLegacyExpansion)},
		{"no-r10-fixup", uint32(dds.LoadNoR10Fixup)},
		{"force-rgb", uint32(dds.LoadForceRGB)},
		{"expand-16bit", uint32(dds.LoadExpand16Bit)},
		{"expand-luminance", uint32(dds.LoadExpandLuminance)},
		{"bad-dxt-tails", uint32(dds.LoadBadDXTTails)},
		{"permissive", uint32(dds.LoadPermissive)},
		{"ignore-mips", uint32(dds.LoadIgnoreMips)},
	}

	filterNames = []namedBits{
		{"wrap-u", uint32(dds.FilterWrapU)},
		{"wrap-v", uint32(dds.FilterWrapV)},
		{"wrap-w", uint32(dds.FilterWrapW)},
		{"mirror-u", uint32(dds.FilterMirrorU)},
		{"mirror-v", uint32(dds.FilterMirrorV)},
		{"mirror-w", uint32(dds.FilterMirrorW)},
		{"separate-alpha", uint32(dds.FilterSeparateAlpha)},
		{"float-x2-bias", uint32(dds.FilterFloatX2Bias)},
		{"copy-red", uint32(dds.FilterCopyRed)},
		{"copy-green", uint32(dds.FilterCopyGreen)},
		{"copy-blue", uint32(dds.FilterCopyBlue)},
		{"copy-alpha", uint32(dds.FilterCopyAlpha)},
		{"dither", uint32(dds.FilterDither)},
		{"dither-diffusion", uint32(dds.FilterDitherDiffusion)},
		{"point", uint32(dds.FilterPoint)},
		{"linear", uint32(dds.FilterLinear)},
		{"cubic", uint32(dds.FilterCubic)},
		{"box", uint32(dds.FilterBox)},
		{"triangle", uint32(dds.FilterTriangle)},
		{"srgb-in", uint32(dds.FilterSRGBIn)},
		{"srgb-out", uint32(dds.FilterSRGBOut)},
	}

	compressionNames = []namedBits{
		{"rgb-dither", uint32(dds.CompressRGBDither)},
		{"alpha-dither", uint32(dds.CompressAlphaDither)},
		{"uniform", uint32(dds.CompressUniform)},
		{"srgb-in", uint32(dds.CompressSRGBIn)},
		{"srgb-out", uint32(dds.CompressSRGBOut)},
	}

	filterConflicts = []dds.FilterFlags{
		dds.FilterWrapU | dds.FilterMirrorU,
		dds.FilterWrapV | dds.FilterMirrorV,
		dds.FilterWrapW | dds.FilterMirrorW,
		dds.FilterDither | dds.FilterDitherDiffusion,
	}
)

func parseFormat(value string, automatic bool) (dds.Format, error) {
	name := strings.TrimPrefix(strings.ToUpper(value), "DXGI_FORMAT_")
	if automatic && name == "AUTO" {
		return dds.FormatUNKNOWN, nil
	}

	if name == "DXT2" || name == "DXT4" {
		return 0, fmt.Errorf("dds: %s also implies premultiplied alpha; use bc2/bc3 with --dds.alpha premultiplied", name)
	}

	for index, canonical := range formatNames {
		if index != 0 && name == canonical {
			return dds.Format(index), nil
		}
	}

	for _, entry := range extraFormats {
		if name == entry.name {
			return entry.value, nil
		}
	}

	for _, entry := range formatAliases {
		if strings.EqualFold(value, entry.name) {
			return entry.value, nil
		}
	}

	return 0, fmt.Errorf("dds: unknown format %q; see --help dds", value)
}

func parseBits(value string, registry []namedBits, filter bool) (uint32, error) {
	if value == "" || strings.EqualFold(value, "default") || strings.EqualFold(value, "none") {
		return 0, nil
	}

	var result uint32

	for token := range strings.SplitSeq(strings.ToLower(value), ",") {
		token = strings.TrimSpace(token)

		var found bool

		for _, entry := range registry {
			if token != entry.name {
				continue
			}

			modeConflict := filter && dds.FilterFlags(entry.value)&filterModes != 0 && dds.FilterFlags(result)&filterModes != 0
			if result&entry.value != 0 || modeConflict {
				return 0, fmt.Errorf("duplicate or conflicting token %q", token)
			}

			result |= entry.value
			found = true

			break
		}

		if !found {
			return 0, fmt.Errorf("unknown token %q", token)
		}
	}

	if filter {
		for _, conflict := range filterConflicts {
			if dds.FilterFlags(result)&conflict == conflict {
				return 0, fmt.Errorf("conflicting addressing or dithering flags")
			}
		}
	}

	return result, nil
}

func bitNames(registry []namedBits, exclude uint32) string {
	var size int

	for _, entry := range registry {
		if entry.value&exclude == 0 {
			size += len(entry.name) + 2
		}
	}

	var builder strings.Builder

	builder.Grow(size)

	for _, entry := range registry {
		if entry.value&exclude != 0 {
			continue
		}

		if builder.Len() > 0 {
			builder.WriteString(", ")
		}

		builder.WriteString(entry.name)
	}

	return builder.String()
}
