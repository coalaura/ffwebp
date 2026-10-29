package resize

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// Bound grid bookkeeping and the 8-byte-per-pixel composition canvas.
const (
	MaxJoinCells  = 10_000
	MaxJoinPixels = 32_000_000
)

type JoinOptions struct {
	Columns    int
	Rows       int
	CellWidth  int
	CellHeight int
	Mode       string
	AlignX     int
	AlignY     int
	Background color.NRGBA
}

func Join(images []image.Image, options JoinOptions) (image.Image, error) {
	widths, heights, err := joinDimensions(images, options)
	if err != nil {
		return nil, err
	}

	width, err := joinExtent(widths)
	if err != nil {
		return nil, err
	}

	height, err := joinExtent(heights)
	if err != nil {
		return nil, err
	}

	if width > MaxJoinPixels/height {
		return nil, fmt.Errorf("join canvas %dx%d exceeds the %d pixel limit; use smaller --join-cell dimensions", width, height, MaxJoinPixels)
	}

	// Preserve 16-bit sources as well as alpha when placing unscaled images.
	canvas := image.NewNRGBA64(image.Rect(0, 0, width, height))

	if options.Background.A != 0 {
		background := joinColor64(options.Background)

		// Fill straight samples directly to avoid premultiply/unpremultiply rounding.
		for row := range height {
			for column := range width {
				canvas.SetNRGBA64(column, row, background)
			}
		}
	}

	var (
		index int
		top   int
	)

	for _, cellHeight := range heights {
		left := 0

		for _, cellWidth := range widths {
			if index == len(images) {
				return canvas, nil
			}

			cell := image.Rect(left, top, left+cellWidth, top+cellHeight)

			placeJoinedImage(canvas, images[index], cell, options)

			index++
			left += cellWidth
		}

		top += cellHeight
	}

	return canvas, nil
}

func joinDimensions(images []image.Image, options JoinOptions) ([]int, []int, error) {
	if options.Columns <= 0 || options.Rows <= 0 || options.Columns > MaxJoinCells/options.Rows {
		return nil, nil, fmt.Errorf("join grid must contain between 1 and %d cells", MaxJoinCells)
	}

	if len(images) == 0 || len(images) > options.Columns*options.Rows {
		return nil, nil, errors.New("join requires at least one image and enough cells for all images")
	}

	if options.CellWidth < 0 || options.CellHeight < 0 || (options.CellWidth == 0) != (options.CellHeight == 0) {
		return nil, nil, errors.New("join cell width and height must both be positive or both automatic")
	}

	if options.AlignX < 0 || options.AlignX > 2 || options.AlignY < 0 || options.AlignY > 2 {
		return nil, nil, errors.New("invalid join alignment")
	}

	switch options.Mode {
	case "original", "fit", "cover", "stretch":
	default:
		return nil, nil, fmt.Errorf("invalid join mode %q", options.Mode)
	}

	widths := make([]int, options.Columns)
	heights := make([]int, options.Rows)

	var (
		maxWidth  int
		maxHeight int
	)

	for index, img := range images {
		if img == nil || img.Bounds().Empty() {
			return nil, nil, fmt.Errorf("join input %d is empty", index+1)
		}

		bounds := img.Bounds()

		column := index % options.Columns
		row := index / options.Columns

		if options.CellWidth > 0 && options.Mode == "original" && (bounds.Dx() > options.CellWidth || bounds.Dy() > options.CellHeight) {
			return nil, nil, fmt.Errorf("join input %d (%dx%d) exceeds cell %dx%d; use --join-mode fit, cover or a larger --join-cell", index+1, bounds.Dx(), bounds.Dy(), options.CellWidth, options.CellHeight)
		}

		widths[column] = max(widths[column], bounds.Dx())
		heights[row] = max(heights[row], bounds.Dy())

		maxWidth = max(maxWidth, bounds.Dx())
		maxHeight = max(maxHeight, bounds.Dy())
	}

	for index := range widths {
		if options.CellWidth > 0 {
			widths[index] = options.CellWidth
		} else if widths[index] == 0 {
			widths[index] = maxWidth
		}
	}

	for index := range heights {
		if options.CellHeight > 0 {
			heights[index] = options.CellHeight
		} else if heights[index] == 0 {
			heights[index] = maxHeight
		}
	}

	return widths, heights, nil
}

func joinExtent(sizes []int) (int, error) {
	total := 0

	for _, size := range sizes {
		if size > MaxJoinPixels-total {
			return 0, fmt.Errorf("join canvas exceeds the %d pixel limit; use smaller --join-cell dimensions", MaxJoinPixels)
		}

		total += size
	}

	return total, nil
}

func placeJoinedImage(canvas *image.NRGBA64, source image.Image, cell image.Rectangle, options JoinOptions) {
	bounds := source.Bounds()

	width := bounds.Dx()
	height := bounds.Dy()

	if options.Mode == "original" {
		left := cell.Min.X + (cell.Dx()-width)*options.AlignX/2
		top := cell.Min.Y + (cell.Dy()-height)*options.AlignY/2

		destination := image.Rect(left, top, left+width, top+height)

		if options.Background.A == 0 {
			copyJoinedImage(canvas, source, destination)
		} else {
			draw.Draw(canvas, destination, source, bounds.Min, draw.Over)
		}

		return
	}

	scaleX := float64(cell.Dx()) / float64(width)
	scaleY := float64(cell.Dy()) / float64(height)

	if options.Mode == "cover" {
		scale := max(scaleX, scaleY)
		left := float64(cell.Min.X) + (float64(cell.Dx())-float64(width)*scale)*float64(options.AlignX)/2
		top := float64(cell.Min.Y) + (float64(cell.Dy())-float64(height)*scale)*float64(options.AlignY)/2
		transform := f64.Aff3{scale, 0, left - float64(bounds.Min.X)*scale, 0, scale, top - float64(bounds.Min.Y)*scale}

		// Clip directly to the cell instead of allocating a potentially huge cover image.
		clipped := canvas.SubImage(cell).(*image.NRGBA64)

		draw.CatmullRom.Transform(clipped, transform, source, bounds, draw.Over, nil)

		return
	}

	destination := cell

	if options.Mode == "fit" {
		scale := min(scaleX, scaleY)

		width = min(cell.Dx(), max(1, int(math.Round(float64(width)*scale))))
		height = min(cell.Dy(), max(1, int(math.Round(float64(height)*scale))))

		left := cell.Min.X + (cell.Dx()-width)*options.AlignX/2
		top := cell.Min.Y + (cell.Dy()-height)*options.AlignY/2

		destination = image.Rect(left, top, left+width, top+height)
	}

	draw.CatmullRom.Scale(canvas, destination, source, bounds, draw.Over, nil)
}

func copyJoinedImage(canvas *image.NRGBA64, source image.Image, destination image.Rectangle) {
	// NRGBA64-to-NRGBA64 has a lossless draw.Src fast path; widen NRGBA explicitly
	// so original mode also preserves straight alpha samples from 8-bit inputs.
	if pixels, ok := source.(*image.NRGBA); ok {
		bounds := pixels.Bounds()

		for row := range bounds.Dy() {
			for column := range bounds.Dx() {
				pixel := pixels.NRGBAAt(bounds.Min.X+column, bounds.Min.Y+row)

				canvas.SetNRGBA64(destination.Min.X+column, destination.Min.Y+row, joinColor64(pixel))
			}
		}

		return
	}

	draw.Draw(canvas, destination, source, source.Bounds().Min, draw.Src)
}

func joinColor64(pixel color.NRGBA) color.NRGBA64 {
	return color.NRGBA64{R: uint16(pixel.R) * 257, G: uint16(pixel.G) * 257, B: uint16(pixel.B) * 257, A: uint16(pixel.A) * 257}
}
