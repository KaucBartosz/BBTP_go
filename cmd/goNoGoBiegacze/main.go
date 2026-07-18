package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type state int

const (
	stateTitle state = iota
	stateInstructions
	stateModeSelect
	stateTraining
	stateTest
	stateBreak
	stateFeedback
	stateResults
)

type trialResult struct {
	Condition    string  `json:"condition"`
	Digit        string  `json:"digit"`
	Pressed      bool    `json:"pressed"`
	Rt           float64 `json:"rt"`
	Anticipatory bool    `json:"anticipatory"`
	WasCorrect   bool    `json:"was_correct"`
}

type statsJSON struct {
	GoTrials          int     `json:"go_trials"`
	NogoTrials        int     `json:"nogo_trials"`
	Hits              int     `json:"hits"`
	Misses            int     `json:"misses"`
	FalseAlarms       int     `json:"false_alarms"`
	CorrectRejections int     `json:"correct_rejections"`
	DPprime           string  `json:"d_prime"`
	AvgHitRtMs        int     `json:"avg_hit_rt_ms"`
	AvgFaRtMs         int     `json:"avg_fa_rt_ms"`
	AccuracyPercent   int     `json:"accuracy_percent"`
}

type resultsJSON struct {
	TestId            string        `json:"testId"`
	SubjectId         string        `json:"subjectId"`
	Timestamp         string        `json:"timestamp"`
	IloscPoprawnych   int           `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych     int           `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc       int           `json:"ogolna_ilosc_nacisniec"`
	SredniCzasReakcji int           `json:"sredni_czas_reakcji"`
	PoziomTrudnosci   string        `json:"poziom_trudnosci"`
	Score             string        `json:"score"`
	Statystyki        statsJSON     `json:"statystyki"`
	Wyniki            []trialResult `json:"wyniki"`
}

type trialDef struct {
	condition string
	digit     string
	soa       float64
}

type game struct {
	state        state
	subjectId    string
	timestamp    string
	trainingMode bool

	trialDefs    []trialDef
	trialIndex   int
	currentBlock int
	totalBlocks  int

	trialStart time.Time
	phase      string
	phaseStart time.Time

	spaceDown     bool
	spaceCaptured bool
	rtSec         float64
	hasResponse   bool

	feedbackText  string
	feedbackClr   color.Color
	feedbackStart time.Time

	allResults []trialResult
	resultsOut *resultsJSON

	titleTimer  time.Time
	started     bool
	escapeReady bool
	startupTicks int
}

func (g *game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.escapeReady = true
	}

	switch g.state {
	case stateTitle:
		g.updateTitle()
	case stateInstructions:
		return g.updateInstructions()
	case stateModeSelect:
		return g.updateModeSelect()
	case stateTraining:
		return g.updateTrial()
	case stateTest:
		return g.updateTrial()
	case stateBreak:
		return g.updateBreak()
	case stateFeedback:
		g.updateFeedback()
	case stateResults:
		return g.updateResults()
	}
	return nil
}

func (g *game) updateTitle() {
	if !g.started {
		g.titleTimer = time.Now()
		g.started = true
	}
	if time.Since(g.titleTimer) >= 500*time.Millisecond {
		g.state = stateInstructions
	}
}

func (g *game) updateInstructions() error {
	if g.escapeReady {
		g.writeEmptyResults()
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.state = stateModeSelect
	}
	return nil
}

func (g *game) updateModeSelect() error {
	if g.escapeReady {
		g.writeEmptyResults()
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.trainingMode = false
		g.beginTest()
	} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.trainingMode = true
		g.beginTraining()
	}
	return nil
}

func (g *game) beginTraining() {
	g.trialDefs = generateBlock(10, 10, 3)
	g.trialIndex = 0
	g.allResults = nil
	g.state = stateTraining
	g.startNextTrial()
}

func (g *game) beginTest() {
	g.totalBlocks = 4
	g.currentBlock = 0
	g.trialDefs = generateBlock(40, 10, 3)
	g.trialIndex = 0
	if !g.trainingMode {
		g.allResults = nil
	}
	g.state = stateTest
	g.startNextTrial()
}

func (g *game) startNextTrial() {
	if g.trialIndex >= len(g.trialDefs) {
		if g.state == stateTraining {
			g.beginTest()
			return
		}
		g.currentBlock++
		if g.currentBlock >= g.totalBlocks {
			g.computeAndWriteResults()
			g.state = stateResults
			return
		}
		g.trialDefs = generateBlock(40, 10, 3)
		g.trialIndex = 0
		g.state = stateBreak
		return
	}
	g.phase = "stimulus"
	g.phaseStart = time.Now()
	g.trialStart = time.Now()
	g.spaceDown = false
	g.spaceCaptured = false
	g.rtSec = 0
	g.hasResponse = false
}

func (g *game) updateTrial() error {
	if g.escapeReady {
		g.writeEmptyResults()
		return ebiten.Termination
	}
	trial := g.trialDefs[g.trialIndex]
	elapsed := time.Since(g.phaseStart)

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) && !g.hasResponse {
		g.hasResponse = true
		g.spaceDown = true
		g.spaceCaptured = true
		g.rtSec = time.Since(g.trialStart).Seconds()
	}

	switch g.phase {
	case "stimulus":
		if elapsed >= 500*time.Millisecond {
			g.phase = "blank"
			g.phaseStart = time.Now()
		}
	case "blank":
		blankDuration := time.Duration(trial.soa*float64(time.Second)) - 500*time.Millisecond
		if elapsed >= blankDuration {
			g.endTrial()
		}
	}
	return nil
}

func (g *game) endTrial() {
	trial := g.trialDefs[g.trialIndex]
	if g.spaceDown {
		g.rtSec = time.Since(g.trialStart).Seconds()
	}

	anticipatory := g.spaceDown && g.rtSec < 0.150
	validResponse := g.spaceDown && !anticipatory

	isGo := trial.condition == "go"
	var wasCorrect bool
	if isGo {
		wasCorrect = validResponse
	} else {
		wasCorrect = !validResponse
	}

	if g.state == stateTraining {
		if anticipatory {
			g.feedbackText = "Zbyt szybko!"
			g.feedbackClr = common.Orange
		} else if wasCorrect {
			g.feedbackText = "Dobrze!"
			g.feedbackClr = common.Green
		} else {
			g.feedbackText = "Źle!"
			g.feedbackClr = common.Red
		}
		g.feedbackStart = time.Now()
		g.state = stateFeedback
		return
	}

	g.allResults = append(g.allResults, trialResult{
		Condition:    trial.condition,
		Digit:        trial.digit,
		Pressed:      g.spaceDown,
		Rt:           g.rtSec,
		Anticipatory: anticipatory,
		WasCorrect:   wasCorrect,
	})
	g.trialIndex++
	g.startNextTrial()
}

func (g *game) updateFeedback() {
	if time.Since(g.feedbackStart) >= 500*time.Millisecond {
		trial := g.trialDefs[g.trialIndex]
		g.allResults = append(g.allResults, trialResult{
			Condition:    trial.condition,
			Digit:        trial.digit,
			Pressed:      g.spaceDown,
			Rt:           g.rtSec,
			Anticipatory: g.spaceDown && g.rtSec < 0.150,
			WasCorrect:   g.computeCorrect(trial),
		})
		g.trialIndex++
		g.startNextTrial()
	}
}

func (g *game) computeCorrect(trial trialDef) bool {
	anticipatory := g.spaceDown && g.rtSec < 0.150
	validResponse := g.spaceDown && !anticipatory
	if trial.condition == "go" {
		return validResponse
	}
	return !validResponse
}

func (g *game) updateBreak() error {
	if g.escapeReady {
		g.writeEmptyResults()
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.trialDefs = generateBlock(40, 10, 3)
		g.trialIndex = 0
		g.state = stateTest
		g.startNextTrial()
	}
	return nil
}

func (g *game) updateResults() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return ebiten.Termination
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)

	switch g.state {
	case stateTitle:
		g.drawTitle(screen)
	case stateInstructions:
		g.drawInstructions(screen)
	case stateModeSelect:
		g.drawModeSelect(screen)
	case stateTraining, stateTest:
		g.drawTrial(screen)
	case stateBreak:
		g.drawBreak(screen)
	case stateFeedback:
		g.drawFeedback(screen)
	case stateResults:
		g.drawResults(screen)
	}
}

func (g *game) drawTitle(screen *ebiten.Image) {
	common.DrawTextCentered(screen, "Go/No-Go Cyfry",
		common.ScreenW/2, common.ScreenH/2, common.FontBig, common.White)
	common.DrawTextCentered(screen, "Naciśnij SPACJĘ, aby kontynuować",
		common.ScreenW/2, common.ScreenH/2+120, common.FontSmall, common.Gray)
}

func (g *game) drawInstructions(screen *ebiten.Image) {
	msg := "Zadanie Go/No-Go\n\n" +
		"Naciśnij SPACJĘ, gdy zobaczysz cyfrę NIEPARZYSTĄ (1, 3, 7, 9).\n" +
		"NIE naciskaj niczego, gdy zobaczysz cyfrę PARZYSTĄ (2, 4, 6, 8).\n\n" +
		"Naciśnij SPACJĘ, aby rozpocząć."
	common.DrawTextCentered(screen, msg,
		common.ScreenW/2, common.ScreenH/2, common.FontMedium, common.White)
}

func (g *game) drawModeSelect(screen *ebiten.Image) {
	msg := "Wybierz tryb:\n\n1 - Badanie\n2 - Trening + Badanie\n\nNaciśnij 1 lub 2."
	common.DrawTextCentered(screen, msg,
		common.ScreenW/2, common.ScreenH/2, common.FontMedium, common.White)
}

func (g *game) drawTrial(screen *ebiten.Image) {
	if g.phase == "stimulus" {
		trial := g.trialDefs[g.trialIndex]
		common.DrawTextCentered(screen, trial.digit,
			common.ScreenW/2, common.ScreenH/2, common.FontBig, common.White)
	}
}

func (g *game) drawBreak(screen *ebiten.Image) {
	common.DrawTextCentered(screen,
		"Przerwa. Naciśnij SPACJĘ, aby kontynuować.",
		common.ScreenW/2, common.ScreenH/2, common.FontMedium, common.White)
}

func (g *game) drawFeedback(screen *ebiten.Image) {
	trial := g.trialDefs[g.trialIndex]
	common.DrawTextCentered(screen, trial.digit,
		common.ScreenW/2, common.ScreenH/2-80, common.FontBig, common.Gray)
	common.DrawTextCentered(screen, g.feedbackText,
		common.ScreenW/2, common.ScreenH/2+60, common.FontMedium, g.feedbackClr)
}

func (g *game) drawResults(screen *ebiten.Image) {
	if g.resultsOut == nil {
		return
	}
	r := g.resultsOut
	lines := []string{
		"Wyniki:",
		fmt.Sprintf("Go: %d  No-Go: %d", r.Statystyki.GoTrials, r.Statystyki.NogoTrials),
		fmt.Sprintf("Poprawne naciśnięcia: %d", r.Statystyki.Hits),
		fmt.Sprintf("Błędne naciśnięcia: %d", r.Statystyki.FalseAlarms),
		fmt.Sprintf("Czas reakcji (śr.): %d ms", r.Statystyki.AvgHitRtMs),
		fmt.Sprintf("d': %s", r.Statystyki.DPprime),
		fmt.Sprintf("Dokładność: %d%%", r.Statystyki.AccuracyPercent),
		"",
		"Naciśnij SPACJĘ, aby zakończyć.",
	}
	common.DrawTextCentered(screen, strings.Join(lines, "\n"),
		common.ScreenW/2, common.ScreenH/2, common.FontSmall, common.White)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return common.ScreenW, common.ScreenH
}

func generateBlock(numGo, numNogo, maxConsecutiveNogo int) []trialDef {
	goStims := []string{"1", "3", "7", "9"}
	nogoStims := []string{"2", "4", "6", "8"}
	total := numGo + numNogo

	for {
		seq := make([]string, 0, total)
		goLeft, nogoLeft := numGo, numNogo
		for i := 0; i < total; i++ {
			if goLeft == 0 {
				seq = append(seq, "nogo")
				nogoLeft--
			} else if nogoLeft == 0 {
				seq = append(seq, "go")
				goLeft--
			} else {
				if rand.Float64() < float64(goLeft)/float64(goLeft+nogoLeft) {
					seq = append(seq, "go")
					goLeft--
				} else {
					seq = append(seq, "nogo")
					nogoLeft--
				}
			}
		}

		maxCons := 0
		curCons := 0
		for _, c := range seq {
			if c == "nogo" {
				curCons++
				if curCons > maxCons {
					maxCons = curCons
				}
			} else {
				curCons = 0
			}
		}
		if maxCons <= maxConsecutiveNogo {
			block := make([]trialDef, total)
			for i, c := range seq {
				var digit string
				if c == "go" {
					digit = goStims[rand.Intn(len(goStims))]
				} else {
					digit = nogoStims[rand.Intn(len(nogoStims))]
				}
				block[i] = trialDef{
					condition: c,
					digit:     digit,
					soa:       1.3 + rand.Float64()*0.3,
				}
			}
			return block
		}
	}
}

func pnorm(p float64) float64 {
	if p < 0.00001 {
		p = 0.00001
	}
	if p > 0.99999 {
		p = 0.99999
	}
	t := 0.0
	if p < 0.5 {
		t = math.Sqrt(-2.0 * math.Log(p))
	} else {
		t = math.Sqrt(-2.0 * math.Log(1.0 - p))
	}
	num := 2.515517 + 0.802853*t + 0.010328*t*t
	den := 1.0 + 1.432788*t + 0.189269*t*t + 0.001308*t*t*t
	z := t - num/den
	if p < 0.5 {
		return -z
	}
	return z
}

func (g *game) computeAndWriteResults() {
	goTrials := 0
	nogoTrials := 0
	hits := 0
	fa := 0
	sumHitRt := 0.0
	countHitRt := 0
	sumFaRt := 0.0
	countFaRt := 0

	for _, t := range g.allResults {
		if t.Anticipatory {
			continue
		}
		if t.Condition == "go" {
			goTrials++
			if t.Pressed {
				hits++
				sumHitRt += t.Rt
				countHitRt++
			}
		} else {
			nogoTrials++
			if t.Pressed {
				fa++
				sumFaRt += t.Rt
				countFaRt++
			}
		}
	}

	hr := 0.0
	if goTrials > 0 {
		hr = float64(countHitRt) / float64(goTrials)
	}
	far := 0.0
	if nogoTrials > 0 {
		far = float64(countFaRt) / float64(nogoTrials)
	}

	hrAdj := hr
	farAdj := far
	if hrAdj == 1 {
		hrAdj = 1.0 - 1.0/(2.0*float64(goTrials))
	}
	if hrAdj == 0 {
		hrAdj = 1.0 / (2.0 * float64(goTrials))
	}
	if farAdj == 1 {
		farAdj = 1.0 - 1.0/(2.0*float64(nogoTrials))
	}
	if farAdj == 0 {
		farAdj = 1.0 / (2.0 * float64(nogoTrials))
	}

	dPrimeVal := pnorm(hrAdj) - pnorm(farAdj)
	dPrimeStr := fmt.Sprintf("%.2f", dPrimeVal)

	avgHitRtMs := 0
	if countHitRt > 0 {
		avgHitRtMs = int((sumHitRt / float64(countHitRt)) * 1000)
	}
	avgFaRtMs := 0
	if countFaRt > 0 {
		avgFaRtMs = int((sumFaRt / float64(countFaRt)) * 1000)
	}

	cr := nogoTrials - fa
	misses := goTrials - hits

	totalCorrect := hits + cr
	totalTrials := goTrials + nogoTrials
	accuracy := 0
	if totalTrials > 0 {
		accuracy = int((float64(totalCorrect) / float64(totalTrials)) * 100)
	}

	allPresses := 0
	for _, t := range g.allResults {
		if t.Pressed && !t.Anticipatory {
			allPresses++
		}
	}

	scoreText := fmt.Sprintf("Hits: %d/%d | FA: %d/%d | Skut: %d%% | d': %s | RT Hits: %d ms | RT FA: %d ms",
		hits, goTrials, fa, nogoTrials, accuracy, dPrimeStr, avgHitRtMs, avgFaRtMs)

	g.resultsOut = &resultsJSON{
		TestId:            "GoNoGoCyfry",
		SubjectId:         g.subjectId,
		Timestamp:         g.timestamp,
		IloscPoprawnych:   hits,
		IloscBlednych:     fa,
		OgolnaIlosc:       allPresses,
		SredniCzasReakcji: avgHitRtMs,
		PoziomTrudnosci:   "Standard",
		Score:             scoreText,
		Statystyki: statsJSON{
			GoTrials:          goTrials,
			NogoTrials:        nogoTrials,
			Hits:              hits,
			Misses:            misses,
			FalseAlarms:       fa,
			CorrectRejections: cr,
			DPprime:           dPrimeStr,
			AvgHitRtMs:        avgHitRtMs,
			AvgFaRtMs:         avgFaRtMs,
			AccuracyPercent:   accuracy,
		},
		Wyniki: g.allResults,
	}

	if err := common.WriteResults(".", g.resultsOut); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *game) writeEmptyResults() {
	err := common.WriteResults(".", &resultsJSON{
		TestId:            "GoNoGoCyfry",
		SubjectId:         g.subjectId,
		Timestamp:         g.timestamp,
		IloscPoprawnych:   0,
		IloscBlednych:     0,
		OgolnaIlosc:       0,
		SredniCzasReakcji: 0,
		PoziomTrudnosci:   "Standard",
		Score:             "",
		Statystyki:        statsJSON{},
		Wyniki:            []trialResult{},
	})
	if err != nil {
		log.Printf("Error writing empty results: %v", err)
	}
}

func main() {
	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	g := &game{
		state:     stateTitle,
		subjectId: common.RandomID(),
		timestamp: common.Timestamp(),
	}

	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetWindowTitle("Go/No-Go Cyfry")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
