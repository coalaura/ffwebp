# FFWebP

FFWebP is a small, single-binary CLI for converting images between formats, think "ffmpeg for images". It auto-detects the input format, lets you pick the output format by file extension or `--codec`, supports stdin/stdout, thumbnails, and rich per-codec options.

## Features

- Single binary: no external tools required
- Auto-detects input codec and infers output from the file extension
- Supports AVIF, BMP, Farbfeld, GIF, HEIF/HEIC (decode-only), ICO/CUR, JPEG, JPEG XL, PCX, PNG, PNM (PBM/PGM/PPM/PAM), PSD (decode-only), QOI, SVG (decode-only), TGA, TIFF, WebP, XBM, XCF (decode-only) and XPM
- Lossy or lossless output with configurable quality
- Thumbnail generation via Lanczos3 resampling
- Per-codec flags for fine-grained control (see `ffwebp --help`)

## Installation

### Prebuilt binaries (recommended)

You can bootstrap **ffwebp-full** with a single command. This script will detect your OS and CPU (`amd64`/`arm64`), download the correct binary and install it to `/usr/local/bin/ffwebp`.

```bash
curl -sL https://src.w2k.sh/ffwebp/install.sh | sh
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
ffwebp -i input.jpg -o output.webp
```

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

## How It Works

- Input sniffing: the input codec is detected by reading magic bytes; if you provide an input filename, its extension is considered.
- Output selection: the output codec is inferred from the destination extension or forced via `--codec`.
- Timing and sizes: ffwebp prints info like decode/encode timings and output size unless `--silent` is set.

## License

See the [LICENSE](LICENSE) file.
