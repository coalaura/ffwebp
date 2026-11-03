package jpegxl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coalaura/ffwebp/internal/codec/jpegutil"
	"github.com/coalaura/ffwebp/internal/opts"
	jxl "github.com/coalaura/jpeg/jpegxl"
	"github.com/urfave/cli/v3"
)

var settingIDs = map[string]jxl.SettingID{
	"resampling":                       jxl.Resampling,
	"extra-channel-resampling":         jxl.ExtraChannelResampling,
	"noise":                            jxl.Noise,
	"dots":                             jxl.Dots,
	"patches":                          jxl.Patches,
	"edge-preserving-filter":           jxl.EdgePreservingFilter,
	"gaborish":                         jxl.Gaborish,
	"modular":                          jxl.Modular,
	"keep-invisible":                   jxl.KeepInvisible,
	"group-order":                      jxl.GroupOrder,
	"group-order-center-x":             jxl.GroupOrderCenterX,
	"group-order-center-y":             jxl.GroupOrderCenterY,
	"responsive":                       jxl.Responsive,
	"progressive-ac":                   jxl.ProgressiveAC,
	"quantized-progressive-ac":         jxl.QuantizedProgressiveAC,
	"progressive-dc":                   jxl.ProgressiveDC,
	"channel-colors-global-percent":    jxl.ChannelColorsGlobalPercent,
	"channel-colors-group-percent":     jxl.ChannelColorsGroupPercent,
	"palette-colors":                   jxl.PaletteColors,
	"lossy-palette":                    jxl.LossyPalette,
	"color-transform":                  jxl.ColorTransform,
	"modular-color-space":              jxl.ModularColorSpace,
	"modular-group-size":               jxl.ModularGroupSize,
	"modular-predictor":                jxl.ModularPredictor,
	"modular-ma-tree-learning-percent": jxl.ModularMATreeLearningPercent,
	"modular-previous-channels":        jxl.ModularPreviousChannels,
	"jpeg-reconstruction-cfl":          jxl.JPEGReconstructionCFL,
	"brotli-effort":                    jxl.BrotliEffort,
	"jpeg-compress-boxes":              jxl.JPEGCompressBoxes,
	"buffering":                        jxl.Buffering,
	"full-image-heuristics":            jxl.FullImageHeuristics,
	"disable-perceptual-heuristics":    jxl.DisablePerceptualHeuristics,
}

func (impl) Flags(flags []cli.Flag) []cli.Flag {
	flags = append(flags,
		&cli.IntFlag{Name: "jpegxl.effort", Value: 7, Usage: "JPEG XL: encoding effort, 1..10 (0 selects 7)"},
		&cli.Float64Flag{Name: "jpegxl.distance", Usage: "JPEG XL: explicit perceptual distance 0..25; omitted maps --quality using libjxl's curve (85=1.45); conflicts with explicit --quality; zero alone is not exact lossless"},
		&cli.Float64Flag{Name: "jpegxl.alpha-distance", Usage: "JPEG XL: alpha distance 0..25; omitted or zero preserves exact alpha"},
		&cli.BoolFlag{Name: "jpegxl.lossless", Usage: "JPEG XL: exact lossless pixels (also selected by --lossless)"},
		&cli.IntFlag{Name: "jpegxl.decoding-speed", Usage: "JPEG XL: decoding-speed tier, 0..4"},
		&cli.IntFlag{Name: "jpegxl.precision", Usage: "JPEG XL: bits/sample 2..16; 0 preserves source precision"},
		&cli.IntFlag{Name: "jpegxl.threads", Usage: "JPEG XL: native encode/decode workers; 0=1, maximum 16"},
		&cli.BoolFlag{Name: "jpegxl.container", Value: true, Usage: "JPEG XL: boxed output (default); --jpegxl.container=false strictly requires a raw codestream or fails"},
		&cli.IntFlag{Name: "jpegxl.codestream-level", Usage: "JPEG XL: 0=automatic, 5 or 10; level 10 requires a container"},
		&cli.BoolFlag{Name: "jpegxl.linear", Usage: "JPEG XL: interpret encoder pixels as linear sRGB rather than nonlinear sRGB"},
		&cli.IntFlag{Name: "jpegxl.orientation", Usage: "JPEG XL: encoder EXIF orientation, 1..8; 0=identity"},
		&cli.Float64Flag{Name: "jpegxl.intensity-target", Usage: "JPEG XL: encoder intensity target in nits; 0=profile default"},
		&cli.BoolFlag{Name: "jpegxl.keep-orientation", Usage: "JPEG XL input: preserve stored orientation rather than applying it to pixels"},
		&cli.BoolFlag{Name: "jpegxl.unpremultiply-alpha", Usage: "JPEG XL input: explicitly allow potentially lossy conversion to straight alpha"},
		&cli.BoolFlag{Name: "jpegxl.render-spot-colors", Usage: "JPEG XL input: render spot channels into color; other unsupported extra channels fail"},
		&cli.Float64Flag{Name: "jpegxl.decode-intensity-target", Usage: "JPEG XL input: HDR tone-mapping target in nits; 0=disabled; floating-point output still requires external display conversion"},
		&cli.StringSliceFlag{Name: "jpegxl.setting", Usage: "JPEG XL: repeat name=integer; names: resampling, extra-channel-resampling, noise, dots, patches, edge-preserving-filter, gaborish, modular, keep-invisible, group-order, group-order-center-x, group-order-center-y, responsive, progressive-ac, quantized-progressive-ac, progressive-dc, channel-colors-global-percent, channel-colors-group-percent, palette-colors, lossy-palette, color-transform, modular-color-space, modular-group-size, modular-predictor, modular-ma-tree-learning-percent, modular-previous-channels, jpeg-reconstruction-cfl, brotli-effort, jpeg-compress-boxes, buffering, full-image-heuristics, disable-perceptual-heuristics; most accept -1=default, 0=off, 1=on; libjxl validates ranges"},
		&cli.Uint64Flag{Name: "jpegxl.max-memory-bytes", Usage: "JPEG XL: native encode/decode working memory limit; 0=512 MiB"},
		&cli.Uint64Flag{Name: "jpegxl.max-decoded-bytes", Usage: "JPEG XL input: total returned frame sample bytes; 0=512 MiB"},
		&cli.IntFlag{Name: "jpegxl.max-frames", Usage: "JPEG XL input: frame limit; 0=256"},
		&cli.BoolFlag{Name: "jpegxl.recompress-jpeg", Usage: "JPEG -> boxed JXL: preserve original JPEG bytes for reconstruction; rejects pixel operations; library input limits are 64 MiB and 16 million pixels"},
		&cli.BoolFlag{Name: "jpegxl.restore-jpeg", Usage: "JXL -> original JPEG: require reconstruction data and restore exact original bytes; rejects pixel operations"},
	)

	return jpegutil.Limits(flags, "jpegxl")
}

func decodeOptions() jxl.DecodeOptions {
	return jxl.DecodeOptions{
		Threads:            command.Int("jpegxl.threads"),
		MaxInputBytes:      command.Int64("jpegxl.max-input-bytes"),
		MaxPixels:          command.Uint64("jpegxl.max-pixels"),
		MaxMemoryBytes:     command.Uint64("jpegxl.max-memory-bytes"),
		MaxDecodedBytes:    command.Uint64("jpegxl.max-decoded-bytes"),
		MaxFrames:          command.Int("jpegxl.max-frames"),
		KeepOrientation:    command.Bool("jpegxl.keep-orientation"),
		UnpremultiplyAlpha: command.Bool("jpegxl.unpremultiply-alpha"),
		RenderSpotColors:   command.Bool("jpegxl.render-spot-colors"),
		IntensityTarget:    float32(command.Float64("jpegxl.decode-intensity-target")),
	}
}

func encodeOptions(common opts.Common) (jxl.Options, error) {
	settings := jxl.Options{
		Lossless:        common.Lossless || command.Bool("jpegxl.lossless"),
		Distance:        new(qualityDistance(common.Quality)),
		AlphaDistance:   jpegutil.OptionalFloat(command, "jpegxl.alpha-distance"),
		Effort:          command.Int("jpegxl.effort"),
		DecodingSpeed:   command.Int("jpegxl.decoding-speed"),
		Threads:         command.Int("jpegxl.threads"),
		BitsPerSample:   command.Int("jpegxl.precision"),
		Container:       command.Bool("jpegxl.container"),
		Linear:          command.Bool("jpegxl.linear"),
		Orientation:     command.Int("jpegxl.orientation"),
		IntensityTarget: float32(command.Float64("jpegxl.intensity-target")),
		CodestreamLevel: command.Int("jpegxl.codestream-level"),
		MaxMemoryBytes:  command.Uint64("jpegxl.max-memory-bytes"),
		ICC:             common.ICC,
	}

	if command.IsSet("jpegxl.linear") {
		settings.ICC = nil
	}

	if command.IsSet("jpegxl.distance") {
		if command.IsSet("quality") {
			return settings, fmt.Errorf("jpegxl: explicit --quality and --jpegxl.distance are mutually exclusive")
		}

		settings.Distance = jpegutil.OptionalFloat(command, "jpegxl.distance")
		if settings.Lossless && *settings.Distance != 0 {
			return settings, fmt.Errorf("jpegxl: nonzero distance conflicts with lossless encoding")
		}
	}

	if !settings.Container && settings.CodestreamLevel == 10 {
		return settings, fmt.Errorf("jpegxl: codestream level 10 requires a container; strict raw output cannot represent it")
	}

	values := command.StringSlice("jpegxl.setting")

	settings.Settings = make([]jxl.Setting, 0, len(values))
	seen := make(map[jxl.SettingID]bool, len(values))

	for _, value := range values {
		name, integer, found := strings.Cut(value, "=")
		identifier, known := settingIDs[name]

		if !found || !known {
			return settings, fmt.Errorf("jpegxl: unknown setting %q; see --help jpegxl for name=value controls", value)
		}

		number, err := strconv.Atoi(integer)
		if err != nil {
			return settings, fmt.Errorf("jpegxl: setting %s requires an integer", name)
		}

		if seen[identifier] {
			return settings, fmt.Errorf("jpegxl: duplicate setting %s", name)
		}

		seen[identifier] = true
		settings.Settings = append(settings.Settings, jxl.Setting{ID: identifier, Value: number})
	}

	return settings, nil
}

func qualityDistance(quality int) float32 {
	if quality >= 100 {
		return 0
	}

	if quality >= 30 {
		return 0.1 + (100-float32(quality))*0.09
	}

	value := float32(quality)
	return 53.0/3000.0*value*value - 23.0/20.0*value + 25.0
}
