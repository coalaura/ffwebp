# Failures exposed by the independent corpus

Baseline: `a36506ab318ce96aaa2e74903642470684c572ff` (2026-10-28).
These are **failing assertions**, not skips, expected-failure passes or relaxed
references. Production code is unchanged by the fixture work. Keep this change a
draft until these failures are resolved or the advertised support is narrowed.

Run them individually from the repository root:

```sh
go test -count=1 -v ./integration -run 'TestCLI_FixtureDecode/alpha.tga$'
```

Substitute a fixture name from the table. Each reference is recorded in
`fixtures/manifest.json`; use the referenced PNG for an independent visual check.

| Fixture | Expected | Current result and location |
|---|---|---|
| `alpha.tga` | Decode uncompressed RGBA TGA with sniffing enabled | Its first four bytes match CUR. `internal/codec/ico` assigns full confidence without validating the directory count, so the detector chooses ICO and errors. Decoding with the `.tga` extension hint works. The RLE companion passes. |
| `cursor.cur` | Decode a real type-2 cursor with a hotspot | The ICO adapter recognizes CUR magic but passes it to `ico.DecodeAll`, which accepts icons only. The dependency has a separate `cur` package. The previous committed `.cur` fixture was actually type-1 ICO, so it never exercised this path. |
| `alpha-lossless.webp` | Preserve visible RGB and partial alpha exactly | `coalaura/webp v1.6.6` returns straight `MODE_RGBA` bytes in an `image.RGBA` (premultiplied) object, including `DecodeAll`. Later conversion unpremultiplies them again. At (16,0), the authored straight color is (16,0,112,85). Opaque and binary-alpha animation fixtures pass. |
| `pattern.pam` | Preserve `RGB_ALPHA` | The PNM adapter calls `netpbm.Decode` with nil options. Its default `Target=PNM` explicitly removes alpha; `Target=PAM` accepts all supported input kinds unchanged. |
| `bilevel-group4.tiff` | Preserve black/white under PhotometricInterpretation=1 (BlackIsZero) | The current TIFF dependency decodes the bilevel image with inverted polarity. Pillow/libtiff and ImageMagick agree with the authored source. The WhiteIsZero companion passes, so this is not a generic Group 4 decoding failure. |
| `pattern.xpm` | Recognize standard named XPM colors such as blue and white | `coalaura/xpm v0.0.3` recognizes only `None` and `#RRGGBB`; other color strings silently become black. The independent ImageMagick encoder legitimately emits named colors. |

Two initially suspicious results were reference-tool differences, not ffwebp bugs:
Pillow's XBM bit polarity and its RGB-mode shortcut for a transparent animated WebP
canvas. Those references were corrected to ImageMagick and the authored lossless
animation frames respectively. JPEG uses bounded pixel differences because chroma
upsampling is not identical between implementations; this is documented in README.

## Validation of this change

On Linux amd64 with Go 1.26.4 and the recorded fixture tools:

- Regeneration followed by `generate.py --check`: all files byte-identical.
- Source and corpus checksums: pass.
- Fixture decoding: 54 pass; the six table entries above fail.
- All six animation cases, 23 encoder smoke cases, full-size Mars pixel check,
  existing adapter tests, and output-safety/filename regressions: pass.
- Full `go test -count=1 -v ./integration`: fails solely on those six decoder cases.
- The Docker recipe is pinned and documented, but was not built in the development
  environment (Docker was unavailable); the native equivalent was exercised.
