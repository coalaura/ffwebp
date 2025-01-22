# ffwebp

A fast and flexible command-line image conversion tool supporting multiple formats and advanced compression options.

## Features

- Multiple input formats: AVIF, BMP, GIF, HEIC/HEIF, ICO, JPEG, JPEG XL, PNG, TIFF, WebP
- Multiple output formats: AVIF, BMP, GIF, ICO, JPEG, JPEG XL, PNG, TIFF, WebP
- Color quantization support
- Format-specific optimization options
- Silent mode for script integration
- Automatic format detection

## Installation

```bash
go install github.com/coalaura/ffwebp@latest
```

## Usage

Basic usage:
```bash
ffwebp [options] <input> [output]
```

If no output path is specified, the result will be written to stdout.

### Options

General options:
- `-h, --help`: Show help message
- `-s, --silent`: Do not print any output
- `-f, --format`: Output format (avif, bmp, gif, jpeg, jxl, png, tiff, webp, ico)

Common image options:
- `-q, --quality`: Quality level for AVIF/JPEG/JXL/WebP (1-100)
- `-c, --colors`: Number of colors for quantization (0=disabled, max 256)

WebP:
- `-x, --exact`: Preserve RGB values in transparent area
- `-l, --lossless`: Use lossless compression
- `-m, --method`: Encoder method (0=fast, 6=slower-better)

AVIF:
- `-r, --ratio`: YCbCr subsample ratio (0=444, 1=422, 2=420, 3=440, 4=411, 5=410)
- `-p, --speed`: Encoder speed (0=fast, 10=slower-better)

JPEG XL:
- `-e, --effort`: Encoder effort (0=fast, 10=slower-better)

PNG:
- `-g, --level`: Compression level (0=none, 1=speed, 2=best)

TIFF:
- `-t, --compression`: Compression type (0=none, 1=deflate, 2=lzw, 3=ccittgroup3, 4=ccittgroup4)

## Examples

Convert JPEG to WebP with 80% quality:
```bash
ffwebp -q 80 input.jpg output.webp
```

Convert PNG to WebP with lossless compression:
```bash
ffwebp -l input.png output.webp
```

Convert image to AVIF with custom subsample ratio:
```bash
ffwebp -f avif -r 2 -q 90 input.jpg output.avif
```

Quantize colors in output:
```bash
ffwebp -c 256 input.png output.png
```

## License

See the [LICENSE](LICENSE) file.
