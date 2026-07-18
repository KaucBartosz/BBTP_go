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
)

const (
	blockSizeHU     = 0.08
	gapHU           = 0.02
	responseTimeout = 30.0
	maxSeqLen       = 20
	feedbackDur     = 1.0
	clickFlashDur   = 0.3
)

var (
	colorDefault = color.RGBA{51, 13, 13, 255}
	colorSystem  = color.RGBA{230, 26, 26, 255}
	colorPlayer  = color.RGBA{26, 204, 26, 255}
)

type diffInfo struct {
	label    string
	gridSize int
}

type flashInfo struct {
	label string
	dur   float64
	gap   float64
}

var diffs = [9]diffInfo{
	{},
	{"Łatwy (3x3)", 3},
	{"Średni (4x4)", 4},
	{"Trudny (5x5)", 5},
	{"Bardzo trudny (6x6)", 6},
	{"Ekspert (7x7)", 7},
	{"Mistrz (8x8)", 8},
	{"Arcymistrz (9x9)", 9},
	{"Legenda (10x10)", 10},
}

var flashCfg = [4]flashInfo{
	{},
	{"Łatwy (1.0s)", 1.0, 0.3},
	{"Normalny (0.5s)", 0.5, 0.2},
	{"Trudny (0.3s)", 0.3, 0.15},
}

type trialDetail struct {
	Trial          int     `json:"trial"`
	SequenceLength int     `json:"sequenceLength"`
	Result         string  `json:"result"`
	ResponseTime   float64 `json:"responseTime"`
}

type statsData struct {
	Poprawne          int     `json:"poprawne"`
	Bledne            int     `json:"bledne"`
	WszystkieProby    int     `json:"wszystkie_proby"`
	Skutecznosc       float64 `json:"skutecznosc"`
	MaxSekwencja      int     `json:"max_sekwencja"`
	SredniCzasMs      float64 `json:"sredni_czas_ms"`
	Poziom            int     `json:"poziom"`
	RozmiarSiatki     int     `json:"rozmiar_siatki"`
	SzybkoscSwiecenia int     `json:"szybkosc_swiecenia"`
}

type resultsData struct {
	TestID              string        `json:"testId"`
	SubjectID           string        `json:"subjectId"`
	Timestamp           string        `json:"timestamp"`
	IloscPoprawnych     int           `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych       int           `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc         int           `json:"ogolna_ilosc_nacisniec"`
	SredniCzasReakcji   float64       `json:"sredni_czas_reakcji"`
	MaxDlugoscSekwencji int           `json:"max_dlugosc_sekwencji"`
	PoziomTrudnosci     string        `json:"poziom_trudnosci"`
	RozmiarSiatki       string        `json:"rozmiar_siatki"`
	SzybkoscSwiecenia   string        `json:"szybkosc_swiecenia"`
	Score               string        `json:"score"`
	Statystyki          statsData     `json:"statystyki"`
	WynikiSzczegolowe   []trialDetail `json:"wyniki_szczegolowe"`
}

type blockRect struct {
	x, y  float64
	cx, cy float64
}

type Game struct {
	state      string
	stateStart time.Time

	diffIdx  int
	gridSize int
	flashIdx int
	flashDur float64
	flashGap float64

	bsz    float64
	gapPx  float64
	blocks []blockRect

	sequence   []int
	seqIdx     int
	curSeqLen  int
	playerSeq  []int
	prevMouse  bool
	respStart  time.Time
	clickBlock int

	fbText    string
	fbClr     color.Color
	isError   bool
	isTimeout bool

	correctSeqIdx int

	totalTrials   int
	correctTrials int
	maxCorrectLen int
	trialData     []trialDetail
	rtSum         float64
	rtCnt         int

	subjectId string
	timestamp string
	startupTicks int
}

func main() {
	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	g := &Game{
		state:     "welcome",
		subjectId: common.RandomID(),
		timestamp: common.Timestamp(),
	}

	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetWindowTitle("TEST CORSI - Pamięć przestrzenna")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func (g *Game) initGrid() {
	g.bsz = blockSizeHU * 1080
	g.gapPx = gapHU * 1080
	totalW := float64(g.gridSize)*g.bsz + float64(g.gridSize-1)*g.gapPx
	startX := (float64(common.ScreenW) - totalW) / 2
	startY := (float64(common.ScreenH) - totalW) / 2

	g.blocks = make([]blockRect, g.gridSize*g.gridSize)
	for row := 0; row < g.gridSize; row++ {
		for col := 0; col < g.gridSize; col++ {
			idx := row*g.gridSize + col
			x := startX + float64(col)*(g.bsz+g.gapPx)
			y := startY + float64(row)*(g.bsz+g.gapPx)
			g.blocks[idx] = blockRect{x: x, y: y, cx: x + g.bsz/2, cy: y + g.bsz/2}
		}
	}
}

func generateSequence(gridSize, length int) []int {
	total := gridSize * gridSize
	seq := make([]int, 0, length)
	used := make(map[int]bool)
	for len(seq) < length {
		idx := rand.Intn(total)
		if !used[idx] {
			seq = append(seq, idx)
			used[idx] = true
			if len(used) >= int(float64(total)*0.8) {
				used = make(map[int]bool)
			}
		}
	}
	return seq
}

func (g *Game) startTrial() {
	g.sequence = generateSequence(g.gridSize, g.curSeqLen)
	g.seqIdx = 0
	g.playerSeq = nil
	g.prevMouse = false
	g.isError = false
	g.isTimeout = false
	g.state = "show_flash"
	g.stateStart = time.Now()
}

func (g *Game) startPlayerTurn() {
	g.playerSeq = nil
	g.prevMouse = false
	g.respStart = time.Now()
	g.state = "player_turn"
}

func (g *Game) handleTimeout() {
	g.isTimeout = true
	g.totalTrials++
	g.trialData = append(g.trialData, trialDetail{
		Trial:          g.totalTrials,
		SequenceLength: g.curSeqLen,
		Result:         "timeout",
		ResponseTime:   responseTimeout,
	})
	g.fbText = "Czas minął!"
	g.fbClr = common.Orange
	g.correctSeqIdx = 0
	g.state = "replay_correct"
	g.stateStart = time.Now()
}

func (g *Game) handleWrongClick() {
	g.isError = true
	rt := time.Since(g.respStart).Seconds()
	g.totalTrials++
	g.trialData = append(g.trialData, trialDetail{
		Trial:          g.totalTrials,
		SequenceLength: g.curSeqLen,
		Result:         "incorrect",
		ResponseTime:   rt,
	})
	g.fbText = "BŁĄD!"
	g.fbClr = common.Red
	g.correctSeqIdx = 0
	g.state = "replay_correct"
	g.stateStart = time.Now()
}

func (g *Game) handleCorrectComplete() {
	rt := time.Since(g.respStart).Seconds()
	g.totalTrials++
	g.correctTrials++
	if g.curSeqLen > g.maxCorrectLen {
		g.maxCorrectLen = g.curSeqLen
	}
	g.rtSum += rt
	g.rtCnt++
	g.trialData = append(g.trialData, trialDetail{
		Trial:          g.totalTrials,
		SequenceLength: g.curSeqLen,
		Result:         "correct",
		ResponseTime:   rt,
	})
	g.fbText = "POPRAWNIE!"
	g.fbClr = common.Green
	g.isError = false
	g.state = "feedback"
	g.stateStart = time.Now()
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

	switch g.state {
	case "welcome":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.state = "difficulty"
			g.stateStart = time.Now()
		}

	case "difficulty":
		for i := ebiten.Key1; i <= ebiten.Key8; i++ {
			if inpututil.IsKeyJustPressed(i) {
				g.diffIdx = int(i-ebiten.Key1) + 1
				g.gridSize = diffs[g.diffIdx].gridSize
				g.state = "flash_speed"
				g.stateStart = time.Now()
				return nil
			}
		}

	case "flash_speed":
		for i := ebiten.Key1; i <= ebiten.Key3; i++ {
			if inpututil.IsKeyJustPressed(i) {
				g.flashIdx = int(i-ebiten.Key1) + 1
				g.flashDur = flashCfg[g.flashIdx].dur
				g.flashGap = flashCfg[g.flashIdx].gap
				g.initGrid()
				g.curSeqLen = 2
				g.startTrial()
				return nil
			}
		}

	case "show_flash":
		if time.Since(g.stateStart).Seconds() >= g.flashDur {
			g.seqIdx++
			if g.seqIdx >= len(g.sequence) {
				g.startPlayerTurn()
			} else {
				g.state = "show_gap"
				g.stateStart = time.Now()
			}
		}

	case "show_gap":
		if time.Since(g.stateStart).Seconds() >= g.flashGap {
			g.state = "show_flash"
			g.stateStart = time.Now()
		}

	case "player_turn":
		if time.Since(g.respStart).Seconds() >= responseTimeout {
			g.handleTimeout()
			return nil
		}

		mouseBtn := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		isNewClick := mouseBtn && !g.prevMouse
		g.prevMouse = mouseBtn

		if isNewClick {
			mx, my := ebiten.CursorPosition()
			clicked := -1
			for i, b := range g.blocks {
				if common.PointInCircle(float64(mx), float64(my), b.cx, b.cy, g.bsz/2) {
					clicked = i
					break
				}
			}
			if clicked >= 0 {
				g.playerSeq = append(g.playerSeq, clicked)
				g.clickBlock = clicked
				g.state = "click_show"
				g.stateStart = time.Now()
			}
		}

	case "click_show":
		if time.Since(g.stateStart).Seconds() >= clickFlashDur {
			idx := len(g.playerSeq) - 1
			if g.playerSeq[idx] != g.sequence[idx] {
				g.handleWrongClick()
			} else if len(g.playerSeq) == len(g.sequence) {
				g.handleCorrectComplete()
			} else {
				g.state = "player_turn"
			}
		}

	case "feedback":
		if time.Since(g.stateStart).Seconds() >= feedbackDur {
			if g.curSeqLen < maxSeqLen {
				g.curSeqLen++
			}
			g.startTrial()
		}

	case "replay_correct":
		elapsed := time.Since(g.stateStart).Seconds()
		if g.correctSeqIdx < len(g.sequence) {
			if elapsed >= 0.3 {
				g.correctSeqIdx++
				g.stateStart = time.Now()
			}
		} else {
			if elapsed >= feedbackDur {
				g.writeResults()
				g.state = "done"
			}
		}

	case "done":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			return ebiten.Termination
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)
	cx := float64(common.ScreenW) / 2
	cy := float64(common.ScreenH) / 2

	switch g.state {
	case "welcome":
		common.DrawTextCentered(screen,
			"TEST CORSI - Pamięć przestrzenna\n\n"+
				"Na ekranie pojawią się kwadraty w siatce.\n"+
				"Kilka kwadratów zapali się na CZERWONO (wzór systemu).\n"+
				"Zapamiętaj kolejność i kliknij na kwadraty — podświetlą się na ZIELONO.\n"+
				"Każda poprawna odpowiedź wydłuży sekwencję.\n"+
				"Test kończy się po błędnej odpowiedzi.\n\n"+
				"Naciśnij SPACJĘ, aby kontynuować.\n"+
				"ESC - wyjście",
			cx, cy, common.FontSmall, common.White)

	case "difficulty":
		common.DrawTextCentered(screen,
			"Wybierz rozmiar siatki:\n\n"+
				"1 - 3x3 (Łatwy)\n2 - 4x4 (Średni)\n3 - 5x5 (Trudny)\n4 - 6x6\n"+
				"5 - 7x7\n6 - 8x8\n7 - 9x9\n8 - 10x10 (Legenda)\n\n"+
				"Naciśnij 1-8\nESC - wyjście",
			cx, cy, common.FontMedium, common.White)

	case "flash_speed":
		common.DrawTextCentered(screen,
			"Wybierz szybkość świecenia:\n\n"+
				"1 - Łatwy (1.0s - długo)\n2 - Normalny (0.5s)\n3 - Trudny (0.3s - szybko)\n\n"+
				"Naciśnij 1, 2 lub 3\nESC - wyjście",
			cx, cy, common.FontMedium, common.White)

	case "show_flash", "show_gap", "player_turn", "click_show", "feedback", "replay_correct":
		g.drawGame(screen)

	case "done":
		accuracy := 0.0
		if g.totalTrials > 0 {
			accuracy = float64(g.correctTrials) / float64(g.totalTrials) * 100
		}
		avgRT := 0.0
		if g.rtCnt > 0 {
			avgRT = (g.rtSum / float64(g.rtCnt)) * 1000
		}
		common.DrawTextCentered(screen,
			fmt.Sprintf("Test zakończony!\n\n"+
				"Max sekwencja: %d\n"+
				"Poprawne: %d/%d\n"+
				"Skuteczność: %.0f%%\n"+
				"Średni RT: %.0f ms\n\n"+
				"Wyniki zapisane.\nNaciśnij SPACE aby zamknąć.",
				g.maxCorrectLen, g.correctTrials, g.totalTrials, accuracy, avgRT),
			cx, cy, common.FontMedium, common.Green)
	}
}

func (g *Game) drawGame(screen *ebiten.Image) {
	for i, b := range g.blocks {
		clr := colorDefault

		switch g.state {
		case "show_flash":
			if g.seqIdx < len(g.sequence) && i == g.sequence[g.seqIdx] {
				clr = colorSystem
			}
		case "click_show":
			if i == g.clickBlock {
				clr = colorPlayer
			}
		case "replay_correct":
			if g.correctSeqIdx < len(g.sequence) && i == g.sequence[g.correctSeqIdx] {
				clr = colorSystem
			}
		}

		common.DrawRect(screen, b.x, b.y, g.bsz, g.bsz, clr)
	}

	var instText string
	switch g.state {
	case "show_flash", "show_gap":
		instText = fmt.Sprintf("Obserwuj sekwencję... (długość: %d)", g.curSeqLen)
	case "player_turn", "click_show":
		instText = "Powtórz sekwencję klikając na kwadraty."
	case "feedback":
		instText = "Powtórz sekwencję klikając na kwadraty."
	case "replay_correct":
		instText = "Poprawna sekwencja:"
	}
	common.DrawTextCentered(screen, instText, float64(common.ScreenW)/2, 60, common.FontSmall, common.White)

	if g.state == "feedback" || g.state == "replay_correct" {
		common.DrawTextCentered(screen, g.fbText, float64(common.ScreenW)/2, float64(common.ScreenH)-80, common.FontMedium, g.fbClr)
	}

	if g.state == "player_turn" {
		remaining := responseTimeout - time.Since(g.respStart).Seconds()
		if remaining < 0 {
			remaining = 0
		}
		common.DrawTextLeft(screen, fmt.Sprintf("%.1fs", remaining), 50, float64(common.ScreenH)-40, common.FontSmall, common.Gray)
	}
}

func (g *Game) writeResults() {
	avgRT := 0.0
	if g.rtCnt > 0 {
		avgRT = (g.rtSum / float64(g.rtCnt)) * 1000
	}
	accuracy := 0.0
	if g.totalTrials > 0 {
		accuracy = float64(g.correctTrials) / float64(g.totalTrials) * 100
	}

	data := resultsData{
		TestID:              "Corsi",
		SubjectID:           g.subjectId,
		Timestamp:           g.timestamp,
		IloscPoprawnych:     g.correctTrials,
		IloscBlednych:       g.totalTrials - g.correctTrials,
		OgolnaIlosc:         g.totalTrials,
		SredniCzasReakcji:   avgRT,
		MaxDlugoscSekwencji: g.maxCorrectLen,
		PoziomTrudnosci:     diffs[g.diffIdx].label,
		RozmiarSiatki:       fmt.Sprintf("%dx%d", g.gridSize, g.gridSize),
		SzybkoscSwiecenia:   flashCfg[g.flashIdx].label,
		Score: fmt.Sprintf("Max: %d | Poprawne: %d/%d | Skuteczność: %.0f%% | Śr. RT: %.0fms",
			g.maxCorrectLen, g.correctTrials, g.totalTrials, accuracy, avgRT),
		Statystyki: statsData{
			Poprawne:          g.correctTrials,
			Bledne:            g.totalTrials - g.correctTrials,
			WszystkieProby:    g.totalTrials,
			Skutecznosc:       accuracy,
			MaxSekwencja:      g.maxCorrectLen,
			SredniCzasMs:      avgRT,
			Poziom:            g.diffIdx,
			RozmiarSiatki:     g.gridSize,
			SzybkoscSwiecenia: g.flashIdx,
		},
		WynikiSzczegolowe: g.trialData,
	}

	if err := common.WriteResults(".", data); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *Game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}
