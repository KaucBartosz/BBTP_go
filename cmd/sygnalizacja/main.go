package main

import (
	"embed"
	"fmt"
	"log"
	"math/rand"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

//go:embed resources/*
var resources embed.FS

const (
	numTrials    = 50
	greenOnset   = 1.0
	trialTimeout = 3.0
)

type trialResult struct {
	CorrectSide string  `json:"correct_side"`
	ClickedSide string  `json:"clicked_side"`
	RT          float64 `json:"rt"`
	Outcome     string  `json:"outcome"`
}

type resultsData struct {
	TestID           string        `json:"testId"`
	SubjectID        string        `json:"subjectId"`
	Timestamp        string        `json:"timestamp"`
	CorrectPresses   int           `json:"ilosc_poprawnych_nacisniec"`
	IncorrectPresses int           `json:"ilosc_blednych_nacisniec"`
	TotalPresses     int           `json:"ogolna_ilosc_nacisniec"`
	AvgRT            float64       `json:"sredni_czas_reakcji"`
	Score            string        `json:"score"`
	TrialResults     []trialResult `json:"wyniki"`
}

type game struct {
	state          string
	subjectID      string
	currentTrial   int
	greenSide      string
	greenShown     bool
	trialStart     time.Time
	greenOnsetTime time.Time
	responseGiven  bool
	responseSide   string
	responseRT     float64
	results        []trialResult
	correctCount   int
	incorrectCount int
	totalResponded int
	rtSum          float64
	redImg         *ebiten.Image
	greenImg       *ebiten.Image
	carImg         *ebiten.Image
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
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.startTrial()
		}

	case "trial":
		elapsed := time.Since(g.trialStart).Seconds()

		if !g.greenShown && elapsed >= greenOnset {
			g.greenShown = true
			g.greenOnsetTime = time.Now()
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyLeft) && !g.responseGiven {
			g.responseGiven = true
			g.responseSide = "left"
			if g.greenShown {
				g.responseRT = float64(time.Since(g.greenOnsetTime).Milliseconds())
			}
			g.finishTrial()
		} else if inpututil.IsKeyJustPressed(ebiten.KeyRight) && !g.responseGiven {
			g.responseGiven = true
			g.responseSide = "right"
			if g.greenShown {
				g.responseRT = float64(time.Since(g.greenOnsetTime).Milliseconds())
			}
			g.finishTrial()
		} else if !g.responseGiven && elapsed >= trialTimeout {
			g.finishTrial()
		}

	case "done":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			return ebiten.Termination
		}
	}
	return nil
}

func (g *game) startTrial() {
	g.state = "trial"
	g.currentTrial++
	g.greenShown = false
	g.responseGiven = false
	g.responseSide = ""
	g.responseRT = 0

	if rand.Float64() < 0.5 {
		g.greenSide = "left"
	} else {
		g.greenSide = "right"
	}

	g.trialStart = time.Now()
}

func (g *game) finishTrial() {
	var outcome string

	if !g.greenShown {
		outcome = "incorrect"
	} else if g.responseSide == "" {
		outcome = "too_slow"
	} else if g.responseSide == g.greenSide {
		outcome = "correct"
	} else {
		outcome = "incorrect"
	}

	g.results = append(g.results, trialResult{
		CorrectSide: g.greenSide,
		ClickedSide: g.responseSide,
		RT:          g.responseRT,
		Outcome:     outcome,
	})

	if outcome == "correct" {
		g.correctCount++
	} else {
		g.incorrectCount++
	}

	if g.responseSide != "" {
		g.totalResponded++
		g.rtSum += g.responseRT
	}

	if g.currentTrial >= numTrials {
		g.writeResults()
		g.state = "done"
	} else {
		g.startTrial()
	}
}

func (g *game) writeResults() {
	var avgRT float64
	if g.totalResponded > 0 {
		avgRT = g.rtSum / float64(g.totalResponded)
	}

	score := fmt.Sprintf("Poprawne: %d, Bledne: %d, Sredni RT: %.0f ms", g.correctCount, g.incorrectCount, avgRT)

	data := resultsData{
		TestID:           "Sygnalizacja",
		SubjectID:        g.subjectID,
		Timestamp:        common.Timestamp(),
		CorrectPresses:   g.correctCount,
		IncorrectPresses: g.incorrectCount,
		TotalPresses:     g.totalResponded,
		AvgRT:            avgRT,
		Score:            score,
		TrialResults:     g.results,
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
			"Sygnalizacja\n\n"+
				"Zobaczysz dwa swiatla drogowe i dwa samochody.\n"+
				"Jedno swiatlo zmieni sie na zielone.\n\n"+
				"Nacisnij STRZALKE LEWA jesli zielone jest po lewej.\n"+
				"Nacisnij STRZALKE PRAWA jesli zielone jest po prawej.\n\n"+
				"Nacisnij SPACE aby rozpoczac.",
			cx, cy, common.FontMedium, common.White)

	case "trial":
		g.drawTrial(screen)

	case "done":
		common.DrawTextCentered(screen,
			"Test zakonczony!\n\n"+
				fmt.Sprintf("Poprawne: %d\n", g.correctCount)+
				fmt.Sprintf("Bledne: %d\n", g.incorrectCount)+
				fmt.Sprintf("Sredni czas reakcji: %.0f ms\n\n", g.rtSum/float64(max(g.totalResponded, 1)))+
				"Nacisnij ENTER aby zakonczyc.",
			cx, cy, common.FontMedium, common.Green)
	}
}

func (g *game) drawTrial(screen *ebiten.Image) {
	lightW := common.HS(0.15)
	lightH := common.HS(0.25)
	carW := common.HS(0.2)
	carH := common.HS(0.2)

	leftLightX, leftLightY := common.H(-0.6, 0.2)
	rightLightX, rightLightY := common.H(0.6, 0.2)
	leftCarX, leftCarY := common.H(-0.3, 0.0)
	rightCarX, rightCarY := common.H(0.3, 0.0)

	leftCarX -= carW / 2
	leftCarY -= carH / 2
	rightCarX -= carW / 2
	rightCarY -= carH / 2
	leftLightX -= lightW / 2
	leftLightY -= lightH / 2
	rightLightX -= lightW / 2
	rightLightY -= lightH / 2

	common.DrawImage(screen, g.carImg, leftCarX, leftCarY, carW, carH, 1.0)
	common.DrawImage(screen, g.carImg, rightCarX, rightCarY, carW, carH, 1.0)

	leftImg := g.redImg
	rightImg := g.redImg
	if g.greenShown {
		if g.greenSide == "left" {
			leftImg = g.greenImg
		} else {
			rightImg = g.greenImg
		}
	}

	common.DrawImage(screen, leftImg, leftLightX, leftLightY, lightW, lightH, 1.0)
	common.DrawImage(screen, rightImg, rightLightX, rightLightY, lightW, lightH, 1.0)
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
		subjectID: common.RandomID(),
		redImg:    common.LoadImageFS(resources, "sygCzer.png"),
		greenImg:  common.LoadImageFS(resources, "sygZiel.png"),
		carImg:    common.LoadImageFS(resources, "sam.png"),
	}

	ebiten.SetWindowTitle("Sygnalizacja")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
