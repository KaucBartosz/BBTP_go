package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	STATE_INSTRUCTIONS = iota
	STATE_FIXATION
	STATE_STIMULUS
	STATE_PAUSE
	STATE_DONE
)

var (
	words       = []string{"czerwony", "niebieski", "zielony", "żółty"}
	colorNames  = []string{"czerwony", "niebieski", "zielony", "żółty"}
	keyToColor  = map[int]int{1: 0, 2: 1, 3: 2, 4: 3}
	wordColors  = []color.RGBA{
		{255, 0, 0, 255},    // czerwony
		{0, 0, 255, 255},    // niebieski
		{0, 200, 0, 255},    // zielony
		{255, 255, 0, 255},  // żółty
	}
)

type TrialResult struct {
	Trial     int    `json:"trial"`
	Word      string `json:"word"`
	Color     string `json:"color"`
	Congruent bool   `json:"congruent"`
	RespKey   string `json:"resp_key"`
	Correct   int    `json:"correct"`
	Rt        *int   `json:"rt"`
	Responded bool   `json:"responded"`
}

type Results struct {
	TestID                  string         `json:"testId"`
	SubjectID               string         `json:"subjectId"`
	Timestamp               string         `json:"timestamp"`
	IloscPoprawnych         int            `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych           int            `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc             int            `json:"ogolna_ilosc_nacisniec"`
	SredniCzas              float64        `json:"sredni_czas_reakcji"`
	PoziomTrudnosci         string         `json:"poziom_trudnosci"`
	Score                   string         `json:"score"`
	Statystyki              map[string]int `json:"statystyki"`
	Wyniki                  []TrialResult  `json:"wyniki"`
}

type Game struct {
	state       int
	numTrials   int
	currentTrial int
	trials      []trialDef
	results     []TrialResult
	stateStart  time.Time
	responseKey int
	responded   bool
	rt          int
	correct     int
	incorrect   int
	largeFace   font.Face
	startupTicks int
}

type trialDef struct {
	word      string
	colorIdx  int
	congruent bool
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	largeFace := loadLargeFace()

	g := &Game{
		state:      STATE_INSTRUCTIONS,
		stateStart: time.Now(),
		largeFace:  largeFace,
	}

	ebiten.SetWindowTitle("Test Stroopa")
	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func loadLargeFace() font.Face {
	fontBytes, err := os.ReadFile("common/font.ttf")
	if err != nil {
		log.Printf("Warning: could not load font for large face: %v, using FontBig", err)
		return common.FontBig
	}
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		log.Printf("Warning: could not parse font: %v, using FontBig", err)
		return common.FontBig
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    180,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("Warning: could not create large face: %v, using FontBig", err)
		return common.FontBig
	}
	return face
}

func (g *Game) generateTrials() []trialDef {
	trials := make([]trialDef, g.numTrials)
	for i := range trials {
		wordIdx := rand.Intn(4)
		colorIdx := rand.Intn(4)
		trials[i] = trialDef{
			word:      words[wordIdx],
			colorIdx:  colorIdx,
			congruent: wordIdx == colorIdx,
		}
	}
	return trials
}

func (g *Game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.writeResults()
		return ebiten.Termination
	}

	elapsed := time.Since(g.stateStart)

	switch g.state {
	case STATE_INSTRUCTIONS:
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.numTrials = 10
			g.startTest()
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.numTrials = 20
			g.startTest()
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.numTrials = 30
			g.startTest()
		}

	case STATE_FIXATION:
		if elapsed >= 500*time.Millisecond {
			g.state = STATE_STIMULUS
			g.stateStart = time.Now()
			g.responded = false
			g.responseKey = 0
			g.rt = 0
		}

	case STATE_STIMULUS:
		if elapsed >= 5*time.Second {
			g.recordTrial(false)
			g.nextTrialOrDone()
		} else if !g.responded {
			for k := 1; k <= 4; k++ {
				var key ebiten.Key
				switch k {
				case 1:
					key = ebiten.Key1
				case 2:
					key = ebiten.Key2
				case 3:
					key = ebiten.Key3
				case 4:
					key = ebiten.Key4
				}
				if inpututil.IsKeyJustPressed(key) {
					g.responded = true
					g.responseKey = k
					g.rt = int(elapsed.Milliseconds())
					tr := g.trials[g.currentTrial]
					if keyToColor[k] == tr.colorIdx {
						g.correct++
						g.recordTrial(true)
					} else {
						g.incorrect++
						g.recordTrial(false)
					}
					g.state = STATE_PAUSE
					g.stateStart = time.Now()
					break
				}
			}
		}

	case STATE_PAUSE:
		if elapsed >= 300*time.Millisecond {
			g.nextTrialOrDone()
		}
	}
	return nil
}

func (g *Game) recordTrial(correct bool) {
	tr := g.trials[g.currentTrial]
	var rtPtr *int
	responded := g.responded
	if responded {
		rtPtr = &g.rt
	}
	respKeyStr := ""
	if g.responded {
		respKeyStr = fmt.Sprintf("%d", g.responseKey)
	}
	c := 0
	if correct {
		c = 1
	}
	g.results = append(g.results, TrialResult{
		Trial:     g.currentTrial + 1,
		Word:      tr.word,
		Color:     colorNames[tr.colorIdx],
		Congruent: tr.congruent,
		RespKey:   respKeyStr,
		Correct:   c,
		Rt:        rtPtr,
		Responded: responded,
	})
}

func (g *Game) startTest() {
	g.currentTrial = 0
	g.correct = 0
	g.incorrect = 0
	g.results = nil
	g.trials = g.generateTrials()
	g.state = STATE_FIXATION
	g.stateStart = time.Now()
}

func (g *Game) nextTrialOrDone() {
	g.currentTrial++
	if g.currentTrial >= g.numTrials {
		g.state = STATE_DONE
		g.writeResults()
	} else {
		g.state = STATE_FIXATION
		g.stateStart = time.Now()
	}
}

func (g *Game) writeResults() {
	totalResponded := 0
	rtSum := 0
	for _, r := range g.results {
		if r.Responded {
			totalResponded++
			rtSum += *r.Rt
		}
	}
	avgRt := 0.0
	if totalResponded > 0 {
		avgRt = float64(rtSum) / float64(totalResponded)
	}
	skutecznosc := 0
	if totalResponded > 0 {
		skutecznosc = g.correct * 100 / totalResponded
	}
	results := Results{
		TestID:          "Stroop",
		SubjectID:       common.RandomID(),
		Timestamp:       common.Timestamp(),
		IloscPoprawnych: g.correct,
		IloscBlednych:   g.incorrect,
		OgolnaIlosc:     totalResponded,
		SredniCzas:      avgRt,
		PoziomTrudnosci: fmt.Sprintf("%d pytań", g.numTrials),
		Score:           fmt.Sprintf("%d/%d poprawnych (%.1f%%)", g.correct, g.numTrials, float64(g.correct)*100/float64(g.numTrials)),
		Statystyki: map[string]int{
			"sredni_czas_ms":    int(avgRt),
			"poprawne_reakcje":  g.correct,
			"wszystkie_proby":   g.numTrials,
			"bledne_reakcje":    g.incorrect,
			"skutecznosc":       skutecznosc,
		},
		Wyniki: g.results,
	}
	if err := common.WriteResults(".", results); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)

	switch g.state {
	case STATE_INSTRUCTIONS:
		g.drawInstructions(screen)
	case STATE_FIXATION:
		common.DrawTextCentered(screen, "+", common.ScreenW/2, common.ScreenH/2, g.largeFace, common.White)
	case STATE_STIMULUS:
		g.drawStimulus(screen)
	case STATE_PAUSE:
		// brief blank
	case STATE_DONE:
		common.DrawTextCentered(screen, "Koniec testu!\nWyniki zapisane.", common.ScreenW/2, common.ScreenH/2, common.FontBig, common.White)
	}
}

func (g *Game) drawInstructions(screen *ebiten.Image) {
	cx := float64(common.ScreenW / 2)

	title := "Test Stroopa"
	common.DrawTextCentered(screen, title, cx, 120, common.FontBig, common.Orange)

	instructions := "Zadanie: Zidentyfikuj KOLOR CZCIONKI słowa.\nIgnoruj treść słowa."
	common.DrawTextCentered(screen, instructions, cx, 280, common.FontMedium, common.White)

	keyMap := "Mapowanie klawiszy:\n1: Czerwony (czerwony)\n2: Niebieski (niebieski)\n3: Zielony (zielony)\n4: Żółty (żółty)"
	common.DrawTextCentered(screen, keyMap, cx, 480, common.FontMedium, common.White)

	length := "Długość testu:\n1: 10 pytań\n2: 20 pytań\n3: 30 pytań"
	common.DrawTextCentered(screen, length, cx, 700, common.FontMedium, common.Yellow)

	hint := "Naciśnij 1, 2 lub 3 aby rozpocząć"
	common.DrawTextCentered(screen, hint, cx, 900, common.FontSmall, common.Gray)

	escHint := "ESC aby wyjść"
	common.DrawTextCentered(screen, escHint, cx, 980, common.FontTiny, common.DarkGray)
}

func (g *Game) drawStimulus(screen *ebiten.Image) {
	hints := []struct {
		key  string
		text string
		clr  color.Color
	}{
		{"1", "Czerwony", common.Red},
		{"2", "Niebieski", common.Blue},
		{"3", "Zielony", common.Green},
		{"4", "Żółty", common.Yellow},
	}

	totalWidth := 0.0
	spacings := make([]float64, len(hints))
	for i, h := range hints {
		label := h.key + ": " + h.text
		bounds := text.BoundString(common.FontSmall, label)
		spacings[i] = float64(bounds.Dx()) + 60
		totalWidth += spacings[i]
	}
	startX := (float64(common.ScreenW) - totalWidth) / 2
	y := 60.0
	for i, h := range hints {
		label := h.key + ": " + h.text
		common.DrawTextLeft(screen, label, startX, y, common.FontSmall, h.clr)
		startX += spacings[i]
	}

	tr := g.trials[g.currentTrial]
	word := strings.ToUpper(tr.word)
	common.DrawTextCentered(screen, word, float64(common.ScreenW/2), float64(common.ScreenH/2), g.largeFace, wordColors[tr.colorIdx])

	elapsed := time.Since(g.stateStart)
	remaining := 5.0 - elapsed.Seconds()
	if remaining < 0 {
		remaining = 0
	}
	timerText := fmt.Sprintf("%.1fs", remaining)
	common.DrawTextLeft(screen, timerText, 50, float64(common.ScreenH)-80, common.FontSmall, common.Gray)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return common.ScreenW, common.ScreenH
}
