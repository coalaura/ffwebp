# FFWebP

FFWebP is a small, single-binary CLI for converting images between formats, think "ffmpeg for images". It auto-detects the input format, lets you pick the output format by file extension or `--codec`, supports stdin/stdout, thumbnails and rich per-codec options.

## Features

- Single binary: no external tools required
- Auto-detects input codec and infers output from the file extension
- Supports AVIF, BMP, DNG (embedded JPEG preview only), Farbfeld, GIF, HEIF/HEIC, ICO/CUR, JPEG, JPEG XL, PCX, PNG/APNG, PNM (PBM/PGM/PPM/PAM), PSD (decode-only), QOI, SVG (decode-only), TGA, TIFF, WebP, XBM, XCF (decode-only) and XPM
- Lossy or lossless output with configurable quality
- Thumbnail generation via Lanczos3 resampling
- Join ordered inputs into image grids with automatic sizing, alignment and backgrounds
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
- APNG is part of the PNG adapter and is included with the `png`, `core` and `full` build tags.

## Usage

Basic conversion
```bash
ffwebp -i input.jpg output.webp
# or, equivalently
ffwebp -i input.jpg -o output.webp
```

The output file, directory or pattern can be supplied as a positional argument instead of `-o`/`--output`. Use only one form per command. If neither is supplied, output defaults to stdout.

File output is encoded into a temporary file beside the destination before publication. New files use mode `0644` filtered by the process umask; replacements preserve existing permission bits. `--skip-existing` never replaces an occupied destination, including one created during conversion. It prefers atomic no-replace rename (Linux/macOS/Windows) or hard-link publication. If neither is available, it exclusively creates the destination and copies the completed encoding: other readers can see a partial file during that copy, but existing files are never truncated. A failed copy removes its newly created output.

Batch destinations, including codec-driven extensions and split tiles, are resolved before workers start. Conflicting outputs and outputs that alias another job's input are rejected; replacing a job's own input is supported. Validation resolves existing symlinks and file aliases and folds Windows/macOS path case conservatively. It cannot protect against unrelated processes changing path aliases after validation. Sequence padding is a minimum width: `%03d` matches both `001` and `1000`, sorted numerically.

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
# Lossy quality (0-100, defaults to 85)
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

### Splitting images and output filenames

`--split COLSxROWS` writes tiles left to right, top to bottom. A `%d` output sequence numbers each tile, starting at `--start-number` (default 1). `%02d` pads with zeros to a minimum width of two; larger numbers are never truncated. With multiple inputs, numbering continues across inputs in their resolved order, regardless of worker completion order.

```bash
# image-01.jpeg, image-02.jpeg, image-03.jpeg
ffwebp --split 3x1 -i transmission.png "image-%02d.jpeg"

# tile-r00-c00.png, tile-r00-c01.png, tile-r01-c00.png, tile-r01-c01.png
ffwebp --split 2x2 -i image.png "tile-r{r:02d}-c{c:02d}.png"

# Keep each input's name and number its tiles from 00
ffwebp --split 3x1 -i "images/*.png" "out/{/.}-{i:02d}.png"

# Separate input numbering from tile numbering
ffwebp --split 3x1 -i "images/*.png" "out/input-{#:03d}-tile-{i:02d}.png"
```

Each numeric placeholder has one fixed meaning:

| Pattern | Meaning |
| --- | --- |
| `%d` / `%02d` | Output file number, continuous across the entire command |
| `{#}` / `{#:02d}` | Input number, shared by every output from that input |
| `{i}` / `{i:02d}` | Tile index within one input, starting at 0 |
| `{r}` / `{r:02d}` | Tile row, starting at 0 |
| `{c}` / `{c:02d}` | Tile column, starting at 0 |

- `--start-number` affects input and output sequences. Tile indices and coordinates reset for each input, remain zero-based and require `--split`.
- All numeric placeholders accept decimal printf formatting: `%02d` or `{i:02d}` for zero padding, `%3d` or `{r:3d}` for space padding and `%-3d` or `{c:-3d}` for left alignment. Signs and precision work too (`%+03d`, `{#:03d}`, `{i:.3d}`). Width and precision are limited to 1,000.
- Every explicit filename pattern is expanded as written, without added coordinate suffixes. Plain filenames such as `image.png` and directory outputs retain the automatic `image_r0_c0.png` naming when splitting.
- Patterns can combine filenames, input numbers, output numbers and tile coordinates. Duplicate destinations are rejected before writing files. For example, splitting into multiple tiles with `image-{#:02d}.png` fails because every tile from one input has the same input number; use `image-{#:02d}-{i:02d}.png` or `image-%02d.png` instead.

Run `ffwebp help patterns` for the full list of filename placeholders and more examples.

### Joining images

Repeat `-i` to select files in order. Each quoted glob or `%d` sequence expands in place; numeric filename parts sort naturally (`photo2` before `photo10` and `_r0_c2` before `_r0_c10`). Explicit paths keep their order and may be repeated. Positional arguments still specify the output, not additional inputs. Repeatable inputs also work for ordinary batch conversions.

```bash
# Join all inputs in one horizontal row (equivalent to --join auto or --join autox1)
ffwebp --join -i first.png -i second.png -i third.png strip.png

# Three images in a 2-column, 2-row grid; the bottom-right cell stays empty
ffwebp -i first.png -i second.png -i third.png --join 2x2 joined.png

# Three columns, enough rows for every matching image
ffwebp -i "tiles/*.png" --join 3xauto joined.png

# Mix explicit files and sequences; make a single horizontal strip
ffwebp -i cover.png -i "frames/%03d.png" --join autox1 strip.png

# Uniform cells, cropped to fill, with a white background
ffwebp -i "photos/*.jpg" --join 4xauto --join-cell 320x240 --join-mode cover --join-align top --join-background white contact-sheet.jpg

# Thumbnail and effects run on the completed grid
ffwebp -i left.png -i right.png --join 2x1 -t 800 joined.webp
```

- Bare `--join`, `--join auto` and `--join autox1` make one horizontal row. `--join 1xauto` makes one vertical column. `--join COLSxROWS` fills cells left to right, top to bottom, like `--split`. Either dimension may be `auto`, but not both. Fewer inputs leave background-filled cells; too many inputs produce an error suggesting a larger grid or `auto`.
- By default, each column uses its widest source and each row its tallest source. Entirely empty columns or rows use the largest populated column width or row height. `--join-cell WIDTHxHEIGHT` selects uniform pixel dimensions instead.
- `--join-mode original` (default) places images at their original size without cropping. An image larger than an explicit cell is an error. `fit` scales up or down to fit while preserving aspect ratio; `cover` scales to fill and crops the overflow; `stretch` scales to the exact cell size without preserving aspect ratio.
- `--join-align` defaults to `center`. It also accepts `top`, `bottom`, `left`, `right`, `top-left`, `top-right`, `bottom-left` and `bottom-right`. Alignment controls padding placement and which part of a cover image remains visible.
- `--join-background` accepts `transparent` (default), `white`, `black`, `#RGB`, `#RGBA`, `#RRGGBB` or `#RRGGBBAA`. Quote hex colors in shell commands. It fills padding and unused cells; transparent backgrounds are composited over white for output formats without alpha.
- Animated inputs use their first frame unless `--frame` or `--time` selects another. Joining precedes thumbnails and effects. `--play` previews the processed canvas.
- Joining requires one output filename or stdout (`-c png`, for example); output directories and per-input patterns are rejected. The destination must not alias an input. `--join` and `--split` cannot be combined and a failed input fails the entire join before output is written. Stdin can be selected once with `-i -`.
- Logs show the resolved order, zero-based row/column placements, sizing policy, final canvas dimensions and empty-cell count. Use `--silent` to suppress them. Grids are limited to 10,000 cells and canvases to 32 million pixels; reduce `--join-cell` if necessary.

Grid values may also be attached (`--join=2x2`). A following output filename stays positional when the grid is omitted; use `-o` or `--` to disambiguate an output name that itself looks like a grid. Flags requiring values report a missing value when followed by another option. For literal values beginning with a dash, use an equals sign (`-i=-photo.png`) or a relative path (`-i ./-photo.png`). Negative numeric values and `-` for stdin/stdout remain supported.

## Codec Options

Each codec exposes its available options via namespaced flags (for example, `--webp.method`, `--tiff.predictor`, `--psd.skip-merged`). Run `ffwebp --help` to see all global and codec-specific flags for your build.

If `--codec` is omitted, the output codec is chosen from the output file extension. If the output filename has no extension, pass `--codec`.

### Format limitations

- CUR output (`.cur` or `--codec cur`) uses a genuine type-2 cursor header and the encoder's default hotspot `(0, 0)`. Input hotspots are not preserved. ICO output retains its type-1 icon header.
- AVIF supports animated input and output, including `--frame`, `--time` and conversion to other animation-capable codecs. Frame durations are rounded to milliseconds. The pinned AVIF decoder does not expose loop counts, so decoded animations default to infinite looping. Its encoder cannot express single-play animations; those inputs produce an error instead of silently looping forever.
- JPEG XL currently uses the first frame. The pinned library exposes animated decoding but returns raw frame ticks without the animation tick rate or loop count, so reliable animation timing requires an upstream API change.
- HEIF/HEIC uses `github.com/gen2brain/h265/heic` for pure-Go decoding and encoding. Both `.heif` and `.heic` output HEIC still images with alpha support. Encoding uses 8-bit samples and 4:2:0 chroma with `--quality`; `--lossless` selects PCM coding and 4:4:4 chroma, but RGB-to-YCbCr conversion can still introduce rounding. Only the primary image is converted; image sequences are not exposed as animations.
- PNG uses `github.com/gen2brain/pngn`, preserving palette transparency, grayscale and 16-bit pixels with strict decoding defaults. `--png.compression` remains `0=none`, `1=default` (the default), `2=best speed`, `3=best compression`.
- APNG uses the same PNG adapter, flags and signature detection: `.png`, `.apng` and `--codec apng` support animated input/output, frame/time extraction, thumbnails, effects and conversion to other animation-capable codecs. Static input produces an ordinary PNG, even with an `.apng` filename. Decoded frames are already composited; a separate default PNG image is excluded from the animation. Encoding uses the first animation frame as the default image. Durations are converted from seconds to the nearest millisecond (half milliseconds round up), then back to seconds on output. Infinite looping, single playback and finite repetition counts are preserved by mapping APNG total plays to ffwebp's GIF-style repeat counts. The pinned encoder cannot represent every delay above 65.535 seconds; unsupported long delays produce an error instead of being silently shortened.
- TIFF uses `github.com/hhrutter/tiff`. Output supports uncompressed (`--tiff.compression 0`), Deflate (`1`, the default) and LZW (`2`). `--tiff.predictor` applies only to Deflate and LZW. Input supports classic TIFF, including JPEG-compressed and CMYK images, CCITT RLE, Group 3 1-D (including byte alignment) and Group 4. Only the first document page is converted; pages are not animation frames. BigTIFF, Group 3 2-D and CCITT uncompressed modes are unsupported. JPEG and CCITT are decode-only compression modes.
- DNG support extracts the embedded JPEG preview; it does not develop raw sensor data. Preview dimensions and quality can differ from the raw image and files without a supported JPEG preview cannot be converted. The adapter is included in `full` builds or with the `dng` build tag.
- SVG rendering uses resvg's PNG-returning API. Supplying both `--svg.width` and `--svg.height` renders directly at that size. Supplying just one dimension preserves the intrinsic aspect ratio, which can require a preliminary render with the pinned renderer.

## How It Works

- Input sniffing: the input codec is detected by reading magic bytes; if you provide an input filename, its extension is considered.
- Output selection: the output codec is inferred from the destination extension or forced via `--codec`.
- Timing and sizes: ffwebp prints info like decode/encode timings and output size unless `--silent` is set.

## License

See the [LICENSE](LICENSE) file.
