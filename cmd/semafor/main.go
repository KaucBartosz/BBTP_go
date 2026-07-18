package main

import (
	"embed"
	"fmt"
	"image/color"
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
	gridN        = 8
	nTrials      = 20
	trialTimeout = 15.0
	feedbackTime = 0.5
	animPeriod   = 7.5
	lampHU       = 0.08
)

const (
	stateMenu = iota
	statePresentation
	stateInstructions
	stateTrial
	stateFeedback
	stateDone
)

type lampKind int

const (
	lampHidden lampKind = iota
	lampGreenOff
	lampDarkOff
	lampGreenOn
	lampLit
)

var gc [gridN]float64

var corners = map[[2]int]bool{
	{0, 0}: true, {0, gridN - 1}: true,
	{gridN - 1, 0}: true, {gridN - 1, gridN - 1}: true,
}

var (
	colorGreenLine  = color.RGBA{38, 242, 38, 255}
	colorYellowFill = color.RGBA{255, 224, 0, 255}
	colorInfoYellow = color.RGBA{255, 255, 115, 255}
	colorHoverGreen = color.RGBA{89, 255, 115, 255}
	colorHoverBlue  = color.RGBA{115, 191, 255, 255}
	colorPrezTitle  = color.RGBA{166, 166, 166, 255}
	colorGrayBack   = color.RGBA{26, 26, 26, 255}
	colorGoldBack   = color.RGBA{255, 191, 51, 255}
	colorCursor     = color.RGBA{230, 230, 230, 255}
)

type trialResult struct {
	XEdge    string `json:"x_edge"`
	YEdge    string `json:"y_edge"`
	XIndex   int    `json:"x_index"`
	YIndex   int    `json:"y_index"`
	TargetX  int    `json:"target_x"`
	TargetY  int    `json:"target_y"`
	ClickedX *int   `json:"clicked_x"`
	ClickedY *int   `json:"clicked_y"`
	Rt       *int   `json:"rt"`
	Correct  int    `json:"correct"`
}

type resultsFile struct {
	TestID            string         `json:"testId"`
	SubjectID         string         `json:"subjectId"`
	Timestamp         string         `json:"timestamp"`
	IloscPoprawnych   int            `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych     int            `json:"ilosc_blednych_nacisniec"`
	IloscBlednychKlik int            `json:"ilosc_blednych_klikniec"`
	OgolnaIlosc       int            `json:"ogolna_ilosc_nacisniec"`
	IloscBrakow       int            `json:"ilosc_brakow_nacisniec"`
	SredniCzas        int            `json:"sredni_czas_reakcji"`
	Score             string         `json:"score"`
	Statystyki        map[string]int `json:"statystyki"`
	Wyniki            []trialResult  `json:"wyniki"`
}

type game struct {
	state int

	imgOFF     *ebiten.Image
	imgZielOFF *ebiten.Image
	imgZielON  *ebiten.Image
	imgON      *ebiten.Image
	circYellow *ebiten.Image
	circWhite  *ebiten.Image

	lamp [gridN][gridN]lampKind

	menuReleased bool
	menuOver1     bool
	menuOver2     bool

	animStart    time.Time
	presLoop     int
	presLampOn   bool
	presBackOver bool

	trialIdx       int
	trialStart     time.Time
	feedbackStart  time.Time
	mouseReleased bool
	showFeedback  bool
	escaped       bool
	xEdge         string
	yEdge         string
	xIdx          int
	yIdx          int
	targetX       int
	targetY       int
	clickedX      int
	clickedY      int
	rtMs          int
	correct       int

	trials []trialResult
	startupTicks int
}

func main() {
	rand.Seed(time.Now().UnixNano())
	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}
	for i := 0; i < gridN; i++ {
		gc[i] = (float64(i) - 3.5) * lampHU
	}

	g := &game{
		state:    stateMenu,
		imgOFF:   common.LoadImageFS(resources, "lampkaOFF.png"),
		imgZielOFF: common.LoadImageFS(resources, "lampkaZielOFF.png"),
		imgZielON:  common.LoadImageFS(resources, "lampkaZielON.png"),
		imgON:      common.LoadImageFS(resources, "lampkaON.png"),
		circYellow: makeCircle(128, colorYellowFill),
		circWhite:  makeCircle(128, colorCursor),
	}

	ebiten.SetWindowTitle("Semafor")
	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetFullscreen(true)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func makeCircle(d int, c color.Color) *ebiten.Image {
	img := ebiten.NewImage(d, d)
	r := float64(d) / 2
	for y := 0; y < d; y++ {
		for x := 0; x < d; x++ {
			dx := float64(x) + 0.5 - r
			dy := float64(y) + 0.5 - r
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
	return img
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func smoothStep(x float64) float64 {
	s := clamp01(x)
	return s * s * (3 - 2*s)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func overRect(mx, my, cx, cy, hw, hh int) bool {
	dx := mx - cx
	if dx < 0 {
		dx = -dx
	}
	dy := my - cy
	if dy < 0 {
		dy = -dy
	}
	return dx <= hw && dy <= hh
}

func overRectF(mx, my int, cx, cy, hw, hh float64) bool {
	dx := float64(mx) - cx
	if dx < 0 {
		dx = -dx
	}
	dy := float64(my) - cy
	if dy < 0 {
		dy = -dy
	}
	return dx <= hw && dy <= hh
}

func (g *game) resetLamp() {
	for ix := 0; ix < gridN; ix++ {
		for iy := 0; iy < gridN; iy++ {
			if corners[[2]int{ix, iy}] {
				g.lamp[ix][iy] = lampHidden
			} else if ix == 0 || ix == gridN-1 || iy == 0 || iy == gridN-1 {
				g.lamp[ix][iy] = lampGreenOff
			} else {
				g.lamp[ix][iy] = lampDarkOff
			}
		}
	}
}

func (g *game) startDemo() {
	g.state = statePresentation
	g.animStart = time.Now()
	g.presLoop = 0
	g.presLampOn = false
	g.resetLamp()
	g.lamp[3][0] = lampGreenOn
	g.lamp[0][5] = lampGreenOn
}

func (g *game) setupTrial() {
	g.resetLamp()

	if rand.Float64() < 0.5 {
		g.xEdge = "top"
	} else {
		g.xEdge = "bottom"
	}
	if rand.Float64() < 0.5 {
		g.yEdge = "left"
	} else {
		g.yEdge = "right"
	}

	yEdgeRow := 0
	if g.xEdge == "bottom" {
		yEdgeRow = gridN - 1
	}
	xEdgeCol := 0
	if g.yEdge == "right" {
		xEdgeCol = gridN - 1
	}

	for {
		g.xIdx = rand.Intn(gridN)
		for g.xIdx == xEdgeCol {
			g.xIdx = rand.Intn(gridN)
		}
		g.yIdx = rand.Intn(gridN)
		for g.yIdx == yEdgeRow {
			g.yIdx = rand.Intn(gridN)
		}
		if !corners[[2]int{g.xIdx, yEdgeRow}] && !corners[[2]int{xEdgeCol, g.yIdx}] {
			break
		}
	}

	g.targetX = g.xIdx
	g.targetY = g.yIdx

	g.lamp[g.xIdx][yEdgeRow] = lampGreenOn
	g.lamp[xEdgeCol][g.yIdx] = lampGreenOn

	g.clickedX = -1
	g.clickedY = -1
	g.rtMs = -1
	g.correct = 0
	g.showFeedback = false
	g.mouseReleased = false
	g.trialStart = time.Now()
}

func (g *game) startTest() {
	g.trialIdx = 0
	g.correct = 0
	g.trials = nil
	g.escaped = false
	g.setupTrial()
	g.state = stateTrial
}

func (g *game) writeResults() {
	totalClicks := 0
	correctCount := 0
	noAnswer := 0
	rtSum := 0
	rtCount := 0

	for _, t := range g.trials {
		if t.ClickedX != nil {
			totalClicks++
		}
		if t.Correct == 1 {
			correctCount++
		}
		if t.Rt != nil {
			rtSum += *t.Rt
			rtCount++
		}
	}
	noAnswer = len(g.trials) - totalClicks
	wrongClicks := totalClicks - correctCount
	totalErrors := wrongClicks + noAnswer
	avgRt := 0
	if rtCount > 0 {
		avgRt = rtSum / rtCount
	}
	accuracy := 0
	if len(g.trials) > 0 {
		accuracy = correctCount * 100 / len(g.trials)
	}

	score := fmt.Sprintf(
		"Kliknięć: %d | Poprawne: %d | Błędne (w tym brak odp.): %d | Brak odp.: %d | Skuteczność: %d%% | Śr. RT: %d ms",
		totalClicks, correctCount, totalErrors, noAnswer, accuracy, avgRt,
	)

	data := resultsFile{
		TestID:            "semafor",
		SubjectID:         common.RandomID(),
		Timestamp:         common.Timestamp(),
		IloscPoprawnych:   correctCount,
		IloscBlednych:     totalErrors,
		IloscBlednychKlik: wrongClicks,
		OgolnaIlosc:       totalClicks,
		IloscBrakow:       noAnswer,
		SredniCzas:        avgRt,
		Score:             score,
		Statystyki: map[string]int{
			"poprawne":          correctCount,
			"bledne_lacznie":    totalErrors,
			"bledne_klikniecia": wrongClicks,
			"brak_odpowiedzi":   noAnswer,
			"wszystkie_kliki":   totalClicks,
			"proby":             len(g.trials),
			"skutecznosc_proc":  accuracy,
		},
		Wyniki: g.trials,
	}

	if err := 	common.WriteResults(".", data); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *game) clickLamp(mx, my int) bool {
	for ix := 0; ix < gridN; ix++ {
		for iy := 0; iy < gridN; iy++ {
			if g.lamp[ix][iy] == lampHidden {
				continue
			}
			sz := common.HS(lampHU)
			cx, cy := common.H(gc[ix], gc[iy])
			hw := sz / 2
			hh := sz / 2
			if math.Abs(float64(mx)-cx) <= hw && math.Abs(float64(my)-cy) <= hh {
				g.clickedX = ix
				g.clickedY = iy
				g.rtMs = int(time.Since(g.trialStart).Milliseconds())
				if ix == g.targetX && iy == g.targetY {
					g.correct = 1
					g.lamp[ix][iy] = lampLit
				} else {
					g.correct = 0
				}
				g.showFeedback = true
				g.mouseReleased = false
				g.feedbackStart = time.Now()
				return true
			}
		}
	}
	return false
}

func (g *game) recordTrial() {
	var cx, cy *int
	var rt *int
	if g.clickedX >= 0 {
		cx = &g.clickedX
		cy = &g.clickedY
	}
	if g.rtMs >= 0 {
		rt = &g.rtMs
	}
	g.trials = append(g.trials, trialResult{
		XEdge:    g.xEdge,
		YEdge:    g.yEdge,
		XIndex:   g.xIdx,
		YIndex:   g.yIdx,
		TargetX:  g.targetX,
		TargetY:  g.targetY,
		ClickedX: cx,
		ClickedY: cy,
		Rt:       rt,
		Correct:  g.correct,
	})
}

func (g *game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		switch g.state {
		case stateMenu:
			return ebiten.Termination
		case statePresentation, stateInstructions:
			g.state = stateMenu
			g.menuReleased = false
		case stateTrial, stateFeedback:
			g.recordTrial()
			g.writeResults()
			return ebiten.Termination
		case stateDone:
			return ebiten.Termination
		}
	}

	switch g.state {
	case stateMenu:
		g.updateMenu()
	case statePresentation:
		g.updatePresentation()
	case stateInstructions:
		g.updateInstructions()
	case stateTrial:
		g.updateTrial()
	case stateFeedback:
		g.updateFeedback()
	}
	return nil
}

func (g *game) updateMenu() {
	mx, my := ebiten.CursorPosition()

	g.menuOver1 = overRectF(mx, my, float64(common.ScreenW)/2, float64(common.ScreenH)*0.44, 324, 54)
	g.menuOver2 = overRectF(mx, my, float64(common.ScreenW)/2, float64(common.ScreenH)*0.60, 324, 54)

	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.menuReleased = true
	}

	if g.menuReleased && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.menuOver1 {
			g.startTest()
			return
		}
		if g.menuOver2 {
			g.startDemo()
			g.menuReleased = false
			return
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.Key1) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.startTest()
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.startDemo()
		g.menuReleased = false
	}
}

func (g *game) updatePresentation() {
	mx, my := ebiten.CursorPosition()

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		g.state = stateMenu
		g.menuReleased = false
		return
	}

	rawAt := time.Since(g.animStart).Seconds()
	at := math.Mod(rawAt, animPeriod)
	loopN := int(rawAt / animPeriod)
	if loopN > g.presLoop {
		g.presLoop = loopN
		g.resetLamp()
		g.lamp[3][0] = lampGreenOn
		g.lamp[0][5] = lampGreenOn
		g.presLampOn = false
	}

	if at >= 5.9 && !g.presLampOn {
		g.lamp[3][5] = lampLit
		g.presLampOn = true
	}

	g.presBackOver = overRectF(mx, my, float64(common.ScreenW)/2, float64(common.ScreenH)*0.962, 238, 27)

	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.menuReleased = true
	}
	if g.menuReleased && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && g.presBackOver {
		g.state = stateMenu
		g.menuReleased = false
	}
}

func (g *game) updateInstructions() {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.state = stateTrial
		g.setupTrial()
	}
}

func (g *game) updateTrial() {
	elapsed := time.Since(g.trialStart).Seconds()

	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.mouseReleased = true
	}

	if !g.showFeedback && g.mouseReleased && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		g.clickLamp(mx, my)
	}

	if g.showFeedback && time.Since(g.feedbackStart).Seconds() >= feedbackTime {
		g.recordTrial()
		g.trialIdx++
		if g.trialIdx >= nTrials {
			g.writeResults()
			g.state = stateDone
		} else {
			g.setupTrial()
		}
		return
	}

	if !g.showFeedback && elapsed >= trialTimeout {
		g.recordTrial()
		g.trialIdx++
		if g.trialIdx >= nTrials {
			g.writeResults()
			g.state = stateDone
		} else {
			g.setupTrial()
		}
	}
}

func (g *game) updateFeedback() {
	if time.Since(g.feedbackStart).Seconds() >= feedbackTime {
		g.recordTrial()
		g.trialIdx++
		if g.trialIdx >= nTrials {
			g.writeResults()
			g.state = stateDone
		} else {
			g.setupTrial()
			g.state = stateTrial
		}
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)

	switch g.state {
	case stateMenu:
		g.drawMenu(screen)
	case statePresentation:
		g.drawPresentation(screen)
	case stateInstructions:
		g.drawInstructions(screen)
	case stateTrial, stateFeedback:
		g.drawTrial(screen)
	case stateDone:
		g.drawDone(screen)
	}
}

func (g *game) drawLamp(screen *ebiten.Image, img *ebiten.Image, ix, iy int) {
	sz := common.HS(lampHU)
	cx, cy := common.H(gc[ix], gc[iy])
	common.DrawImage(screen, img, cx-sz/2, cy-sz/2, sz, sz, 1.0)
}

func (g *game) drawGrid(screen *ebiten.Image) {
	for ix := 0; ix < gridN; ix++ {
		for iy := 0; iy < gridN; iy++ {
			switch g.lamp[ix][iy] {
			case lampHidden:
				continue
			case lampGreenOff:
				g.drawLamp(screen, g.imgZielOFF, ix, iy)
			case lampDarkOff:
				g.drawLamp(screen, g.imgOFF, ix, iy)
			case lampGreenOn:
				g.drawLamp(screen, g.imgZielON, ix, iy)
			case lampLit:
				g.drawLamp(screen, g.imgON, ix, iy)
			}
		}
	}
}

func (g *game) drawCircle(screen *ebiten.Image, img *ebiten.Image, cx, cy, radiusPx, alpha float64) {
	if alpha <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	s := radiusPx * 2 / float64(img.Bounds().Dx())
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(cx-radiusPx, cy-radiusPx)
	op.ColorScale.ScaleAlpha(float32(alpha))
	screen.DrawImage(img, op)
}

func (g *game) drawCenteredRect(screen *ebiten.Image, cx, cy, w, h float64, clr color.Color) {
	common.DrawRect(screen, cx-w/2, cy-h/2, w, h, clr)
}

func (g *game) drawMenu(screen *ebiten.Image) {
	cx := float64(common.ScreenW / 2)

	titleY := float64(common.ScreenH) * 0.22
	opt1Y := float64(common.ScreenH) * 0.44
	opt2Y := float64(common.ScreenH) * 0.60
	hintY := float64(common.ScreenH) * 0.78

	common.DrawTextCentered(screen, "Semafor", cx, titleY, common.FontBig, common.White)

	clr1 := common.White
	if g.menuOver1 {
		clr1 = colorHoverGreen
	}
	clr2 := common.White
	if g.menuOver2 {
		clr2 = colorHoverBlue
	}

	common.DrawTextCentered(screen, "1.  Test", cx, opt1Y, common.FontMedium, clr1)
	common.DrawTextCentered(screen, "2.  Prezentacja", cx, opt2Y, common.FontMedium, clr2)
	common.DrawTextCentered(screen, "[ klawisze:  1  lub  2 ]", cx, hintY, common.FontTiny, common.DarkGray)
}

func (g *game) drawPresentation(screen *ebiten.Image) {
	rawAt := time.Since(g.animStart).Seconds()
	at := math.Mod(rawAt, animPeriod)

	common.DrawTextCentered(screen, "— Prezentacja —", float64(common.ScreenW/2), float64(common.ScreenH)*0.06, common.FontSmall, colorPrezTitle)

	g.drawGrid(screen)

	demoTx, demoTy := common.H(gc[3], gc[5])

	lineAlpha := clamp01((at-1.0)/1.5) * 0.85 * clamp01(1.0-(at-7.0)/0.5)
	if lineAlpha > 0 {
		lc := color.RGBA{38, 242, 38, uint8(lineAlpha * 255)}
		hW := common.HS(0.68)
		hH := common.HS(0.006)
		g.drawCenteredRect(screen, float64(common.ScreenW/2), demoTy, hW, hH, lc)

		vW := common.HS(0.006)
		vH := common.HS(0.68)
		g.drawCenteredRect(screen, demoTx, float64(common.ScreenH/2), vW, vH, lc)
	}

	circAlpha := 0.0
	if at >= 2.5 && at < 4.6 {
		circAlpha = math.Max(0, 0.6+0.4*math.Sin(at*math.Pi*3.0))
	}
	g.drawCircle(screen, g.circYellow, demoTx, demoTy, common.HS(0.048), circAlpha)

	cursorAlpha := 0.0
	if at >= 4.5 && at < 6.2 {
		moveT := smoothStep(clamp01((at - 4.5) / 1.3))
		startPx, startPy := common.H(0.38, 0.36)
		curX := lerp(startPx, demoTx, moveT)
		curY := lerp(startPy, demoTy, moveT)
		clickFade := clamp01((at - 5.80) / 0.15)
		clickReturn := clamp01((at - 5.95) / 0.15)
		cursorAlpha = 1.0 - clickFade*0.85 + clickReturn*0.85
		g.drawCircle(screen, g.circWhite, curX, curY, common.HS(0.026), cursorAlpha)
	}

	var info string
	switch {
	case at < 1.0:
		info = "Dwie lampki zapalają się na zielono..."
	case at < 2.5:
		info = "Każda wyznacza linię przez całą siatkę..."
	case at < 4.5:
		info = "Wskaż lampkę na przecięciu tych linii!"
	case at < 5.9:
		info = "Kliknij myszką w to miejsce!"
	default:
		info = "✓  To jest prawidłowa odpowiedź!"
	}
	infoY := float64(common.ScreenH) * 0.90
	common.DrawTextCentered(screen, info, float64(common.ScreenW/2), infoY, common.FontSmall, colorInfoYellow)

	backY := float64(common.ScreenH) * 0.962
	backClr := colorGrayBack
	if g.presBackOver {
		backClr = colorGoldBack
	}
	common.DrawTextCentered(screen, "← Wróć do menu", float64(common.ScreenW/2), backY, common.FontTiny, backClr)
}

func (g *game) drawInstructions(screen *ebiten.Image) {
	cx := float64(common.ScreenW / 2)
	cy := float64(common.ScreenH / 2)

	common.DrawTextCentered(screen, "Semafor", cx, cy-200, common.FontBig, common.Orange)

	instr := "Za chwilę zobaczysz planszę z lampkami. Twoim zadaniem będzie,\nza pomocą MYSZY, wskazać tę lampkę, która znajduje się na przecięciu\nprostych dwóch lampek zapalonych na zielono. Staraj się klikać\nnajszybciej jak potrafisz. Aby rozpocząć zadanie, wciśnij SPACJĘ."
	common.DrawTextCentered(screen, instr, cx, cy, common.FontMedium, common.White)
}

func (g *game) drawTrial(screen *ebiten.Image) {
	g.drawGrid(screen)

	elapsed := time.Since(g.trialStart).Seconds()
	remaining := trialTimeout - elapsed
	if remaining < 0 {
		remaining = 0
	}
	timerText := fmt.Sprintf("%.1fs", remaining)
	common.DrawTextLeft(screen, timerText, 50, float64(common.ScreenH)-80, common.FontSmall, common.Gray)

	trialText := fmt.Sprintf("Próba: %d / %d", g.trialIdx+1, nTrials)
	common.DrawTextLeft(screen, trialText, 50, 50, common.FontSmall, common.Gray)
}

func (g *game) drawDone(screen *ebiten.Image) {
	cx := float64(common.ScreenW / 2)
	cy := float64(common.ScreenH / 2)

	correctCount := 0
	for _, t := range g.trials {
		if t.Correct == 1 {
			correctCount++
		}
	}

	text := fmt.Sprintf("Koniec testu!\n\nPoprawne: %d / %d\nWyniki zapisane.", correctCount, len(g.trials))
	common.DrawTextCentered(screen, text, cx, cy, common.FontBig, common.Green)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return common.ScreenW, common.ScreenH
}
