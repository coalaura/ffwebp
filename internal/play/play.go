//go:build play

package play

import (
	"errors"
	"image"
	"time"

	"github.com/coalaura/ffwebp/internal/codec"
	"github.com/hajimehoshi/ebiten/v2"
)

type player struct {
	anim       *codec.Animation
	static     *ebiten.Image
	frames     []*ebiten.Image
	currentIdx int
	lastUpdate time.Time
	accum      time.Duration
}

func (p *player) Update() error {
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)

	if ctrl && (ebiten.IsKeyPressed(ebiten.KeyQ) || ebiten.IsKeyPressed(ebiten.KeyW)) {
		return ebiten.Termination
	}

	if p.anim != nil && len(p.frames) > 1 {
		now := time.Now()
		delta := now.Sub(p.lastUpdate)

		p.lastUpdate = now
		p.accum += delta

		delay := time.Duration(p.anim.Delays[p.currentIdx]) * time.Millisecond
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}

		for p.accum >= delay {
			p.accum -= delay
			p.currentIdx = (p.currentIdx + 1) % len(p.frames)

			delay = time.Duration(p.anim.Delays[p.currentIdx]) * time.Millisecond
			if delay <= 0 {
				delay = 100 * time.Millisecond
			}
		}
	}

	return nil
}

func (p *player) Draw(screen *ebiten.Image) {
	if p.static != nil {
		screen.DrawImage(p.static, nil)
	} else if p.anim != nil && len(p.frames) > 0 {
		screen.DrawImage(p.frames[p.currentIdx], nil)
	}
}

func (p *player) Layout(outsideWidth, outsideHeight int) (int, int) {
	if p.static != nil {
		bounds := p.static.Bounds()

		return bounds.Dx(), bounds.Dy()
	}

	if p.anim != nil && len(p.frames) > 0 {
		bounds := p.frames[0].Bounds()

		return bounds.Dx(), bounds.Dy()
	}

	return 320, 240
}

func getInitialSize(dx, dy int) (int, int) {
	w := dx
	h := dy

	if w > 1280 {
		h = h * 1280 / w
		w = 1280
	}

	if h > 720 {
		w = w * 720 / h
		h = 720
	}

	return w, h
}

func PlayImage(img image.Image) error {
	bounds := img.Bounds()

	w, h := getInitialSize(bounds.Dx(), bounds.Dy())

	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowTitle("FFWebP Play")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	p := &player{
		static: ebiten.NewImageFromImage(img),
	}

	err := ebiten.RunGame(p)
	if errors.Is(err, ebiten.Termination) {
		return nil
	}

	return err
}

func PlayAnimation(anim *codec.Animation) error {
	if anim == nil || len(anim.Frames) == 0 {
		return errors.New("no frames to play")
	}

	frames := make([]*ebiten.Image, len(anim.Frames))

	for i, f := range anim.Frames {
		frames[i] = ebiten.NewImageFromImage(f)
	}

	bounds := anim.Frames[0].Bounds()

	w, h := getInitialSize(bounds.Dx(), bounds.Dy())

	ebiten.SetWindowSize(w, h)
	ebiten.SetWindowTitle("FFWebP Play")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	p := &player{
		anim:       anim,
		frames:     frames,
		lastUpdate: time.Now(),
	}

	err := ebiten.RunGame(p)
	if errors.Is(err, ebiten.Termination) {
		return nil
	}

	return err
}
