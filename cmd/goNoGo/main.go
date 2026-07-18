package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type trialResult struct {
	NumberShown string `json:"number_shown"`
	Condition   string `json:"condition"`
	WasCorrect  bool   `json:"was_correct"`
	Pressed     bool   `json:"pressed"`
	RT          *int   `json:"rt"`
}

type resultsData struct {
	TestID           string        `json:"testId"`
	SubjectID        string        `json:"subjectId"`
	Timestamp        string        `json:"timestamp"`
	CorrectPresses   int           `json:"ilosc_poprawnych_nacisniec"`
	IncorrectPresses int           `json:"ilosc_blednych_nacisniec"`
	TotalPresses     int           `json:"ogolna_ilosc_nacisniec"`
	AvgRT            float64       `json:"sredni_czas_reakcji"`
	Difficulty       string        `json:"poziom_trudnosci"`
	Score            string        `json:"score"`
	TrialResults     []trialResult `json:"wyniki"`
}

type game struct {
	state         string
	fontBig       font.Face
	fontMedium    font.Face
	fontSmall     font.Face
	fontHuge      font.Face
	decisionTime  time.Duration
	numTrials     int
	trials        []int
	currentTrial  int
	trialStart    time.Time
	pressed       bool
	feedbackStart time.Time
	results       []trialResult
	correctCount  int
	incorrectCount int
	totalPresses  int
	rtSum         float64
	rtCount       int
	startupTicks int
}

func loadHugeFont() font.Face {
	candidates := []string{
		filepath.Join("common", "font.ttf"),
		filepath.Join("..", "..", "common", "font.ttf"),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		f, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    200,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			continue
		}
		return face
	}
	return common.FontBig
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
			g.decisionTime = 1500 * time.Millisecond
			g.state = "length"
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.decisionTime = 1000 * time.Millisecond
			g.state = "length"
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.decisionTime = 500 * time.Millisecond
			g.state = "length"
		}

	case "length":
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.numTrials = 10
			g.generateTrials()
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.numTrials = 30
			g.generateTrials()
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.numTrials = 60
			g.generateTrials()
		} else {
			break
		}
		g.state = "trial"
		g.currentTrial = 0
		g.trialStart = time.Now()
		g.pressed = false

	case "trial":
		if !g.pressed && inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.pressed = true
		}
		if time.Since(g.trialStart) >= g.decisionTime {
			g.recordTrial()
			g.currentTrial++
			if g.currentTrial >= g.numTrials {
				g.state = "done"
				g.writeResults()
			} else {
				g.state = "feedback"
				g.feedbackStart = time.Now()
			}
		}

	case "feedback":
		if time.Since(g.feedbackStart) >= 1500*time.Millisecond {
			g.state = "trial"
			g.trialStart = time.Now()
			g.pressed = false
		}
	}
	return nil
}

func (g *game) generateTrials() {
	g.trials = make([]int, g.numTrials)
	for i := 0; i < g.numTrials; i++ {
		g.trials[i] = rand.Intn(10)
	}
}

func (g *game) recordTrial() {
	num := g.trials[g.currentTrial]
	isEven := num%2 == 0
	wasPressed := g.pressed
	correct := (isEven && wasPressed) || (!isEven && !wasPressed)

	cond := "go"
	if !isEven {
		cond = "nogo"
	}

	tr := trialResult{
		NumberShown: strconv.Itoa(num),
		Condition:   cond,
		WasCorrect:  correct,
		Pressed:     wasPressed,
		RT:          nil,
	}

	if wasPressed {
		g.totalPresses++
		if isEven {
			rtMs := int(time.Since(g.trialStart).Milliseconds())
			tr.RT = &rtMs
			g.rtSum += float64(rtMs)
			g.rtCount++
			g.correctCount++
		} else {
			g.incorrectCount++
		}
	}

	g.results = append(g.results, tr)
}

func (g *game) writeResults() {
	subjectID := common.RandomID()
	ts := common.Timestamp()

	var diffName string
	switch g.decisionTime {
	case 1500 * time.Millisecond:
		diffName = "Easy"
	case 1000 * time.Millisecond:
		diffName = "Normal"
	case 500 * time.Millisecond:
		diffName = "Hard"
	}

	var avgRT float64
	if g.rtCount > 0 {
		avgRT = g.rtSum / float64(g.rtCount)
	}

	score := fmt.Sprintf("Poprawne: %d, Bledne: %d, Sredni RT: %.0f ms", g.correctCount, g.incorrectCount, avgRT)

	data := resultsData{
		TestID:           "GoNoGo",
		SubjectID:        subjectID,
		Timestamp:        ts,
		CorrectPresses:   g.correctCount,
		IncorrectPresses: g.incorrectCount,
		TotalPresses:     g.totalPresses,
		AvgRT:            avgRT,
		Difficulty:       diffName,
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
			"GoNoGo - Test Hamowania\n\n"+
				"Nacisnij SPACE jesli liczba jest PARNA (0,2,4,6,8)\n"+
				"NIE naciskaj jesli liczba jest NIEPARNA (1,3,5,7,9)\n\n"+
				"Nacisnij SPACE aby kontynuowac",
			cx, cy, g.fontMedium, common.White)

	case "difficulty":
		common.DrawTextCentered(screen,
			"Wybierz poziom trudnosci:\n\n"+
				"1 - Latwy (1.5s)\n"+
				"2 - Normalny (1.0s)\n"+
				"3 - Trudny (0.5s)",
			cx, cy, g.fontMedium, common.White)

	case "length":
		common.DrawTextCentered(screen,
			"Wybierz liczbe prob:\n\n"+
				"1 - 10 prob\n"+
				"2 - 30 prob\n"+
				"3 - 60 prob",
			cx, cy, g.fontMedium, common.White)

	case "trial":
		num := g.trials[g.currentTrial]
		common.DrawTextCentered(screen, strconv.Itoa(num), cx, cy, g.fontHuge, common.White)

	case "feedback":
		common.DrawTextCentered(screen, "...", cx, cy, g.fontBig, common.DarkGray)

	case "done":
		common.DrawTextCentered(screen,
			"Test zakonczony!\n\nDziekuje za udzial.",
			cx, cy, g.fontMedium, common.Green)
	}
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}

func main() {
	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	g := &game{
		state:      "instruction",
		fontBig:    common.FontBig,
		fontMedium: common.FontMedium,
		fontSmall:  common.FontSmall,
		fontHuge:   loadHugeFont(),
	}

	ebiten.SetWindowTitle("GoNoGo - Test Hamowania")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
