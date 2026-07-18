package common

import (
	_ "embed"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed font.ttf
var fontData []byte

var (
	FontBig    font.Face
	FontMedium font.Face
	FontSmall  font.Face
	FontTiny   font.Face
)

func InitFonts() error {
	f, err := opentype.Parse(fontData)
	if err != nil {
		return err
	}

	dpi := 72.0
	opts := &opentype.FaceOptions{
		Size:    48,
		DPI:     dpi,
		Hinting: font.HintingFull,
	}
	FontBig, err = opentype.NewFace(f, opts)
	if err != nil {
		return err
	}

	opts.Size = 36
	FontMedium, err = opentype.NewFace(f, opts)
	if err != nil {
		return err
	}

	opts.Size = 28
	FontSmall, err = opentype.NewFace(f, opts)
	if err != nil {
		return err
	}

	opts.Size = 20
	FontTiny, err = opentype.NewFace(f, opts)
	if err != nil {
		return err
	}

	return nil
}

func lineSpacing(face font.Face) float64 {
	metrics := face.Metrics()
	return float64(metrics.Height) / 64.0
}

// totalVisualHeight returns the total pixel height of N lines of text.
// text.Draw uses baseline y, so visual height = ascent + (N-1)*lineHeight + descent.
func totalVisualHeight(face font.Face, numLines int) float64 {
	metrics := face.Metrics()
	ls := float64(metrics.Height) / 64.0
	ascent := float64(metrics.Ascent) / 64.0
	descent := float64(metrics.Descent) / 64.0
	if numLines <= 0 {
		return 0
	}
	return ascent + ls*float64(numLines-1) + descent
}

// firstBaselineY returns the baseline y for the first line to center N lines around cy.
func firstBaselineY(face font.Face, cy float64, numLines int) float64 {
	metrics := face.Metrics()
	ascent := float64(metrics.Ascent) / 64.0
	th := totalVisualHeight(face, numLines)
	return cy - th/2 + ascent
}

func DrawTextCentered(screen *ebiten.Image, str string, cx, cy float64, face font.Face, clr color.Color) {
	lines := strings.Split(str, "\n")
	ls := lineSpacing(face)
	startY := firstBaselineY(face, cy, len(lines))

	for i, line := range lines {
		bounds := text.BoundString(face, line)
		x := int(cx) - (bounds.Min.X+bounds.Max.X)/2
		y := int(startY) + i*int(ls)
		text.Draw(screen, line, face, x, y, clr)
	}
}

func DrawTextLeft(screen *ebiten.Image, str string, x, cy float64, face font.Face, clr color.Color) {
	lines := strings.Split(str, "\n")
	ls := lineSpacing(face)
	startY := firstBaselineY(face, cy, len(lines))

	for i, line := range lines {
		y := int(startY) + i*int(ls)
		text.Draw(screen, line, face, int(x), y, clr)
	}
}

func MeasureText(str string, face font.Face) (float64, float64) {
	lines := strings.Split(str, "\n")
	ls := lineSpacing(face)
	maxW := 0.0
	for _, line := range lines {
		bounds := text.BoundString(face, line)
		if float64(bounds.Dx()) > maxW {
			maxW = float64(bounds.Dx())
		}
	}
	return maxW, ls*float64(len(lines))
}

var White = color.RGBA{255, 255, 255, 255}
var Black = color.RGBA{0, 0, 0, 255}
var Gray = color.RGBA{128, 128, 128, 255}
var DarkGray = color.RGBA{80, 80, 80, 255}
var LightGray = color.RGBA{200, 200, 200, 255}
var Red = color.RGBA{255, 0, 0, 255}
var Green = color.RGBA{0, 200, 0, 255}
var Blue = color.RGBA{0, 100, 255, 255}
var Yellow = color.RGBA{255, 255, 0, 255}
var Orange = color.RGBA{255, 165, 0, 255}

// PsychoPy coordinate system: height units (y up, centered)
// Screen: 1920x1080, center at (960,540)
// conversion: px = x*1080+960, py = -y*1080+540
func H(x, y float64) (float64, float64) {
	return x*1080 + 960, -y*1080 + 540
}

func HS(s float64) float64 {
	return s * 1080
}
