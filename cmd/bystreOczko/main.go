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
)

//go:embed resources/*
var resources embed.FS

const (
	STATE_INSTRUCTIONS = iota
	STATE_LENGTH
	STATE_TRIAL
	STATE_FEEDBACK
	STATE_INTER_TRIAL
	STATE_DONE
)

const (
	ROWS     = 3
	COLS     = 6
	RT_LIMIT = 3.0
	ONSETMIN = 1.0
	ONSETMAX = 2.0
	FBDUR    = 0.5
)

type trialResult struct {
	GreenOnset float64  `json:"greenOnset"`
	TargetRow  int      `json:"target_row"`
	TargetCol  int      `json:"target_col"`
	ClickedRow *int     `json:"clicked_row"`
	ClickedCol *int     `json:"clicked_col"`
	Rt         *float64 `json:"rt"`
	Correct    int      `json:"correct"`
}

type resultsData struct {
	TestID            string        `json:"testId"`
	SubjectID         string        `json:"subjectId"`
	Timestamp         string        `json:"timestamp"`
	IloscPoprawnych   int           `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych     int           `json:"ilosc_blednych_nacisniec"`
	IloscBrakow       int           `json:"ilosc_brakow_nacisniec"`
	OgolnaIlosc       int           `json:"ogolna_ilosc_nacisniec"`
	IloscObiektow     int           `json:"ilosc_obiektow_do_klikniecia"`
	SredniCzasReakcji float64       `json:"sredni_czas_reakcji"`
	Score             string        `json:"score"`
	Wyniki            []trialResult `json:"wyniki"`
}

type game struct {
	state    int
	imgRed   *ebiten.Image
	imgGreen *ebiten.Image
	imgNeut  *ebiten.Image

	tlX [COLS]float64
	tlY [ROWS]float64
	lSz float64

	nTrials    int
	curTrial   int
	results    []trialResult
	tStart     time.Time
	greenOnset float64
	tgtRow     int
	tgtCol     int
	greenOn    bool
	responded  bool
	clickRow   int
	clickCol   int
	rt         float64
	fbStart    time.Time
	itStart    time.Time
	startupTicks int
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	lSz := common.HS(0.12)

	g := &game{
		state:    STATE_INSTRUCTIONS,
		imgRed:   common.LoadImageFS(resources, "sygCzer.png"),
		imgGreen: common.LoadImageFS(resources, "sygZiel.png"),
		imgNeut:  common.LoadImageFS(resources, "syg.png"),
		lSz:      lSz,
	}

	xPsy := make([]float64, COLS)
	for i := 0; i < COLS; i++ {
		xPsy[i] = (float64(i) - float64(COLS-1)/2.0) * 0.18
	}
	yPsy := []float64{0.2, 0.0, -0.2}
	for c := 0; c < COLS; c++ {
		cx, _ := common.H(xPsy[c], 0)
		g.tlX[c] = cx - lSz/2
	}
	for r := 0; r < ROWS; r++ {
		_, cy := common.H(0, yPsy[r])
		g.tlY[r] = cy - lSz/2
	}

	ebiten.SetWindowTitle("Bystre Oczko")
	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func (g *game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.state >= STATE_TRIAL && g.state <= STATE_INTER_TRIAL {
			g.writeResults()
		}
		return ebiten.Termination
	}

	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	switch g.state {
	case STATE_INSTRUCTIONS:
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.state = STATE_LENGTH
		}

	case STATE_LENGTH:
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.nTrials = 10
			g.beginTest()
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.nTrials = 30
			g.beginTest()
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.nTrials = 60
			g.beginTest()
		}

	case STATE_TRIAL:
		t := time.Since(g.tStart).Seconds()

		if !g.greenOn && t >= g.greenOnset {
			g.greenOn = true
		}

		if g.greenOn && !g.responded {
			greenT := t - g.greenOnset
			if greenT >= RT_LIMIT {
				g.responded = true
				g.rt = RT_LIMIT
				g.recordTrial()
				g.state = STATE_FEEDBACK
				g.fbStart = time.Now()
				return nil
			}

			if clicked {
				mx, my := ebiten.CursorPosition()
				mxf, myf := float64(mx), float64(my)
				for r := 0; r < ROWS; r++ {
					for c := 0; c < COLS; c++ {
						if common.PointInRect(mxf, myf, g.tlX[c], g.tlY[r], g.lSz, g.lSz) {
							g.responded = true
							g.clickRow = r
							g.clickCol = c
							g.rt = greenT
							g.recordTrial()
							g.state = STATE_FEEDBACK
							g.fbStart = time.Now()
							return nil
						}
					}
				}
			}
		}

	case STATE_FEEDBACK:
		if time.Since(g.fbStart) >= time.Duration(FBDUR*float64(time.Second)) {
			g.state = STATE_INTER_TRIAL
			g.itStart = time.Now()
		}

	case STATE_INTER_TRIAL:
		mx, my := ebiten.CursorPosition()
		sx := float64(common.ScreenW)/2 - 25
		sy := float64(common.ScreenH)/2 - 25
		if common.PointInRect(float64(mx), float64(my), sx, sy, 50, 50) {
			g.nextTrial()
		}
	}
	return nil
}

func (g *game) beginTest() {
	g.curTrial = 0
	g.results = nil
	g.nextTrial()
}

func (g *game) nextTrial() {
	if g.curTrial >= g.nTrials {
		g.state = STATE_DONE
		g.writeResults()
		return
	}
	g.tStart = time.Now()
	g.greenOnset = ONSETMIN + rand.Float64()*(ONSETMAX-ONSETMIN)
	g.tgtRow = rand.Intn(ROWS)
	g.tgtCol = rand.Intn(COLS)
	g.greenOn = false
	g.responded = false
	g.clickRow = -1
	g.clickCol = -1
	g.rt = 0
	g.state = STATE_TRIAL
}

func (g *game) recordTrial() {
	correct := 0
	if g.clickRow == g.tgtRow && g.clickCol == g.tgtCol {
		correct = 1
	}
	tr := trialResult{
		GreenOnset: g.greenOnset,
		TargetRow:  g.tgtRow,
		TargetCol:  g.tgtCol,
		Correct:    correct,
	}
	if g.clickRow >= 0 {
		cr := g.clickRow
		cc := g.clickCol
		tr.ClickedRow = &cr
		tr.ClickedCol = &cc
	}
	if g.responded {
		rt := g.rt
		tr.Rt = &rt
	}
	g.results = append(g.results, tr)
	g.curTrial++
}

func (g *game) writeResults() {
	n := len(g.results)
	correct := 0
	wrong := 0
	noResp := 0
	totalClicks := 0
	sumRT := 0.0

	for _, r := range g.results {
		if r.Rt != nil {
			sumRT += *r.Rt
		} else {
			sumRT += RT_LIMIT
		}
		if r.ClickedRow != nil {
			totalClicks++
			if r.Correct == 1 {
				correct++
			} else {
				wrong++
			}
		} else {
			noResp++
		}
	}

	avgRT := 0.0
	if n > 0 {
		avgRT = math.Round(sumRT / float64(n) * 1000)
	}

	score := fmt.Sprintf("Poprawne: %d | Bledne: %d | Braki: %d | Sr. RT: %.0f ms", correct, wrong, noResp, avgRT)

	data := resultsData{
		TestID:            "bystreOczko",
		SubjectID:         common.RandomID(),
		Timestamp:         common.Timestamp(),
		IloscPoprawnych:   correct,
		IloscBlednych:     wrong,
		IloscBrakow:       noResp,
		OgolnaIlosc:       totalClicks,
		IloscObiektow:     n,
		SredniCzasReakcji: avgRT,
		Score:             score,
		Wyniki:            g.results,
	}

	if err := 	common.WriteResults(".", data); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *game) drawGrid(screen *ebiten.Image) {
	for r := 0; r < ROWS; r++ {
		for c := 0; c < COLS; c++ {
			img := g.imgRed
			if g.state == STATE_TRIAL && g.greenOn && r == g.tgtRow && c == g.tgtCol {
				img = g.imgGreen
			} else if g.state == STATE_FEEDBACK || g.state == STATE_INTER_TRIAL {
				img = g.imgNeut
			}
			common.DrawImage(screen, img, g.tlX[c], g.tlY[r], g.lSz, g.lSz, 1.0)
		}
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)
	cx := float64(common.ScreenW) / 2

	switch g.state {
	case STATE_INSTRUCTIONS:
		common.DrawTextCentered(screen, "Bystre Oczko", cx, 100, common.FontBig, common.Orange)
		common.DrawTextCentered(screen,
			"Za chwilę na ekranie pojawią się sygnalizacje świetlne.\n"+
				"Twoim zadaniem jest, za pomocą MYSZY, kliknąć na tę\n"+
				"z sygnalizacji, w której światło zmieni kolor na zielony.\n"+
				"Staraj się reagować najszybciej jak potrafisz.\n\n"+
				"Gdy sygnalizacje zgasną, umieść kursor myszki na małym\n"+
				"czerwonym kwadracie na środku ekranu, aby rozpocząć\n"+
				"kolejną rundę.\n\n"+
				"Aby rozpocząć zadanie, wciśnij SPACJĘ.",
			cx, 420, common.FontMedium, common.White)

	case STATE_LENGTH:
		common.DrawTextCentered(screen,
			"Wybierz ilość rund:\n\n"+
				"1 - 10 rund\n"+
				"2 - 30 rund\n"+
				"3 - 60 rund\n\n"+
				"Naciśnij odpowiedni klawisz.",
			cx, float64(common.ScreenH/2), common.FontMedium, common.White)

	case STATE_TRIAL:
		g.drawGrid(screen)

	case STATE_FEEDBACK:
		g.drawGrid(screen)

	case STATE_INTER_TRIAL:
		g.drawGrid(screen)
		sx := float64(common.ScreenW)/2 - 25
		sy := float64(common.ScreenH)/2 - 25
		common.DrawRect(screen, sx, sy, 50, 50, common.Red)

	case STATE_DONE:
		common.DrawTextCentered(screen,
			"Koniec testu!\nWyniki zapisane.",
			cx, float64(common.ScreenH/2), common.FontBig, common.Green)
	}
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}
