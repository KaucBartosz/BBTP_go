package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"
)

const (
	targetCorrect = 50
	digitSizeH    = 200
)

type phase int

const (
	phaseInstruction phase = iota
	phaseNBackSelect
	phaseDifficultySelect
	phaseInitSequence
	phaseShowDigit
	phaseWaitResponse
	phaseFeedback
	phaseDone
)

type trialResult struct {
	TrialNumber    int      `json:"trial_number"`
	CurrentNumber  string   `json:"current_number"`
	IsMatch        bool     `json:"is_match"`
	UserResponse   string   `json:"user_response"`
	WasCorrect     bool     `json:"was_correct"`
	Rt             *float64 `json:"rt"`
	TotalCorrect   int      `json:"total_correct"`
}

type results struct {
	TestID                string        `json:"testId"`
	SubjectID             string        `json:"subjectId"`
	Timestamp             string        `json:"timestamp"`
	IloscPoprawnych       int           `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych         int           `json:"ilosc_blednych_nacisniec"`
	OgolnaIloscNacisniec  int           `json:"ogolna_ilosc_nacisniec"`
	SredniCzasReakcji     int           `json:"sredni_czas_reakcji"`
	PoziomTrudnosci       string        `json:"poziom_trudnosci"`
	NBackLevel            int           `json:"nback_level"`
	DecisionTime          float64       `json:"decision_time"`
	TotalCorrect          int           `json:"total_correct"`
	TestEndedReason       string        `json:"test_ended_reason"`
	Score                 string        `json:"score"`
	Wyniki                []trialResult `json:"wyniki"`
}

type game struct {
	phase       phase
	sequence    []int
	totalCorrect int
	trials      []trialResult

	nbackLevel int
	nbackName  string
	difficulty int
	diffName   string
	decTime    float64

	initIndex      int
	initTimer      time.Time
	initNum        int

	currentIndex    int
	currentNum      int
	isMatch         bool
	trialStart      time.Time
	responded       bool
	userSaidYes     bool
	rtSec           float64
	responseRecorded bool

	feedbackTimer    time.Time
	feedbackCorrect  bool
	feedbackText     string
	feedbackNumMatch int

	testEnded      bool
	testEndedReason string

	keysHandled map[ebiten.Key]struct{}
	startupTicks int
}

func main() {
	g := &game{
		phase:       phaseInstruction,
		nbackLevel:  1,
		nbackName:   "1 wstecz",
		difficulty:  2,
		diffName:    "Normalny (2s)",
		decTime:     2.0,
		keysHandled: make(map[ebiten.Key]struct{}),
	}

	ebiten.SetWindowTitle("N Wstecz - Test Pamięci Roboczej")
	ebiten.SetFullscreen(true)
	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)

	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}

func (g *game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	// Global ESC check
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.writeResults()
		return ebiten.Termination
	}

	switch g.phase {
	case phaseInstruction:
		g.updateInstruction()
	case phaseNBackSelect:
		g.updateNBackSelect()
	case phaseDifficultySelect:
		g.updateDifficultySelect()
	case phaseInitSequence:
		g.updateInitSequence()
	case phaseShowDigit:
		g.startNewTrial()
	case phaseWaitResponse:
		g.updateWaitResponse()
	case phaseFeedback:
		g.updateFeedback()
	}

	return nil
}

func (g *game) justPressed(k ebiten.Key) bool {
	if inpututil.IsKeyJustPressed(k) {
		if _, ok := g.keysHandled[k]; !ok {
			g.keysHandled[k] = struct{}{}
			return true
		}
	}
	return false
}

func pressedAny(keys []ebiten.Key) ebiten.Key {
	pressed := inpututil.AppendJustPressedKeys(nil)
	for _, p := range pressed {
		for _, k := range keys {
			if p == k {
				return k
			}
		}
	}
	return -1
}

func (g *game) updateInstruction() {
	k := pressedAny([]ebiten.Key{ebiten.KeySpace})
	if k == ebiten.KeySpace {
		g.phase = phaseNBackSelect
	}
}

func (g *game) updateNBackSelect() {
	k := pressedAny([]ebiten.Key{
		ebiten.Key1, ebiten.Key2, ebiten.Key3, ebiten.Key4, ebiten.Key5,
	})
	switch k {
	case ebiten.Key1:
		g.nbackLevel, g.nbackName = 1, "1 wstecz"
		g.phase = phaseDifficultySelect
	case ebiten.Key2:
		g.nbackLevel, g.nbackName = 2, "2 wstecz"
		g.phase = phaseDifficultySelect
	case ebiten.Key3:
		g.nbackLevel, g.nbackName = 3, "3 wstecz"
		g.phase = phaseDifficultySelect
	case ebiten.Key4:
		g.nbackLevel, g.nbackName = 4, "4 wstecz"
		g.phase = phaseDifficultySelect
	case ebiten.Key5:
		g.nbackLevel, g.nbackName = 5, "5 wstecz"
		g.phase = phaseDifficultySelect
	}
}

func (g *game) updateDifficultySelect() {
	k := pressedAny([]ebiten.Key{ebiten.Key1, ebiten.Key2, ebiten.Key3})
	switch k {
	case ebiten.Key1:
		g.difficulty, g.diffName, g.decTime = 1, "Łatwy (5s)", 5.0
		g.phase = phaseInitSequence
		g.startInitSequence()
	case ebiten.Key2:
		g.difficulty, g.diffName, g.decTime = 2, "Normalny (2s)", 2.0
		g.phase = phaseInitSequence
		g.startInitSequence()
	case ebiten.Key3:
		g.difficulty, g.diffName, g.decTime = 3, "Trudny (1s)", 1.0
		g.phase = phaseInitSequence
		g.startInitSequence()
	}
}

func (g *game) startInitSequence() {
	g.sequence = make([]int, 5)
	for i := range g.sequence {
		g.sequence[i] = rand.Intn(10)
	}
	g.initIndex = 0
	g.initNum = g.sequence[0]
	g.initTimer = time.Now()
}

func (g *game) updateInitSequence() {
	if time.Since(g.initTimer) >= 2*time.Second {
		g.initIndex++
		if g.initIndex >= 5 {
			g.phase = phaseShowDigit
			g.startNewTrial()
			return
		}
		g.initNum = g.sequence[g.initIndex]
		g.initTimer = time.Now()
	}
}

func (g *game) startNewTrial() {
	newNum := rand.Intn(10)
	g.sequence = append(g.sequence, newNum)
	g.currentIndex = len(g.sequence) - 1
	g.currentNum = newNum

	if g.currentIndex >= g.nbackLevel {
		g.isMatch = g.sequence[g.currentIndex] == g.sequence[g.currentIndex-g.nbackLevel]
	} else {
		g.isMatch = false
	}

	g.responded = false
	g.userSaidYes = false
	g.rtSec = 0
	g.responseRecorded = false
	g.trialStart = time.Now()
	g.phase = phaseWaitResponse
}

func (g *game) updateWaitResponse() {
	elapsed := time.Since(g.trialStart)
	if elapsed >= time.Duration(g.decTime*float64(time.Second)) {
		// Timeout → automatic "Nie"
		g.responded = false
		g.phase = phaseFeedback
		g.evaluateAnswer()
		return
	}

	pressed := inpututil.AppendJustPressedKeys(nil)
	for _, k := range pressed {
		switch k {
		case ebiten.KeyY, ebiten.KeyT:
			g.responded = true
			g.userSaidYes = true
			g.rtSec = time.Since(g.trialStart).Seconds()
			g.responseRecorded = true
			g.phase = phaseFeedback
			g.evaluateAnswer()
			return
		case ebiten.KeyN:
			g.responded = true
			g.userSaidYes = false
			g.rtSec = time.Since(g.trialStart).Seconds()
			g.responseRecorded = true
			g.phase = phaseFeedback
			g.evaluateAnswer()
			return
		}
	}
}

func (g *game) evaluateAnswer() {
	wasCorrect := false
	if g.isMatch && g.userSaidYes {
		wasCorrect = true
	} else if !g.isMatch && g.responded && !g.userSaidYes {
		wasCorrect = true
	} else if !g.isMatch && !g.responded {
		wasCorrect = true
	}

	trialNum := g.currentIndex - 9
	if trialNum < 1 {
		trialNum = 1
	}

	respStr := "TAK"
	if g.userSaidYes {
		respStr = "TAK"
	} else if g.responded {
		respStr = "NIE"
	} else {
		respStr = "BRAK"
	}

	if wasCorrect {
		g.totalCorrect++
		g.feedbackCorrect = true
		g.feedbackText = "Poprawnie!"
	} else {
		g.feedbackCorrect = false
		if g.isMatch {
			g.feedbackText = fmt.Sprintf("Błąd! Ta cyfra była taka sama jak %d miejsc temu.", g.nbackLevel)
		} else {
			g.feedbackText = "Błąd!"
		}
	}

	var rtPtr *float64
	if g.responseRecorded {
		rtPtr = &g.rtSec
	}

	g.trials = append(g.trials, trialResult{
		TrialNumber:   trialNum,
		CurrentNumber: fmt.Sprintf("%d", g.currentNum),
		IsMatch:       g.isMatch,
		UserResponse:  respStr,
		WasCorrect:    wasCorrect,
		Rt:            rtPtr,
		TotalCorrect:  g.totalCorrect,
	})

	g.feedbackTimer = time.Now()
	g.feedbackNumMatch = g.nbackLevel

	if !wasCorrect {
		g.testEnded = true
		g.testEndedReason = "wrong_answer"
	}

	if g.totalCorrect >= targetCorrect {
		g.testEnded = true
		g.testEndedReason = "50_correct"
	}

	g.phase = phaseFeedback
}

func (g *game) updateFeedback() {
	if time.Since(g.feedbackTimer) >= 1500*time.Millisecond {
		if g.testEnded {
			g.writeResults()
			g.phase = phaseDone
		} else {
			g.startNewTrial()
		}
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)

	switch g.phase {
	case phaseInstruction:
		g.drawInstruction(screen)
	case phaseNBackSelect:
		g.drawNBackSelect(screen)
	case phaseDifficultySelect:
		g.drawDifficultySelect(screen)
	case phaseInitSequence:
		g.drawInitSequence(screen)
	case phaseShowDigit:
		g.drawInitSequence(screen)
	case phaseWaitResponse:
		g.drawTrialDigit(screen)
	case phaseFeedback:
		g.drawFeedback(screen)
	case phaseDone:
		g.drawDone(screen)
	}
}

func (g *game) drawInstruction(screen *ebiten.Image) {
	text := "W tym teście będziesz widzieć kolejne cyfry.\n\n" +
		"Po wyświetleniu pierwszych 5 cyfr, Twoim zadaniem będzie ocenić,\n" +
		"czy aktualna cyfra jest TAKA SAMA jak cyfra zapisana N miejsc wcześniej.\n\n" +
		"Naciśnij TAK (Y) jeśli cyfra jest taka sama, NIE (N) jeśli jest inna.\n" +
		"Brak reakcji oznacza automatycznie odpowiedź \"Nie\".\n\n" +
		"Naciśnij SPACJĘ, aby kontynuować."
	common.DrawTextCentered(screen, text, common.ScreenW/2, common.ScreenH/2, common.FontSmall, common.White)
}

func (g *game) drawNBackSelect(screen *ebiten.Image) {
	text := "Wybierz ile miejsc wstecz chcesz zapamiętywać:\n\n" +
		"1 - 1 miejsce wstecz\n2 - 2 miejsca wstecz\n3 - 3 miejsca wstecz\n" +
		"4 - 4 miejsca wstecz\n5 - 5 miejsc wstecz\n\n" +
		"Naciśnij 1, 2, 3, 4 lub 5."
	common.DrawTextCentered(screen, text, common.ScreenW/2, common.ScreenH/2, common.FontSmall, common.White)
}

func (g *game) drawDifficultySelect(screen *ebiten.Image) {
	text := "Wybierz czas na odpowiedź:\n\n" +
		"1 - ŁATWY (5 sekund)\n2 - NORMALNY (2 sekundy)\n3 - TRUDNY (1 sekunda)\n\n" +
		"Naciśnij 1, 2 lub 3."
	common.DrawTextCentered(screen, text, common.ScreenW/2, common.ScreenH/2, common.FontSmall, common.White)
}

func (g *game) drawInitSequence(screen *ebiten.Image) {
	countdown := 5 - g.initIndex
	common.DrawTextLeft(screen, fmt.Sprintf("%d", countdown), 100, 120, common.FontMedium, common.Gray)

	digitStr := fmt.Sprintf("%d", g.initNum)
	common.DrawTextCentered(screen, digitStr, common.ScreenW/2, common.ScreenH/2, g.digitFont(), common.White)
}

func (g *game) drawTrialDigit(screen *ebiten.Image) {
	digitStr := fmt.Sprintf("%d", g.currentNum)
	common.DrawTextCentered(screen, digitStr, common.ScreenW/2, common.ScreenH/2-digitSizeH/2, g.digitFont(), common.White)

	question := fmt.Sprintf("Czy ta cyfra pojawiła się %d miejsc temu? (TAK/NIE)", g.nbackLevel)
	common.DrawTextCentered(screen, question, common.ScreenW/2, common.ScreenH/2+100, common.FontSmall, common.White)

	trialNum := g.currentIndex - 9
	if trialNum < 1 {
		trialNum = 1
	}
	progress := fmt.Sprintf("Próba: %d | Poprawne: %d", trialNum, g.totalCorrect)
	common.DrawTextCentered(screen, progress, common.ScreenW/2, common.ScreenH-80, common.FontTiny, common.Gray)
}

func (g *game) drawFeedback(screen *ebiten.Image) {
	digitStr := fmt.Sprintf("%d", g.currentNum)
	common.DrawTextCentered(screen, digitStr, common.ScreenW/2, common.ScreenH/2-100, g.digitFont(), common.White)

	var clr color.Color
	if g.feedbackCorrect {
		clr = common.Green
	} else {
		clr = common.Red
	}
	common.DrawTextCentered(screen, g.feedbackText, common.ScreenW/2, common.ScreenH/2+80, common.FontBig, clr)
}

func (g *game) drawDone(screen *ebiten.Image) {
	if g.testEndedReason == "50_correct" {
		common.DrawTextCentered(screen, "Gratulacje! Osiągnięto 50 poprawnych odpowiedzi!",
			common.ScreenW/2, common.ScreenH/2, common.FontBig, common.Green)
	} else {
		common.DrawTextCentered(screen, fmt.Sprintf("Test zakończony: %s", g.feedbackText),
			common.ScreenW/2, common.ScreenH/2-30, common.FontMedium, common.Red)

		summary := fmt.Sprintf("Poprawne: %d | Błędne: %d | N-Back: %s",
			g.totalCorrect, len(g.trials)-g.totalCorrect, g.nbackName)
		common.DrawTextCentered(screen, summary,
			common.ScreenW/2, common.ScreenH/2+50, common.FontSmall, common.White)
	}

	common.DrawTextCentered(screen, "Naciśnij ESC aby zakończyć.",
		common.ScreenW/2, common.ScreenH-100, common.FontTiny, common.Gray)
}

func (g *game) digitFont() font.Face {
	return common.FontBig
}

func (g *game) writeResults() {
	wszystkie := 0
	for _, t := range g.trials {
		if t.UserResponse != "BRAK" {
			wszystkie++
		}
	}

	bledne := 0
	for _, t := range g.trials {
		if !t.WasCorrect {
			bledne++
		}
	}

	var sumRT float64
	var countRT int
	for _, t := range g.trials {
		if t.Rt != nil {
			sumRT += *t.Rt
			countRT++
		}
	}
	avgRTMs := 0
	if countRT > 0 {
		avgRTMs = int((sumRT / float64(countRT)) * 1000)
	}

	scoreText := fmt.Sprintf("Poprawne: %d | Błędne: %d | N-Back: %s | Poziom: %s | Śr. RT: %d ms",
		g.totalCorrect, bledne, g.nbackName, g.diffName, avgRTMs)

	r := results{
		TestID:               "NWstecz",
		SubjectID:            common.RandomID(),
		Timestamp:            common.Timestamp(),
		IloscPoprawnych:      g.totalCorrect,
		IloscBlednych:        bledne,
		OgolnaIloscNacisniec: wszystkie,
		SredniCzasReakcji:    avgRTMs,
		PoziomTrudnosci:      fmt.Sprintf("%s | %s", g.nbackName, g.diffName),
		NBackLevel:           g.nbackLevel,
		DecisionTime:         g.decTime,
		TotalCorrect:         g.totalCorrect,
		TestEndedReason:      g.testEndedReason,
		Score:                scoreText,
		Wyniki:               g.trials,
	}

	if err := common.WriteResults(".", r); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}
