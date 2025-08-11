# FFWebP

FFWebP is a command line utility for converting images between multiple formats. It automatically detects the input format and encodes the output using

## Features

- Pure Go implementation with no external runtime dependencies
- Supports AVIF, BMP, GIF, ICO, JPEG, JPEGXL, PNG, PNM (PBM/PGM/PPM/PAM), PSD (no encoding), TGA, TIFF and WebP
- Lossy or lossless output with configurable quality
- Output codec selected from the output file extension when `--codec` is omitted
- Full set of format-specific flags for every supported format (see `ffwebp --help`)
- Additional formats may be added in the future

## Building

Compile with all codecs enabled using the `full` build tag:

```bash
go build -tags full -o ffwebp ./cmd/ffwebp
```

You can enable a subset of codecs by selecting the appropriate build tags (for example `-tags "jpeg,png"`).

## Usage

```bash
ffwebp -i input.jpg -o output.webp
```

Run `ffwebp --help` to see the full list of flags.

## License

See the [LICENSE](LICENSE) file.
