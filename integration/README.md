# Integration tests

The integration suite builds and runs the CLI with the `full` build tag. Input fixtures and reference pixels come from independent tools, not ffwebp's codecs. The corpus contains 60 fixtures and 28 deduplicated PNG references, totaling about 2.45 MB with the original sources.

## Run

From the repository root, with Go installed:

```sh
go test -count=1 -v ./integration
```

The suite builds its own temporary CLI binary. Docker and the fixture-generation tools are needed only to regenerate the corpus.

The HEIC fixture currently exceeds its pixel tolerance with `github.com/gen2brain/h265` v0.2.4: RGB output uses different chroma interpolation from the reference (maximum channel error 12/255 versus the allowed 3/255). The raw YCbCr output agrees within 1/255 when converted with Go's standard nearest-neighbor chroma sampling. The fixture and tolerance remain unchanged.

## Coverage

The suite checks fixture checksums, decoder pixels, animations, one encoder smoke test per output extension and focused adapter and CLI regressions. Per-file purposes, references and tolerances are recorded in [`fixtures/manifest.json`](fixtures/manifest.json).

| Family | Cases |
|---|---|
| JPEG | Baseline 4:2:0, progressive 4:4:4, grayscale, CMYK |
| PNG | RGB, RGBA including partial alpha, gray8, gray16, indexed with tRNS, Adam7 |
| GIF | Static transparency; partial frames, local palettes, disposal 1/3/2, finite repeats and single playback |
| APNG | Blending/disposal, infinite looping; separate poster and finite total plays |
| WebP | Lossy photo, lossless partial alpha, lossless animation |
| AVIF | Independent still and lossless animated encodings |
| JPEG XL | Lossless RGBA |
| TIFF | Uncompressed, Deflate, LZW, JPEG, CMYK, gray16, Group 4, multiple pages |
| PNM | P1-P7 via PBM/PGM/PPM/PAM/PNM; P1/P2/P3 ASCII and P5 16-bit |
| ICO / CUR | Multiple PNG icon sizes, DIB icon, type-2 cursor with hotspot |
| BMP / PCX / TGA | RGB and palette BMP/PCX; raw and RLE RGBA TGA |
| Other | Farbfeld, RGB/RGBA QOI, HEIC, PSD/PSB, XCF, SVG, XBM, named-color XPM |
| DNG | Little- and big-endian TIFF/DNG containers with an embedded JPEG strip |

Extension aliases such as `.heic`/`.heif` and `.tif`/`.tiff` do not require duplicate fixture blobs. DNG fixtures are **preview-only synthetic containers**, not camera RAW files. Layered PSD/XCF behavior, genuine camera DNGs, ICC/EXIF transforms and exhaustive codec options are outside this corpus.

### Pixel and animation checks

Each decoder produces a PNG read by Go's standard PNG decoder. Tests compare bounds and every visible premultiplied RGBA channel against the reference, ignoring hidden RGB under zero alpha. Exact cases allow no pixel error, including significant low bytes in gray16.

JPEG chroma interpolation varies between implementations. Baseline JPEG and its DNG previews allow a maximum channel error of 12/255; other lossy photo references allow 3/255. All nonzero-tolerance cases also require mean channel error of at most 1/255. These bounds measure decoder agreement, not lossy encoder quality.

Animation tests check every composited frame, unequal delays (70/130/110/190 ms), loop counts and selection at each frame/time boundary. A separate APNG poster must not appear as an animation frame. Encoder smoke tests run once per output extension; lossless and adapter checks run separately.

### Reference generation

References normally come from Pillow or native codec tools, with these exceptions and reproducibility details:

- ImageMagick supplies the XBM reference because Pillow uses the opposite black/white bit polarity. ImageMagick's black-foreground convention agrees with ffwebp's XBM library.
- Animated WebP uses the authored lossless source frames. Cropping can leave opaque encoded subframes on a transparent canvas; Pillow selects RGB from the file's alpha feature flag and drops the canvas alpha.
- libavif 1.0.4 writes the current time into animated ISO BMFF headers. The generator zeroes only creation/modification fields in `mvhd`, `tkhd` and `mdhd`, preserving codec data while making regeneration reproducible.

The generator also verifies PNG bit depth, color type and interlacing; CUR headers; GIF disposal and subrectangles; APNG poster separation; and animation frame counts and delays.

## Regenerate fixtures

Run these commands from the repository root in a POSIX shell (Linux, macOS or Windows/WSL):

```sh
docker build --platform linux/amd64 -f integration/Dockerfile -t ffwebp-fixtures .
docker run --rm --platform linux/amd64 --network none \
	--user "$(id -u):$(id -g)" -v "$PWD:/repo" ffwebp-fixtures
```

To regenerate into a temporary directory and compare every file without changing the corpus:

```sh
docker run --rm --platform linux/amd64 --network none \
	--user "$(id -u):$(id -g)" -v "$PWD:/repo" ffwebp-fixtures --check
```

Only the image build downloads dependencies. Generation is offline and needs neither Go nor an ffwebp binary. The Dockerfile pins the amd64 base image by digest, Ubuntu packages to a dated snapshot and Python 3.12 wheels by version and SHA-256. On Windows without WSL, use the equivalent bind mount syntax and omit `--user`.

For native Ubuntu generation, install the packages listed in [`Dockerfile`](Dockerfile) and [`requirements.txt`](requirements.txt) into the system and a Python 3.12 virtual environment, respectively, then run:

```sh
python3 integration/generate.py
python3 integration/generate.py --check
```

Different tool versions or architectures can produce different compressed bytes. [`fixtures/toolchain.json`](fixtures/toolchain.json) records the recipe's tools; [`fixtures/SHA256SUMS`](fixtures/SHA256SUMS) records output checksums. The generator verifies the Mars source checksum before building any fixture, then stages and checks the complete corpus before replacing `fixtures/`. Failed generation leaves the previous corpus in place.

## Sources

- [`sources/mars.png`](sources/mars.png): the supplied, unmodified 1200x900 RGBA PNG (1,372,884 bytes), with fully opaque alpha. SHA-256: `894d8eecadcdd8d11a6ccf5e67d5ceff05e5d632f8569274c6a4a23ca2bab6f2`. The full image has its own lossless CLI check; most photographic fixtures use a 120x90 Lanczos derivative.
- Procedural patterns in [`generate.py`](generate.py): odd-sized 63x47 RGB/alpha samples, a true 16-bit gray ramp, indexed colors and four 40x24 animation frames. They use no randomness, fonts, downloaded assets or platform rendering.
- [`sources/shapes.svg`](sources/shapes.svg): integer-aligned vector rectangles with an analytic raster reference, no embedded photograph and no external dependencies.

The repository owner supplied the Mars PNG without an original publication URL or image license. No new license is asserted for the photograph; retain its original credit and license when established. The procedural graphics and vector source were authored for this repository and follow its license.

## References

- [Pillow file-format documentation](https://pillow.readthedocs.io/en/stable/handbook/image-file-formats.html): frame disposal, APNG posters and save options.
- [libavif 1.0.4 tools](https://github.com/AOMediaCodec/libavif/tree/v1.0.4/apps): reference encoder and decoder implementations.
