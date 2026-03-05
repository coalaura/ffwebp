package resize

import (
	"errors"
	"image"
	"image/draw"
)

func Split(img image.Image, x, y int) ([]image.Image, error) {
	if x <= 0 || y <= 0 {
		return nil, errors.New("x and y must be > 0")
	}

	b := img.Bounds()

	srcW := b.Dx()
	srcH := b.Dy()

	if x > srcW || y > srcH {
		return nil, errors.New("x/y too large for image dimensions")
	}

	parts := make([]image.Image, 0, x*y)

	for row := range y {
		y0 := b.Min.Y + row*srcH/y
		y1 := b.Min.Y + (row+1)*srcH/y

		for col := range x {
			x0 := b.Min.X + col*srcW/x
			x1 := b.Min.X + (col+1)*srcW/x

			dst := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))

			draw.Draw(dst, dst.Bounds(), img, image.Point{X: x0, Y: y0}, draw.Src)

			parts = append(parts, dst)
		}
	}

	return parts, nil
}
