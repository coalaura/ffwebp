#!/usr/bin/env python3
"""Rebuild the committed corpus with independent encoders. See README.md."""
import argparse
import hashlib
import json
import io
import os
from pathlib import Path
import shutil
import struct
import subprocess
import tempfile

from PIL import Image, ImageDraw, features
import PIL
import tifffile

ROOT = Path(__file__).resolve().parent
SOURCE_SHA256 = "894d8eecadcdd8d11a6ccf5e67d5ceff05e5d632f8569274c6a4a23ca2bab6f2"
ENV = dict(os.environ, LC_ALL="C", TZ="UTC", OMP_NUM_THREADS="1", SOURCE_DATE_EPOCH="0")


def run(*args, stdin=None, env=None):
    result = subprocess.run([str(a) for a in args], input=stdin, capture_output=True,
                            env=ENV | (env or {}), timeout=120)
    if result.returncode:
        raise RuntimeError(f"{' '.join(map(str, args))}\n{result.stderr.decode(errors='replace')}")
    return result.stdout


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def normalize_avif_times(path):
    """libavif 1.0 writes wall-clock ISO BMFF track times; zero only those fields."""
    data = bytearray(path.read_bytes())

    def boxes(start, end):
        while start < end:
            size, kind = struct.unpack_from(">I4s", data, start)
            if size < 8 or start + size > end:
                raise ValueError("Unexpected AVIF box layout")
            if kind in (b"moov", b"trak", b"mdia"):
                boxes(start + 8, start + size)
            elif kind in (b"mvhd", b"tkhd", b"mdhd"):
                version = data[start + 8]
                if version not in (0, 1):
                    raise ValueError("Unexpected AVIF time field version")
                width = 8 if version else 4
                data[start + 12:start + 12 + 2 * width] = bytes(2 * width)
            start += size

    boxes(0, len(data))
    path.write_bytes(data)


def generate(out):
    inputs = out / "inputs"
    refs = out / "reference"
    inputs.mkdir()
    refs.mkdir()
    cases = []

    def reference(im):
        # Keep 16-bit grayscale intact; canonicalize other modes for Go's PNG reader.
        if im.mode not in ("I;16", "I;16B", "I"):
            im = im.convert("RGBA")
        target = refs / "pending.png"
        im.save(target, format="PNG", compress_level=9)
        name = sha(target)[:16] + ".png"
        target.rename(refs / name)
        return "reference/" + name

    def add(name, purpose, expected=None, tolerance=0, animated=False, loops=None):
        print(f"  {name}", flush=True)
        path = inputs / name
        if expected is None:
            with Image.open(path) as im:
                start = 1 if im.info.get("default_image") else 0
                frames = []
                delays = []
                for index in range(start, im.n_frames if animated else start + 1):
                    if animated or index:
                        im.seek(index)
                    frames.append(im.copy())
                    if animated:
                        delays.append(round(im.info["duration"]))
        else:
            frames = expected if isinstance(expected, list) else [expected]
            delays = []
        case = {"file": "inputs/" + name, "purpose": purpose,
                "reference": [reference(im) for im in frames], "tolerance": tolerance}
        if animated:
            case.update(delays=delays, loops=loops)
        cases.append(case)

    # Preserve the original independently of the small working photograph.
    source = ROOT / "sources/mars.png"
    if sha(source) != SOURCE_SHA256:
        raise RuntimeError("Mars source checksum changed; review the source before regenerating")
    with Image.open(source) as im:
        if im.size != (1200, 900) or im.getchannel("A").getextrema() != (255, 255):
            raise RuntimeError("Expected the original opaque 1200x900 Mars source")
        photo = im.convert("RGB").resize((120, 90), Image.Resampling.LANCZOS)
    photo.save(inputs / "mars-rgb.png")
    add("mars-rgb.png", "RGB photograph; shared small source", photo)

    # Odd dimensions expose padding/stride mistakes. Color under alpha is intentional.
    rgba = Image.new("RGBA", (63, 47))
    rgba.putdata([((x * 17 + y * 3) % 256, (y * 29) % 256, (x * 7) % 256,
                   (0, 85, 170, 255)[min(3, x // 16)]) for y in range(47) for x in range(63)])
    rgba.save(inputs / "alpha.png")
    add("alpha.png", "RGBA with zero, partial and full alpha; odd dimensions", rgba)
    gray = photo.convert("L")
    gray.save(inputs / "gray.png")
    add("gray.png", "8-bit grayscale", gray)
    gray16 = Image.new("I", (63, 47))
    gray16.putdata([(x * 997 + y * 541) % 65536 for y in range(47) for x in range(63)])
    gray16 = gray16.convert("I;16")
    gray16.save(inputs / "gray16.png")
    add("gray16.png", "16-bit grayscale with significant low bytes", gray16)
    palette = Image.new("P", (63, 47))
    palette.putpalette([255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 255] + [0] * 756)
    palette.putdata([(x // 16 + y // 12) % 4 for y in range(47) for x in range(63)])
    palette.save(inputs / "palette-alpha.png", transparency=bytes([0, 85, 170, 255]), bits=2)
    add("palette-alpha.png", "2-bit indexed PNG with tRNS alpha")
    palette.save(inputs / "palette.gif", transparency=0, optimize=False)
    add("palette.gif", "Single-frame indexed GIF with transparency")
    run("convert", inputs / "mars-rgb.png", "-interlace", "PNG", inputs / "interlaced.png")
    add("interlaced.png", "Adam7 interlacing", photo)

    for name, im, options in [
        ("mars-baseline.jpg", photo, dict(quality=90, subsampling=2)),
        ("mars-progressive.jpeg", photo, dict(quality=90, subsampling=0, progressive=True)),
        ("gray.jpg", gray, dict(quality=90)),
        ("cmyk.jpg", photo.convert("CMYK"), dict(quality=90)),
        ("mars.bmp", photo, {}), ("palette.bmp", palette, {}),
        ("mars.pcx", photo, {}), ("palette.pcx", palette, {}),
        ("alpha.qoi", rgba, {}), ("mars.qoi", photo, {}),
        ("alpha.tga", rgba, {}), ("alpha-rle.tga", rgba, dict(compression="tga_rle")),
        ("mars-lossy.webp", photo, dict(quality=90, method=6)),
        ("alpha-lossless.webp", rgba, dict(lossless=True, exact=True, method=6)),
    ]:
        im.save(inputs / name, **options)
        add(name, name.rsplit(".", 1)[0].replace("-", " "), tolerance=12 if name == "mars-baseline.jpg" else 3 if name.endswith(("jpg", "jpeg")) else 0)

    for name, im, compression in [
        ("rgb-uncompressed.tif", photo, "raw"), ("alpha-deflate.tiff", rgba, "tiff_adobe_deflate"),
        ("rgb-lzw.tiff", photo, "tiff_lzw"), ("rgb-jpeg.tiff", photo, "jpeg"),
        ("cmyk.tiff", photo.convert("CMYK"), "tiff_lzw"),
        ("gray16.tiff", gray16, "raw"),
        ("bilevel-group4.tiff", gray.point(lambda x: 255 if x >= 100 else 0, "1"), "group4"),
    ]:
        im.save(inputs / name, compression=compression)
        add(name, name.rsplit(".", 1)[0].replace("-", " "), tolerance=3 if compression == "jpeg" else 0)
    with Image.open(inputs / "bilevel-group4.tiff") as bilevel:
        bilevel.save(inputs / "bilevel-group4-white.tiff", compression="group4", tiffinfo={262: 0})
        add("bilevel-group4-white.tiff", "CCITT Group 4 with WhiteIsZero polarity", bilevel)
    photo.save(inputs / "multipage.tiff", save_all=True,
               append_images=[Image.new("RGB", photo.size, "magenta")], compression="tiff_lzw")
    add("multipage.tiff", "TIFF converts first page, not a document animation", photo)

    for ext, src in [("pbm", "gray.png"), ("pgm", "gray.png"), ("ppm", "mars-rgb.png"),
                     ("pam", "alpha.png"), ("pnm", "mars-rgb.png"), ("xbm", "gray.png"),
                     ("xpm", "palette-alpha.png"), ("psd", "mars-rgb.png"), ("psb", "mars-rgb.png")]:
        name = "pattern." + ext
        run("convert", inputs / src, "-strip", "-depth", "8", inputs / name)
        if ext in ("pam", "pnm", "psb", "xpm", "xbm"):
            decoded = run("convert", inputs / name, "png:-")
            add(name, ext + " independent ImageMagick encoding", Image.open(io.BytesIO(decoded)))
        else:
            add(name, ext + " independent ImageMagick encoding")
    for extension, source in [("pbm", "gray.png"), ("pgm", "gray.png"), ("ppm", "mars-rgb.png")]:
        run("convert", inputs / source, "-compress", "None", inputs / ("ascii." + extension))
        add("ascii." + extension, "PNM ASCII " + extension.upper())
    run("convert", inputs / "gray16.png", inputs / "gray16.pgm")
    add("gray16.pgm", "PNM 16-bit binary samples", gray16)

    icon = rgba.resize((32, 32), Image.Resampling.NEAREST)
    icon.save(inputs / "multi-png.ico", sizes=[(16, 16), (32, 32)])
    add("multi-png.ico", "Select largest entry; PNG payload", icon)
    icon.save(out / "icon.png")
    run("convert", out / "icon.png", inputs / "dib.ico")
    add("dib.ico", "ICO bitmap payload", icon)
    run("icotool", "--create", "--cursor", "--hotspot-x=5", "--hotspot-y=7",
        "--output", inputs / "cursor.cur", out / "icon.png")
    (out / "icon.png").unlink()
    add("cursor.cur", "Real CUR header and bitmap payload")

    (inputs / "alpha.ff").write_bytes(run("png2ff", stdin=(inputs / "alpha.png").read_bytes()))
    add("alpha.ff", "Farbfeld 16-bit RGBA", rgba)
    run("cjxl", inputs / "alpha.png", inputs / "alpha.jxl", "-d", "0", "-e", "3", "--num_threads=1")
    add("alpha.jxl", "JPEG XL lossless alpha", rgba)
    run("heif-enc", "-q", "90", "-p", "x265:pools=none", "-o", inputs / "mars.heic", inputs / "mars-rgb.png")
    run("heif-convert", inputs / "mars.heic", out / "heic.png")
    add("mars.heic", "HEVC in HEIF container (heic/heif aliases)", Image.open(out / "heic.png"), tolerance=3)
    (out / "heic.png").unlink()
    run("avifenc", "--jobs", "1", "--speed", "8", "--qcolor", "90", inputs / "mars-rgb.png", inputs / "mars.avif")
    run("avifdec", inputs / "mars.avif", out / "avif.png")
    add("mars.avif", "AV1 still image in AVIF container", Image.open(out / "avif.png"), tolerance=3)
    (out / "avif.png").unlink()

    # tifffile writes the TIFF/DNG directory; Pillow supplies a standalone JPEG strip.
    # This is deliberately a preview-only container, not a simulated RAW sensor image.
    preview = (inputs / "mars-baseline.jpg").read_bytes()
    for name, order in [("preview-le.dng", "<"), ("preview-be.dng", ">")]:
        tifffile.imwrite(inputs / name, iter([preview]), shape=(90, 120, 3), dtype="uint8",
                         photometric="ycbcr", compression="jpeg", rowsperstrip=90,
                         byteorder=order, metadata=None,
                         extratags=[(50706, "B", 4, (1, 4, 0, 0), False)])
        add(name, "Synthetic DNG JPEG preview; " + ("little" if order == "<" else "big") + " endian",
            Image.open(io.BytesIO(preview)), tolerance=12)

    shutil.copyfile(ROOT / "sources/shapes.svg", inputs / "shapes.svg")
    # Integer-aligned, opaque rectangles have an analytic reference (no renderer oracle).
    vector = Image.new("RGB", (63, 47), "white")
    draw = ImageDraw.Draw(vector)
    for box, fill in [((0, 0, 20, 46), "red"), ((21, 0, 41, 46), "lime"),
                      ((42, 0, 62, 46), "blue"), ((7, 9, 55, 37), "black"), ((13, 15, 49, 31), "white")]:
        draw.rectangle(box, fill=fill)
    add("shapes.svg", "Native vector shapes; no fonts, network or embedded raster", vector)

    # GIMP is the XCF writer. RGB/8-bit and legacy normal mode keep it readable by ffwebp.
    batch = f'''(let* ((img (car (gimp-file-load RUN-NONINTERACTIVE "{inputs / 'mars-rgb.png'}" "")))
                      (layer (car (gimp-image-get-active-layer img))))
                 (gimp-layer-set-mode layer NORMAL-MODE)
                 (gimp-xcf-save RUN-NONINTERACTIVE img layer "{inputs / 'mars.xcf'}" "mars.xcf")
                 (gimp-image-delete img))'''
    gimp_log = run("gimp", "--new-instance", "--no-interface", "--no-data", "--no-fonts", "--no-splash", "-b", batch, "-b", "(gimp-quit 0)", env={"GIMP2_DIRECTORY": str(out.parent / "gimp")})
    if not (inputs / "mars.xcf").exists():
        raise RuntimeError("GIMP did not create the XCF fixture: " + gimp_log.decode(errors="replace"))
    add("mars.xcf", "GIMP-authored RGB document with legacy normal layer", photo)

    # Four moving colored squares, transparent canvas and distinct delays. No downloaded GIF.
    frames = []
    for x, color in [(2, "red"), (10, "lime"), (18, "blue"), (26, "yellow")]:
        frame = Image.new("RGBA", (40, 24))
        ImageDraw.Draw(frame).rectangle((x, 6, x + 9, 15), fill=color)
        frames.append(frame)
    durations = [70, 130, 110, 190]
    gif_frames = []
    for frame in frames:
        # A fixed explicit palette prevents transparency from aliasing an opaque color.
        indexed = Image.new("P", frame.size)
        indexed.putpalette([0, 0, 0, 255, 0, 0, 0, 255, 0, 0, 0, 255, 255, 255, 0] + [0] * 753)
        indexed.putdata([0 if p[3] == 0 else {(255, 0, 0): 1, (0, 255, 0): 2,
                         (0, 0, 255): 3, (255, 255, 0): 4}[p[:3]] for p in frame.get_flattened_data()])
        gif_frames.append(indexed)
    for name, loop in [("disposal.gif", 2), ("single-play.gif", None)]:
        options = {} if loop is None else dict(loop=loop)
        gif_frames[0].save(inputs / name, save_all=True, append_images=gif_frames[1:],
                           duration=durations, disposal=[1, 3, 2, 1], transparency=0,
                           background=0, optimize=False, **options)
        add(name, "GIF partial frames, local palettes, transparency, disposal 1/3/2", animated=True,
            loops=-1 if loop is None else loop)
    for name, options, loop in [("animation.apng", {}, 0), ("poster.apng", dict(default_image=True), 3)]:
        start = Image.new("RGBA", frames[0].size, "magenta") if options else frames[0]
        rest = frames if options else frames[1:]
        start.save(inputs / name, format="PNG", save_all=True, append_images=rest,
                   duration=durations, loop=loop, disposal=[0, 2, 1, 0], blend=[0, 1, 1, 1], **options)
        add(name, "APNG blend/disposal" + ("; separate poster excluded" if options else ""),
            animated=True, loops=loop - 1 if loop else 0)
    frames[0].save(inputs / "animation.webp", save_all=True, append_images=frames[1:],
                   duration=durations, loop=0, lossless=True, exact=True, method=6)
    # libwebp can crop every frame to an opaque rectangle. Pillow then chooses RGB
    # from the VP8X alpha flag and loses transparent canvas pixels. Use the source frames.
    add("animation.webp", "Lossless animated WebP with alpha and unequal delays", frames, animated=True, loops=0)
    cases[-1]["delays"] = durations

    avif_args = ["avifenc", "--lossless", "--jobs", "1", "--speed", "8", "--timescale", "1000"]
    for index, (frame, duration) in enumerate(zip(frames, durations)):
        path = out / f"frame-{index}.png"
        frame.save(path)
        avif_args.extend(["--duration", str(duration), path])
    run(*avif_args, inputs / "animation.avif")
    normalize_avif_times(inputs / "animation.avif")
    add("animation.avif", "Independent AVIF sequence; millisecond timing, alpha", frames, animated=True, loops=0)
    cases[-1]["delays"] = durations
    for path in out.glob("frame-*.png"):
        path.unlink()

    for name, depth, color_type, interlace in [
        ("alpha.png", 8, 6, 0), ("gray16.png", 16, 0, 0),
        ("palette-alpha.png", 2, 3, 0), ("interlaced.png", 8, 2, 1),
    ]:
        header = (inputs / name).read_bytes()
        if (header[24], header[25], header[28]) != (depth, color_type, interlace):
            raise RuntimeError(f"Wrong PNG layout: {name}")
    if (inputs / "cursor.cur").read_bytes()[:6] != bytes([0, 0, 2, 0, 1, 0]):
        raise RuntimeError("CUR encoder produced an icon instead of a cursor")
    for name in ("disposal.gif", "single-play.gif"):
        with Image.open(inputs / name) as animation:
            disposals = []
            partial = False
            for index in range(animation.n_frames):
                animation.seek(index)
                disposals.append(animation.disposal_method)
                partial |= animation.dispose_extent != (0, 0, 40, 24)
            if disposals != [1, 3, 2, 1] or not partial:
                raise RuntimeError(f"GIF lost its disposal/partial-frame coverage: {name}")
    with Image.open(inputs / "poster.apng") as poster:
        if not poster.info.get("default_image") or poster.n_frames != 5:
            raise RuntimeError("APNG lost its separate poster image")

    for case in cases:
        if "delays" in case and (case["delays"] != durations or len(case["reference"]) != 4):
            raise RuntimeError(f"Unexpected generated animation timing: {case['file']}")

    versions = {"Pillow": PIL.__version__, "tifffile": tifffile.__version__,
                "pillow_codecs": {name: features.version(name) for name in ("jpg", "zlib", "libtiff", "webp")},
                "packages": run("dpkg-query", "-W", "-f=${Package}=${Version}\n",
                                "imagemagick", "gimp", "farbfeld", "libjxl-tools", "libavif-bin",
                                "libheif-examples", "libheif-plugin-x265", "libheif-plugin-libde265", "icoutils",
                                "libmagickcore-6.q16-7t64", "libjxl0.7", "libaom3", "libheif1",
                                "libde265-0", "libx265-199", "libpng16-16t64").decode().splitlines()}
    (out / "toolchain.json").write_text(json.dumps(versions, indent=2) + "\n")
    manifest = {"source_sha256": SOURCE_SHA256, "cases": cases}
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    files = sorted(p for p in out.rglob("*") if p.is_file())
    checksums = "".join(f"{sha(p)}  {p.relative_to(out).as_posix()}\n" for p in files)
    checksums += "".join(f"{sha(p)}  ../sources/{p.name}\n" for p in sorted((ROOT / "sources").glob("*")) if p.is_file())
    (out / "SHA256SUMS").write_text(checksums)
    print(f"Generated {len(cases)} fixtures, {sum(p.stat().st_size for p in files):,} bytes")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="regenerate in a temporary directory and compare every byte")
    args = parser.parse_args()
    for tool in ("convert", "png2ff", "cjxl", "heif-enc", "heif-convert", "avifenc", "avifdec", "icotool", "gimp"):
        if not shutil.which(tool):
            parser.error(f"Missing {tool}; use the fixture Dockerfile (see integration/README.md)")
    with tempfile.TemporaryDirectory(prefix="ffwebp-fixtures-", dir=ROOT) as temp:
        out = Path(temp) / "fixtures"
        out.mkdir()
        generate(out)
        destination = ROOT / "fixtures"
        if args.check:
            def contents(directory):
                return {p.relative_to(directory).as_posix(): sha(p) for p in directory.rglob("*") if p.is_file()}
            expected, actual = contents(destination), contents(out)
            changed = sorted(k for k in expected.keys() | actual.keys() if expected.get(k) != actual.get(k))
            if changed:
                raise SystemExit("Fixture drift: " + ", ".join(changed))
            print("All fixture bytes match")
        else:
            backup = Path(temp) / "old-fixtures"
            if destination.exists():
                destination.rename(backup)
            try:
                out.rename(destination)
            except OSError:
                if backup.exists():
                    backup.rename(destination)
                raise


if __name__ == "__main__":
    main()
