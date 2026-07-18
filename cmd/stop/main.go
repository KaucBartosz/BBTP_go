package main

import (
	"embed"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	_ "image/png"
)

//go:embed resources/*
var resources embed.FS

const (
	nTrials         = 50
	carSizeHU       = 0.28
	stopSizeHU      = 0.2
	carXHU          = 0.25
	carYBaseHU      = -0.3
	stopYHU         = 0.2
	distractSizeHU  = 0.15
	maxTrialSec     = 10.0
	timeoutSec      = 5.0
	freezeSec       = 1.0
	minStopDelay    = 1.0
	maxStopDelay    = 3.0
	distractSpeedLo = 0.3
	distractSpeedHi = 0.5
)

type trialResult struct {
	StopOnset  float64  `json:"stopOnset"`
	Responded  bool     `json:"responded"`
	RT         *float64 `json:"rt"`
	Correct    int      `json:"correct"`
	IsFalstart bool     `json:"isFalstart"`
}

type statsJSON struct {
	SredniCzasMs    float64 `json:"sredni_czas_ms"`
	PoprawneReakcje int     `json:"poprawne_reakcje"`
	WszystkieProby  int     `json:"wszystkie_proby"`
	Skutecznosc     float64 `json:"skutecznosc"`
	Reakcje         int     `json:"reakcje"`
	BledneReakcje   int     `json:"bledne_reakcje"`
	TotalClicks     int     `json:"totalClicks"`
	Falstarty       int     `json:"falstarty"`
}

type resultsJSON struct {
	TestID            string        `json:"testId"`
	SubjectID         string        `json:"subjectId"`
	Timestamp         string        `json:"timestamp"`
	IloscPoprawnych   int           `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych     int           `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc       int           `json:"ogolna_ilosc_nacisniec"`
	SredniCzasReakcji float64       `json:"sredni_czas_reakcji"`
	TotalClicks       int           `json:"totalClicks"`
	Score             string        `json:"score"`
	Statystyki        statsJSON     `json:"statystyki"`
	Wyniki            []trialResult `json:"wyniki"`
}

type game struct {
	state string

	bgImg      *ebiten.Image
	carImg     *ebiten.Image
	stopImg    *ebiten.Image
	jelonekImg *ebiten.Image
	drzewoImg  *ebiten.Image

	trialIndex int
	trialStart time.Time
	drawTime   float64

	stopDelay    float64
	stopAppeared bool
	stopXHU      float64
	stopOnsetSec float64
	stopOnsetT   time.Time

	responded  bool
	isFalstart bool
	rtMs       float64

	jelXHU, jelYHU float64
	drzXHU, drzYHU float64
	jelSpeed        float64
	drzSpeed        float64

	pauseStart time.Time

	results        []trialResult
	correctCount   int
	incorrectCount int
	respondedCount int
	totalClicks    int
	rtSum          float64
	rtCount        int
	falstarts      int

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
			g.startTrial()
			g.state = "trial"
		}
	case "trial":
		g.updateTrial()
	case "pause":
		if time.Since(g.pauseStart) >= time.Duration(freezeSec*float64(time.Second)) {
			g.advanceTrial()
		}
	case "done":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			return ebiten.Termination
		}
	}
	return nil
}

func (g *game) startTrial() {
	g.trialStart = time.Now()
	g.drawTime = 0
	g.stopDelay = minStopDelay + rand.Float64()*(maxStopDelay-minStopDelay)
	g.stopAppeared = false
	g.stopXHU = 0
	g.stopOnsetSec = 0
	g.responded = false
	g.isFalstart = false
	g.rtMs = 0

	if rand.Float64() < 0.5 {
		g.jelXHU = -0.55 + rand.Float64()*0.35
	} else {
		g.jelXHU = 0.45 + rand.Float64()*0.15
	}
	g.jelYHU = 0.6 + rand.Float64()*0.2
	g.jelSpeed = distractSpeedLo + rand.Float64()*(distractSpeedHi-distractSpeedLo)

	if rand.Float64() < 0.5 {
		g.drzXHU = -0.55 + rand.Float64()*0.35
	} else {
		g.drzXHU = 0.45 + rand.Float64()*0.15
	}
	g.drzYHU = 0.6 + rand.Float64()*0.2
	g.drzSpeed = distractSpeedLo + rand.Float64()*(distractSpeedHi-distractSpeedLo)
}

func (g *game) updateTrial() {
	now := time.Now()
	g.drawTime = now.Sub(g.trialStart).Seconds()
	elapsed := g.drawTime

	spaceJustPressed := inpututil.IsKeyJustPressed(ebiten.KeySpace)
	mouseJustPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if mouseJustPressed {
		g.totalClicks++
	}

	if !g.responded && (spaceJustPressed || mouseJustPressed) {
		mx, my := ebiten.CursorPosition()
		clickOnTarget := mouseJustPressed && (g.hitTestCar(float64(mx), float64(my)) || g.hitTestStop(float64(mx), float64(my)))

		if spaceJustPressed || clickOnTarget {
			if g.stopAppeared {
				g.responded = true
				g.isFalstart = false
				g.rtMs = now.Sub(g.stopOnsetT).Seconds() * 1000
				g.recordTrial(true)
				g.state = "pause"
				g.pauseStart = now
				return
			}
			g.responded = true
			g.isFalstart = true
			g.rtMs = 0
			g.recordTrial(false)
			g.falstarts++
			g.advanceTrial()
			return
		}
	}

	if g.stopAppeared && now.Sub(g.stopOnsetT).Seconds() >= timeoutSec {
		g.recordTrial(false)
		g.advanceTrial()
		return
	}

	if elapsed >= maxTrialSec {
		g.recordTrial(false)
		g.advanceTrial()
		return
	}

	if !g.stopAppeared && elapsed >= g.stopDelay {
		g.stopAppeared = true
		g.stopOnsetT = now
		g.stopOnsetSec = elapsed
		g.stopXHU = -0.6 + rand.Float64()*1.2
	}

	g.jelYHU -= g.jelSpeed / 60.0
	g.drzYHU -= g.drzSpeed / 60.0

	if g.jelYHU < -0.7 {
		g.jelYHU = 0.6 + rand.Float64()*0.2
		if rand.Float64() < 0.5 {
			g.jelXHU = -0.55 + rand.Float64()*0.35
		} else {
			g.jelXHU = 0.45 + rand.Float64()*0.15
		}
		g.jelSpeed = distractSpeedLo + rand.Float64()*(distractSpeedHi-distractSpeedLo)
	}
	if g.drzYHU < -0.7 {
		g.drzYHU = 0.6 + rand.Float64()*0.2
		if rand.Float64() < 0.5 {
			g.drzXHU = -0.55 + rand.Float64()*0.35
		} else {
			g.drzXHU = 0.45 + rand.Float64()*0.15
		}
		g.drzSpeed = distractSpeedLo + rand.Float64()*(distractSpeedHi-distractSpeedLo)
	}
}

func (g *game) hitTestCar(mx, my float64) bool {
	carYHU := carYBaseHU + math.Sin(g.drawTime*math.Pi*0.8)*0.05
	cx, cy := common.H(carXHU, carYHU)
	s := carSizeHU * 1080
	return common.PointInRect(mx, my, cx-s/2, cy-s/2, s, s)
}

func (g *game) hitTestStop(mx, my float64) bool {
	if !g.stopAppeared {
		return false
	}
	cx, cy := common.H(g.stopXHU, stopYHU)
	s := stopSizeHU * 1080
	return common.PointInRect(mx, my, cx-s/2, cy-s/2, s, s)
}

func (g *game) recordTrial(correct bool) {
	tr := trialResult{
		StopOnset:  g.stopOnsetSec,
		Responded:  g.responded,
		Correct:    boolToInt(correct),
		IsFalstart: g.isFalstart,
	}

	if g.responded && !g.isFalstart && correct {
		tr.RT = &g.rtMs
		g.correctCount++
		g.rtSum += g.rtMs
		g.rtCount++
	} else if g.isFalstart {
		g.incorrectCount++
	}

	if g.responded {
		g.respondedCount++
	}

	g.results = append(g.results, tr)
}

func (g *game) advanceTrial() {
	g.trialIndex++
	if g.trialIndex >= nTrials {
		g.writeResults()
		g.state = "done"
	} else {
		g.startTrial()
		g.state = "trial"
	}
}

func (g *game) writeResults() {
	var avgRT float64
	if g.rtCount > 0 {
		avgRT = g.rtSum / float64(g.rtCount)
	}

	skutecznosc := 0.0
	if nTrials > 0 {
		skutecznosc = float64(g.correctCount) / float64(nTrials) * 100
	}

	score := fmt.Sprintf("Poprawne: %d, Bledne: %d, Sredni RT: %.0f ms, Falstarty: %d",
		g.correctCount, g.incorrectCount, avgRT, g.falstarts)

	data := resultsJSON{
		TestID:            "Stop",
		SubjectID:         g.subjectId,
		Timestamp:         g.timestamp,
		IloscPoprawnych:   g.correctCount,
		IloscBlednych:     g.incorrectCount,
		OgolnaIlosc:       g.respondedCount,
		SredniCzasReakcji: avgRT,
		TotalClicks:       g.totalClicks,
		Score:             score,
		Statystyki: statsJSON{
			SredniCzasMs:    avgRT,
			PoprawneReakcje: g.correctCount,
			WszystkieProby:  nTrials,
			Skutecznosc:     skutecznosc,
			Reakcje:         g.respondedCount,
			BledneReakcje:   g.incorrectCount,
			TotalClicks:     g.totalClicks,
			Falstarty:       g.falstarts,
		},
		Wyniki: g.results,
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
			"Stop - Test Hamowania\n\n"+
				"Gdy zobaczysz znak STOP, nacisnij SPACE\n"+
				"lub kliknij na znak STOP lub samochod.\n\n"+
				"Nacisnij SPACE aby kontynuowac",
			cx, cy, common.FontMedium, common.White)
	case "trial", "pause":
		g.drawTrial(screen)
	case "done":
		common.DrawTextCentered(screen,
			"Test zakonczony!\n\nDziekuje za udzial.\n\nNacisnij SPACE aby zamknac.",
			cx, cy, common.FontMedium, common.Green)
	}
}

func (g *game) drawTrial(screen *ebiten.Image) {
	common.DrawImage(screen, g.bgImg, 0, 0, float64(common.ScreenW), float64(common.ScreenH), 1.0)

	dSize := distractSizeHU * 1080
	jelCx, jelCy := common.H(g.jelXHU, g.jelYHU)
	common.DrawImage(screen, g.jelonekImg, jelCx-dSize/2, jelCy-dSize/2, dSize, dSize, 1.0)

	drzCx, drzCy := common.H(g.drzXHU, g.drzYHU)
	common.DrawImage(screen, g.drzewoImg, drzCx-dSize/2, drzCy-dSize/2, dSize, dSize, 1.0)

	carYHU := carYBaseHU + math.Sin(g.drawTime*math.Pi*0.8)*0.05
	carCx, carCy := common.H(carXHU, carYHU)
	carPx := carSizeHU * 1080
	common.DrawImage(screen, g.carImg, carCx-carPx/2, carCy-carPx/2, carPx, carPx, 1.0)

	if g.stopAppeared {
		stopCx, stopCy := common.H(g.stopXHU, stopYHU)
		stopPx := stopSizeHU * 1080
		common.DrawImage(screen, g.stopImg, stopCx-stopPx/2, stopCy-stopPx/2, stopPx, stopPx, 1.0)
	}
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func main() {
	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	g := &game{
		state:      "instruction",
		subjectId:  common.RandomID(),
		timestamp:  common.Timestamp(),
		bgImg:      common.LoadImageFS(resources, "tlo.png"),
		carImg:     common.LoadImageFS(resources, "car.png"),
		stopImg:    common.LoadImageFS(resources, "stop.png"),
		jelonekImg: common.LoadImageFS(resources, "jelonek.png"),
		drzewoImg:  common.LoadImageFS(resources, "drzewo.png"),
	}

	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetWindowTitle("Stop - Test Hamowania")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
