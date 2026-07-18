package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

//go:embed resources/*
var resources embed.FS

const (
	mapScale       = 1.6
	mapAspect      = 0.555
	mapHalfAspect  = mapAspect / 2
	carW_HU        = 0.03
	carH_HU        = 0.05
	carHitboxOffY  = -0.012
	moveSpeed      = 0.007
	freezeDur      = 0.5
	metaDur        = 2.0
	fallbackStartX = -0.336
	fallbackStartY = -0.322
)

type statsJSON struct {
	Trasa   string `json:"trasa"`
	Kolizje int    `json:"kolizje"`
	CzasMs  int    `json:"czas_trwania_ms"`
}

type resultsJSON struct {
	TestID                string     `json:"testId"`
	SubjectID             string     `json:"subjectId"`
	Timestamp             string     `json:"timestamp"`
	PoziomTrudnosci       string     `json:"poziom_trudnosci"`
	IloscPoprawnych       int        `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych         int        `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc           int        `json:"ogolna_ilosc_nacisniec"`
	CzasPokonaniaTrasySec int        `json:"czas_pokonania_trasy_sek"`
	Score                 string     `json:"score"`
	Statystyki            statsJSON  `json:"statystyki"`
	Wyniki                []struct{} `json:"wyniki"`
}

type game struct {
	state string

	trackEbiten *ebiten.Image
	trackPixels image.Image
	carEbiten   *ebiten.Image

	imgW, imgH int

	carX, carY     float64
	startX, startY float64
	carAngle       float64

	collisionCount int
	startTime      time.Time
	freezeTimer    float64
	metaStart      time.Time

	difficulty string

	subjectId string
	timestamp string

	startupTicks int
}

func (g *game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.writeResults()
		return ebiten.Termination
	}

	switch g.state {
	case "instruction":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.state = "difficulty"
		}
	case "difficulty":
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.startGame("1")
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.startGame("2")
		}
	case "playing":
		g.updatePlaying()
	case "meta":
		if time.Since(g.metaStart) >= time.Duration(metaDur*float64(time.Second)) {
			g.writeResults()
			g.state = "done"
		}
	case "done":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			return ebiten.Termination
		}
	}
	return nil
}

func (g *game) startGame(diff string) {
	g.difficulty = diff
	trackFile := "trasa.png"
	if diff == "2" {
		trackFile = "trasa2.png"
	}

	data, err := resources.ReadFile(trackFile)
	if err != nil {
		log.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}

	g.trackPixels = img
	g.imgW = img.Bounds().Dx()
	g.imgH = img.Bounds().Dy()
	g.trackEbiten = ebiten.NewImageFromImage(img)

	g.carEbiten = common.LoadImageFS(resources, "sam.png")

	g.startX, g.startY = g.findGreenStart()
	g.carX, g.carY = g.startX, g.startY
	g.carAngle = 0
	g.collisionCount = 0
	g.startTime = time.Now()
	g.freezeTimer = 0
	g.state = "playing"
}

func (g *game) findGreenStart() (float64, float64) {
	var sumX, sumY float64
	var count int

	for y := 0; y < g.imgH; y++ {
		for x := 0; x < g.imgW; x++ {
			r, gg, b, _ := g.trackPixels.At(x, y).RGBA()
			r8 := uint8(r >> 8)
			g8 := uint8(gg >> 8)
			b8 := uint8(b >> 8)
			if r8 < 50 && g8 > 200 && b8 < 50 {
				sumX += float64(x)
				sumY += float64(y)
				count++
			}
		}
	}

	if count > 0 {
		avgX := sumX / float64(count)
		avgY := sumY / float64(count)
		sx := (avgX/float64(g.imgW) - 0.5) * mapScale
		sy := (mapHalfAspect - avgY/float64(g.imgW)) * mapScale
		return sx, sy
	}

	return fallbackStartX, fallbackStartY
}

func (g *game) isOnTrack(x, y float64) bool {
	relX := x / mapScale
	relY := y / mapScale
	px := int((relX + 0.5) * float64(g.imgW))
	py := int((mapHalfAspect - relY) * float64(g.imgW))

	if px < 0 || px >= g.imgW || py < 0 || py >= g.imgH {
		return false
	}

	r, gg, b, _ := g.trackPixels.At(px, py).RGBA()
	return uint8(r>>8) > 50 || uint8(gg>>8) > 50 || uint8(b>>8) > 50
}

func (g *game) isAtFinish(x, y float64) bool {
	relX := x / mapScale
	relY := y / mapScale
	px := int((relX + 0.5) * float64(g.imgW))
	py := int((mapHalfAspect - relY) * float64(g.imgW))

	if px < 0 || px >= g.imgW || py < 0 || py >= g.imgH {
		return false
	}

	r, gg, b, _ := g.trackPixels.At(px, py).RGBA()
	return uint8(r>>8) > 150 && uint8(gg>>8) < 100 && uint8(b>>8) < 100
}

func (g *game) updatePlaying() {
	if g.freezeTimer > 0 {
		g.freezeTimer -= 1.0 / 60.0
		return
	}

	var dx, dy float64
	if ebiten.IsKeyPressed(ebiten.KeyLeft) {
		dx -= 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) {
		dx += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) {
		dy += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) {
		dy -= 1
	}

	if dx != 0 || dy != 0 {
		length := math.Sqrt(dx*dx + dy*dy)
		g.carX += (dx / length) * moveSpeed
		g.carY += (dy / length) * moveSpeed
		g.carAngle = math.Atan2(dx, dy)
	}

	halfW := carW_HU / 2
	halfH := carH_HU / 2
	corners := [4][2]float64{
		{-halfW, -halfH + carHitboxOffY},
		{halfW, -halfH + carHitboxOffY},
		{-halfW, halfH + carHitboxOffY},
		{halfW, halfH + carHitboxOffY},
	}

	for _, c := range corners {
		rx := g.carX + c[0]*math.Cos(g.carAngle) + c[1]*math.Sin(g.carAngle)
		ry := g.carY - c[0]*math.Sin(g.carAngle) + c[1]*math.Cos(g.carAngle)

		if g.isAtFinish(rx, ry) {
			g.state = "meta"
			g.metaStart = time.Now()
			return
		}
		if !g.isOnTrack(rx, ry) {
			g.carX, g.carY = g.startX, g.startY
			g.carAngle = 0
			g.collisionCount++
			g.freezeTimer = freezeDur
			return
		}
	}
}

func (g *game) writeResults() {
	duration := time.Since(g.startTime).Seconds()
	correct := 0
	if duration > 0 {
		correct = 1
	}

	score := fmt.Sprintf("Trasa %s | Kolizje: %d | Czas: %ds",
		g.difficulty, g.collisionCount, int(math.Round(duration)))

	data := resultsJSON{
		TestID:                "samochodzik",
		SubjectID:             g.subjectId,
		Timestamp:             g.timestamp,
		PoziomTrudnosci:       g.difficulty,
		IloscPoprawnych:       correct,
		IloscBlednych:         g.collisionCount,
		OgolnaIlosc:           correct + g.collisionCount,
		CzasPokonaniaTrasySec: int(math.Round(duration)),
		Score:                 score,
		Statystyki: statsJSON{
			Trasa:   g.difficulty,
			Kolizje: g.collisionCount,
			CzasMs:  int(math.Round(duration * 1000)),
		},
		Wyniki: []struct{}{},
	}

	if err := common.WriteResults(".", data); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)
	cx := float64(common.ScreenW) / 2
	cy := float64(common.ScreenH) / 2

	switch g.state {
	case "instruction":
		common.DrawTextCentered(screen,
			"Samochodzik - Test Nawigacji\n\n"+
				"Twoim zadaniem bedzie przejechanie labiryntu.\n"+
				"Za pomocia strzalek na klawiaturze, pokieruj samochodem do mety.\n"+
				"Staraj sie dokladnie kierowac samochodem, aby nie wyjechac\n"+
				"poza krawedz labiryntu.\n"+
				"Wyjechanie poza krawedz spowoduje powrot samochodu na start.\n\n"+
				"Nacisnij SPACJE aby wybrac trase.",
			cx, cy, common.FontMedium, common.White)
	case "difficulty":
		common.DrawTextCentered(screen,
			"WYBIERZ TRASE:\n\n"+
				"1 - Klasyczna (Latwa)\n"+
				"2 - Labirynt (Trudna)\n\n"+
				"Nacisnij 1 lub 2",
			cx, cy, common.FontMedium, common.White)
	case "playing", "meta":
		g.drawPlaying(screen)
		if g.state == "meta" {
			common.DrawTextCentered(screen, "META!", cx, cy, common.FontBig, common.Green)
		}
	case "done":
		common.DrawTextCentered(screen,
			"Test zakonczony!\n\n"+
				fmt.Sprintf("Kolizje: %d\n", g.collisionCount)+
				fmt.Sprintf("Czas: %ds\n\n", int(math.Round(time.Since(g.startTime).Seconds())))+
				"Nacisnij SPACE aby zamknac.",
			cx, cy, common.FontMedium, common.Green)
	}
}

func (g *game) drawPlaying(screen *ebiten.Image) {
	trackW := mapScale * float64(common.ScreenH)
	trackH := mapScale * mapAspect * float64(common.ScreenH)
	trackX := (float64(common.ScreenW) - trackW) / 2
	trackY := (float64(common.ScreenH) - trackH) / 2

	common.DrawImage(screen, g.trackEbiten, trackX, trackY, trackW, trackH, 1.0)

	carPxW := carW_HU * float64(common.ScreenH)
	carPxH := carH_HU * float64(common.ScreenH)
	carCx, carCy := common.H(g.carX, g.carY)

	alpha := 1.0
	if g.freezeTimer > 0 {
		alpha = 0.5
	}

	common.DrawImageRotated(screen, g.carEbiten,
		carCx-carPxW/2, carCy-carPxH/2,
		carPxW, carPxH,
		g.carAngle, alpha)
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}

func main() {
	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	g := &game{
		state:     "instruction",
		subjectId: common.RandomID(),
		timestamp: common.Timestamp(),
	}

	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetWindowTitle("Samochodzik - Test Nawigacji")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}