package common

import (
	"bytes"
	"image/color"
	"io/fs"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	ScreenW = 1920
	ScreenH = 1080
)

func DrawRect(screen *ebiten.Image, x, y, w, h float64, clr color.Color) {
	ebitenutil.DrawRect(screen, x, y, w, h, clr)
}

func DrawRectOutline(screen *ebiten.Image, x, y, w, h float64, clr color.Color, lineWidth float64) {
	DrawRect(screen, x, y, w, lineWidth, clr)
	DrawRect(screen, x, y+h-lineWidth, w, lineWidth, clr)
	DrawRect(screen, x, y, lineWidth, h, clr)
	DrawRect(screen, x+w-lineWidth, y, lineWidth, h, clr)
}

func LoadImageFS(fsys fs.FS, name string) *ebiten.Image {
	data, err := fs.ReadFile(fsys, "resources/"+name)
	if err != nil {
		log.Printf("Warning: cannot load embedded image %s: %v", name, err)
		return ebiten.NewImage(1, 1)
	}
	img, _, err := ebitenutil.NewImageFromReader(bytes.NewReader(data))
	if err != nil {
		log.Printf("Warning: cannot decode embedded image %s: %v", name, err)
		return ebiten.NewImage(1, 1)
	}
	return img
}

func DrawImage(screen *ebiten.Image, img *ebiten.Image, x, y, w, h float64, alpha float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w/float64(img.Bounds().Dx()), h/float64(img.Bounds().Dy()))
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(float32(alpha))
	screen.DrawImage(img, op)
}

func DrawImageRotated(screen *ebiten.Image, img *ebiten.Image, x, y, w, h, angle float64, alpha float64) {
	op := &ebiten.DrawImageOptions{}
	cx, cy := float64(img.Bounds().Dx())/2, float64(img.Bounds().Dy())/2
	op.GeoM.Translate(-cx, -cy)
	op.GeoM.Scale(w/float64(img.Bounds().Dx()), h/float64(img.Bounds().Dy()))
	op.GeoM.Rotate(angle)
	op.GeoM.Translate(x+w/2, y+h/2)
	op.ColorScale.ScaleAlpha(float32(alpha))
	screen.DrawImage(img, op)
}

func DrawCircleFilled(screen *ebiten.Image, cx, cy, r float64, clr color.Color) {
	c := ebiten.NewImage(int(r*2), int(r*2))
	c.Fill(clr)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cx-r, cy-r)
	screen.DrawImage(c, op)
}

func PointInRect(px, py, rx, ry, rw, rh float64) bool {
	return px >= rx && px <= rx+rw && py >= ry && py <= ry+rh
}

func PointInCircle(px, py, cx, cy, r float64) bool {
	dx := px - cx
	dy := py - cy
	return dx*dx+dy*dy <= r*r
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func Abs(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}

func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
