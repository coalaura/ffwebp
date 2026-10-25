# FFWebP

FFWebP is a small, single-binary CLI for converting images between formats, think "ffmpeg for images". It auto-detects the input format, lets you pick the output format by file extension or `--codec`, supports stdin/stdout, thumbnails and rich per-codec options.

## Features

- Single binary: no external tools required
- Auto-detects input codec and infers output from the file extension
- Supports AVIF, BMP, DNG (embedded JPEG preview only), Farbfeld, GIF, HEIF/HEIC (decode-only), ICO/CUR, JPEG, JPEG XL, PCX, PNG, PNM (PBM/PGM/PPM/PAM), PSD (decode-only), QOI, SVG (decode-only), TGA, TIFF, WebP, XBM, XCF (decode-only) and XPM
- Lossy or lossless output with configurable quality
- Thumbnail generation via Lanczos3 resampling
- Per-codec flags for fine-grained control (see `ffwebp --help`)

## Installation

### Prebuilt binaries (recommended)

You can bootstrap **ffwebp-full** with a single command. This script will detect your OS and CPU (`amd64`/`arm64`), download the correct binary and install it to `/usr/local/bin/ffwebp`.

```bash
curl -sL https://src.ws2.sh/ffwebp/install.sh | sh
```

### Build from source (optional)

```bash
# All codecs
go build -tags full -o ffwebp ./cmd/ffwebp

# Core set (common formats)
go build -tags core -o ffwebp ./cmd/ffwebp

# Custom subset
go build -tags "jpeg,png,webp" -o ffwebp ./cmd/ffwebp
```

Notes
- The banner at startup prints the compiled-in codecs (from build tags).
- You can enable individual codecs with tags matching their names (e.g., `-tags avif,jpegxl,tiff`).

## Usage

Basic conversion
```bash
ffwebp -i input.jpg output.webp
# or, equivalently
ffwebp -i input.jpg -o output.webp
```

The output file, directory or pattern can be supplied as a positional argument instead of `-o`/`--output`. Use only one form per command. If neither is supplied, output defaults to stdout.

Pipe stdin/stdout
```bash
cat input.png | ffwebp -o out.jpg
# or
ffwebp < input.png > out.jpg
```

Force output codec (if the output name has no or different extension)
```bash
ffwebp -i in.png -o out.any -c jpeg
```

Quality and lossless
```bash
# Lossy quality (0–100, defaults to 85)
ffwebp -i in.png -o out.webp -q 82

# Force lossless (overrides quality)
ffwebp -i in.png -o out.webp --lossless
```

For AVIF and JPEG XL, `--lossless` enables the encoder's explicit lossless mode. AVIF also forces alpha quality 100 and 4:4:4 chroma, overriding `--avif.quality-alpha` and `--avif.chroma`. Quality 100 alone is not a portable substitute for `--lossless`.

Thumbnails
```bash
# Create a thumbnail no larger than 256x256
ffwebp -i big.jpg -o thumb.webp -t 256
```

Silence logs
```bash
ffwebp -i in.jpg -o out.png -s
```

## Codec Options

Each codec exposes its available options via namespaced flags (for example, `--webp.method`, `--tiff.predictor`, `--psd.skip-merged`). Run `ffwebp --help` to see all global and codec-specific flags for your build.

If `--codec` is omitted, the output codec is chosen from the output file extension. If the output filename has no extension, pass `--codec`.

### Format limitations

- AVIF supports animated input and output, including `--frame`, `--time`, and conversion to other animation-capable codecs. Frame durations are rounded to milliseconds. The pinned AVIF decoder does not expose loop counts, so decoded animations default to infinite looping. Its encoder cannot express single-play animations; those inputs produce an error instead of silently looping forever.
- JPEG XL currently uses the first frame. The pinned library exposes animated decoding but returns raw frame ticks without the animation tick rate or loop count, so reliable animation timing requires an upstream API change.
- TIFF output supports uncompressed (`--tiff.compression 0`) and Deflate (`1`, the default) only. The differencing predictor applies to Deflate. Input supports classic TIFF, not BigTIFF.
- DNG support extracts the embedded JPEG preview; it does not develop raw sensor data. Preview dimensions and quality can differ from the raw image, and files without a supported JPEG preview cannot be converted. The adapter is included in `full` builds or with the `dng` build tag.
- SVG rendering uses resvg's PNG-returning API. Supplying both `--svg.width` and `--svg.height` renders directly at that size. Supplying just one dimension preserves the intrinsic aspect ratio, which can require a preliminary render with the pinned renderer.

## How It Works

- Input sniffing: the input codec is detected by reading magic bytes; if you provide an input filename, its extension is considered.
- Output selection: the output codec is inferred from the destination extension or forced via `--codec`.
- Timing and sizes: ffwebp prints info like decode/encode timings and output size unless `--silent` is set.

## License

See the [LICENSE](LICENSE) file.
