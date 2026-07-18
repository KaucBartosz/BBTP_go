package main

import (
	"embed"
	"BBTP_go/common"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

//go:embed resources/*
var resources embed.FS

const (
	appearanceRate     = 107.0
	timeBetweenCircles = 60.0 / appearanceRate
	circleSizeHU       = 0.15
	carWidthHU         = 0.3
	carHeightHU        = 0.15
)

type trialData struct {
	CircleID int      `json:"circleId"`
	Correct  int      `json:"correct"`
	Miss     int      `json:"miss"`
	RT       *float64 `json:"rt"`
}

type results struct {
	TestID               string      `json:"testId"`
	SubjectID            string      `json:"subjectId"`
	Timestamp            string      `json:"timestamp"`
	IloscPoprawnych      int         `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych        int         `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc          int         `json:"ogolna_ilosc_nacisniec"`
	SredniCzasReakcji    float64     `json:"sredni_czas_reakcji"`
	KliknieciaBezKolka   int         `json:"klikniecia_bez_kolka"`
	Score                string      `json:"score"`
	Wyniki               []trialData `json:"wyniki"`
	SelectedDuration     int         `json:"selectedDuration"`
}

type circle struct {
	id        int
	x, y      float64
	onsetTime float64
}

type Game struct {
	state string

	duration      int
	carX, carY    float64
	carImg        *ebiten.Image
	circleImg     *ebiten.Image

	currentCircle      *circle
	circleID           int
	correctCount       int
	missCount          int
	clicksWithoutCircle int
	totalResponses     int
	rtSum              float64
	nextCircleTime     float64
	waitingForRelease  bool
	wyniki             []trialData

	prevMousePressed bool
	trialClock       float64
	escaped          bool
	startTime        time.Time
	startupTicks int
}

func main() {
	if err := common.InitFonts(); err != nil {
		panic(err)
	}

	g := &Game{
		state:      "duration",
		carX:       0,
		carY:       -0.35,
		carImg:     common.LoadImageFS(resources, "car.png"),
		nextCircleTime: timeBetweenCircles,
	}

	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetWindowTitle("Złap Sygnał")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}

func (g *Game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	switch g.state {
	case "duration":
		return g.updateDuration()
	case "instruction":
		return g.updateInstruction()
	case "test":
		return g.updateTest()
	case "done":
		return g.updateDone()
	}
	return nil
}

func (g *Game) updateDuration() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.duration = 30
		g.state = "instruction"
	} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.duration = 60
		g.state = "instruction"
	} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
		g.duration = 90
		g.state = "instruction"
	} else if inpututil.IsKeyJustPressed(ebiten.Key4) {
		g.duration = 120
		g.state = "instruction"
	}
	return nil
}

func (g *Game) updateInstruction() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.state = "test"
		g.trialClock = 0
		g.startTime = time.Now()
		g.nextCircleTime = timeBetweenCircles
	}
	return nil
}

func (g *Game) updateTest() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.escaped = true
		g.state = "done"
		return nil
	}

	g.trialClock += 1.0 / 60.0
	elapsed := g.trialClock

	if g.duration > 0 && elapsed >= float64(g.duration) {
		g.state = "done"
		return nil
	}

	isPressedNow := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	isNewClick := isPressedNow && !g.prevMousePressed
	g.prevMousePressed = isPressedNow

	if g.waitingForRelease && !isPressedNow {
		g.waitingForRelease = false
		g.nextCircleTime = elapsed + timeBetweenCircles
	}

	if g.currentCircle != nil {
		if elapsed >= g.currentCircle.onsetTime+timeBetweenCircles*1.5 {
			g.missCount++
			g.wyniki = append(g.wyniki, trialData{
				CircleID: g.currentCircle.id,
				Correct:  0,
				Miss:     1,
				RT:       nil,
			})
			g.currentCircle = nil
			g.nextCircleTime = elapsed + timeBetweenCircles
		}
	}

	if g.currentCircle == nil && !g.waitingForRelease && elapsed >= g.nextCircleTime {
		g.circleID++
		rx := (rand.Float64() - 0.5) * 1.0
		ry := -0.10 + rand.Float64()*0.53
		g.currentCircle = &circle{
			id:        g.circleID,
			x:         rx,
			y:         ry,
			onsetTime: elapsed,
		}
	}

	carPx, carPy := common.H(g.carX, g.carY)
	carW := common.HS(carWidthHU)
	carH := common.HS(carHeightHU)

	if g.currentCircle != nil && isNewClick {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)
		if fmx >= carPx-carW/2 && fmx <= carPx+carW/2 && fmy >= carPy-carH/2 && fmy <= carPy+carH/2 {
			rt := elapsed - g.currentCircle.onsetTime
			g.correctCount++
			g.totalResponses++
			g.rtSum += rt
			rtCopy := rt
			g.wyniki = append(g.wyniki, trialData{
				CircleID: g.currentCircle.id,
				Correct:  1,
				Miss:     0,
				RT:       &rtCopy,
			})
			g.currentCircle = nil
			g.waitingForRelease = true
		}
	}

	if isNewClick && g.currentCircle == nil {
		mx, my := ebiten.CursorPosition()
		fmx, fmy := float64(mx), float64(my)
		if fmx >= carPx-carW/2 && fmx <= carPx+carW/2 && fmy >= carPy-carH/2 && fmy <= carPy+carH/2 {
			g.clicksWithoutCircle++
		}
	}

	return nil
}

func (g *Game) updateDone() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.writeResults()
		return ebiten.Termination
	}
	return nil
}

func (g *Game) writeResults() {
	totalCircles := g.correctCount + g.missCount
	avgRT := 0.0
	if g.totalResponses > 0 {
		avgRT = math.Round((g.rtSum / float64(g.totalResponses)) * 1000)
	}
	r := results{
		TestID:             "ZlapSygnal",
		SubjectID:          common.RandomID(),
		Timestamp:          common.Timestamp(),
		IloscPoprawnych:    g.correctCount,
		IloscBlednych:      g.missCount,
		OgolnaIlosc:        totalCircles,
		SredniCzasReakcji:  avgRT,
		KliknieciaBezKolka: g.clicksWithoutCircle,
		Score:              fmt.Sprintf("Poprawne: %d | Misses: %d | Bez kółka: %d | Łącznie: %d | Śr. RT: %.0f ms", g.correctCount, g.missCount, g.clicksWithoutCircle, totalCircles, avgRT),
		Wyniki:             g.wyniki,
		SelectedDuration:   g.duration,
	}
	if err := common.WriteResults(".", r); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)

	switch g.state {
	case "duration":
		common.DrawTextCentered(screen,
			"Wybierz czas trwania testu:\n\n"+
				"1 - 30 sekund\n"+
				"2 - 60 sekund\n"+
				"3 - 90 sekund\n"+
				"4 - 120 sekund",
			common.ScreenW/2, common.ScreenH/2, common.FontMedium, common.White)
	case "instruction":
		common.DrawTextCentered(screen,
			"Na ekranie w krótkich odstępach czasu pojawiać się będzie czerwone kółko.\n"+
				"Twoim zadaniem jest, za pomocą MYSZY, kliknąć na samochód za każdym razem, gdy pojawi się nowe kółko.\n"+
				"Staraj się reagować najszybciej jak potrafisz.\n\n"+
				"Aby rozpocząć zadanie, wciśnij SPACJĘ.",
			common.ScreenW/2, common.ScreenH/2, common.FontMedium, common.White)
	case "test":
		g.drawTest(screen)
	case "done":
		common.DrawTextCentered(screen,
			"KONIEC TESTU\n\n"+
				fmt.Sprintf("Poprawne: %d | Misses: %d", g.correctCount, g.missCount),
			common.ScreenW/2, common.ScreenH/2, common.FontMedium, common.White)
	}
}

func (g *Game) drawTest(screen *ebiten.Image) {
	if g.currentCircle != nil {
		cx, cy := common.H(g.currentCircle.x, g.currentCircle.y)
		r := common.HS(circleSizeHU / 2)
		ebitenutil.DrawCircle(screen, cx, cy, r, common.Red)
	}

	carPx, carPy := common.H(g.carX, g.carY)
	carW := common.HS(carWidthHU)
	carH := common.HS(carHeightHU)
	common.DrawImage(screen, g.carImg, carPx-carW/2, carPy-carH/2, carW, carH, 1.0)

	elapsed := time.Since(g.startTime).Seconds()
	remaining := float64(g.duration) - elapsed
	if remaining < 0 {
		remaining = 0
	}
	common.DrawTextCentered(screen, fmt.Sprintf("%.0fs", remaining),
		common.ScreenW/2, 50, common.FontSmall, common.White)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return common.ScreenW, common.ScreenH
}
