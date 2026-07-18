package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"BBTP_go/common"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	testDuration        = 120.0
	paddleSpeed         = 0.5
	paddleWidth         = 0.02
	ballRadius          = 0.015
	wallX              = 0.5
	paddleX            = 0.45
	wallThickness      = 0.005
	maxSpeedMultiplier = 4.0
	speedIncInterval   = 1.5
	speedIncAmount     = 0.2
	survSpeedIncInt    = 3.0
	survSpeedIncAmt    = 0.1
)

type difficulty struct {
	key            string
	label          string
	baseSpeed      float64
	paddleHeight   float64
	isSurvival     bool
	hasSpeedGrowth bool
}

var diffMap = map[string]difficulty{
	"1": {key: "Easy", label: "Łatwy", baseSpeed: 0.005, paddleHeight: 0.25},
	"2": {key: "Normal", label: "Normalny", baseSpeed: 0.0096, paddleHeight: 0.20},
	"3": {key: "Hard", label: "Trudny", baseSpeed: 0.0096, paddleHeight: 0.18, hasSpeedGrowth: true},
	"4": {key: "Survival", label: "Przetrwanie", baseSpeed: 0.0096, paddleHeight: 0.18, isSurvival: true, hasSpeedGrowth: true},
}

type statsJSON struct {
	LewaSciana      int     `json:"lewa_sciana"`
	PrawaSciana     int     `json:"prawa_sciana"`
	OdbiciaPaletka  int     `json:"odbicia_paletka"`
	MaxPredkoscX    float64 `json:"max_predkosc_x"`
	ZmianyPredkosci int     `json:"zmiany_predkosci"`
}

type resultsJSON struct {
	TestID          string    `json:"testId"`
	SubjectID       string    `json:"subjectId"`
	Timestamp       string    `json:"timestamp"`
	IloscPoprawnych int       `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych   int       `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc     int       `json:"ogolna_ilosc_nacisniec"`
	PoziomTrudnosci string    `json:"poziom_trudnosci"`
	CzasTrwaniaSek  int       `json:"czas_trwania_sek"`
	Score           string    `json:"score"`
	Statystyki      statsJSON `json:"statystyki"`
}

type game struct {
	state     string
	subjectId string
	timestamp string

	diff difficulty

	leftPaddleY, rightPaddleY float64
	ballX, ballY              float64
	ballVX, ballVY            float64

	paddleHits     int
	leftWallHits   int
	rightWallHits  int
	totalWallHits  int
	speedMultiplier float64
	speedChanges    int
	maxSpeedReached float64

	lastPaddleHitTime float64
	elapsedTime       float64
	survivalTime      float64

	prevTime time.Time
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

	now := time.Now()
	dt := now.Sub(g.prevTime).Seconds()
	g.prevTime = now
	if dt > 0.1 {
		dt = 0.016
	}

	switch g.state {
	case "welcome":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.state = "difficulty"
		}
	case "difficulty":
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.startGame("1")
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.startGame("2")
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.startGame("3")
		} else if inpututil.IsKeyJustPressed(ebiten.Key4) {
			g.startGame("4")
		}
	case "playing":
		g.updatePlaying(dt)
	case "done":
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			return ebiten.Termination
		}
	}
	return nil
}

func (g *game) startGame(key string) {
	g.diff = diffMap[key]
	g.leftPaddleY = 0
	g.rightPaddleY = 0
	g.paddleHits = 0
	g.leftWallHits = 0
	g.rightWallHits = 0
	g.totalWallHits = 0
	g.speedMultiplier = 1.0
	g.speedChanges = 0
	g.maxSpeedReached = 1.0
	g.lastPaddleHitTime = 0
	g.elapsedTime = 0
	g.survivalTime = 0
	g.resetBall()
	g.state = "playing"
}

func (g *game) resetBall() {
	g.ballX = 0
	g.ballY = 0
	hDir := 1.0
	if rand.Float64() > 0.5 {
		hDir = -1.0
	}
	vAngle := (rand.Float64() - 0.5) * 0.5
	speed := g.diff.baseSpeed * g.speedMultiplier
	g.ballVX = hDir * speed * math.Cos(vAngle)
	g.ballVY = speed * math.Sin(vAngle)
}

func (g *game) updateBallSpeed() {
	current := math.Sqrt(g.ballVX*g.ballVX + g.ballVY*g.ballVY)
	newSpeed := g.diff.baseSpeed * g.speedMultiplier
	if current > 0 {
		g.ballVX = (g.ballVX / current) * newSpeed
		g.ballVY = (g.ballVY / current) * newSpeed
	}
}

func (g *game) updatePlaying(dt float64) {
	frameDt := dt * 60.0
	g.elapsedTime += dt

	pHalfH := g.diff.paddleHeight / 2
	pMove := paddleSpeed * dt

	if ebiten.IsKeyPressed(ebiten.KeyW) {
		g.leftPaddleY = common.Clamp(g.leftPaddleY+pMove, -0.5+pHalfH, 0.5-pHalfH)
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		g.leftPaddleY = common.Clamp(g.leftPaddleY-pMove, -0.5+pHalfH, 0.5-pHalfH)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyO) {
		g.rightPaddleY = common.Clamp(g.rightPaddleY+pMove, -0.5+pHalfH, 0.5-pHalfH)
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyL) {
		g.rightPaddleY = common.Clamp(g.rightPaddleY-pMove, -0.5+pHalfH, 0.5-pHalfH)
	}

	g.ballX += g.ballVX * frameDt
	g.ballY += g.ballVY * frameDt

	if g.ballY+ballRadius >= 0.5 {
		g.ballY = 0.5 - ballRadius
		g.ballVY = -math.Abs(g.ballVY)
	}
	if g.ballY-ballRadius <= -0.5 {
		g.ballY = -0.5 + ballRadius
		g.ballVY = math.Abs(g.ballVY)
	}

	pw := paddleWidth

	if g.ballVX < 0 &&
		g.ballX-ballRadius <= -paddleX+pw/2 &&
		g.ballX+ballRadius >= -paddleX-pw/2 &&
		g.leftPaddleY-pHalfH <= g.ballY &&
		g.ballY <= g.leftPaddleY+pHalfH {
		g.ballX = -paddleX + pw/2 + ballRadius
		g.ballVX = math.Abs(g.ballVX)
		hitPos := (g.ballY - g.leftPaddleY) / pHalfH
		g.ballVY += hitPos * 0.003
		g.paddleHits++
		g.onPaddleHit()
	}

	if g.ballVX > 0 &&
		g.ballX+ballRadius >= paddleX-pw/2 &&
		g.ballX-ballRadius <= paddleX+pw/2 &&
		g.rightPaddleY-pHalfH <= g.ballY &&
		g.ballY <= g.rightPaddleY+pHalfH {
		g.ballX = paddleX - pw/2 - ballRadius
		g.ballVX = -math.Abs(g.ballVX)
		hitPos := (g.ballY - g.rightPaddleY) / pHalfH
		g.ballVY += hitPos * 0.003
		g.paddleHits++
		g.onPaddleHit()
	}

	if g.ballX-ballRadius <= -wallX {
		g.leftWallHits++
		g.totalWallHits++
		if g.diff.isSurvival {
			g.survivalTime = g.elapsedTime
			g.writeResults()
			g.state = "done"
			return
		}
		if g.diff.hasSpeedGrowth {
			g.speedMultiplier = 1.0
			g.lastPaddleHitTime = g.elapsedTime
		}
		g.resetBall()
	}

	if g.ballX+ballRadius >= wallX {
		g.rightWallHits++
		g.totalWallHits++
		if g.diff.isSurvival {
			g.survivalTime = g.elapsedTime
			g.writeResults()
			g.state = "done"
			return
		}
		if g.diff.hasSpeedGrowth {
			g.speedMultiplier = 1.0
			g.lastPaddleHitTime = g.elapsedTime
		}
		g.resetBall()
	}

	if g.diff.hasSpeedGrowth && !g.diff.isSurvival {
		timeSince := g.elapsedTime - g.lastPaddleHitTime
		if timeSince >= speedIncInterval {
			newMult := g.speedMultiplier + speedIncAmount
			if newMult > maxSpeedMultiplier {
				newMult = maxSpeedMultiplier
			}
			if newMult != g.speedMultiplier {
				g.speedMultiplier = newMult
				g.speedChanges++
				if g.speedMultiplier > g.maxSpeedReached {
					g.maxSpeedReached = g.speedMultiplier
				}
				g.lastPaddleHitTime = g.elapsedTime
				g.updateBallSpeed()
			}
		}
	} else if g.diff.hasSpeedGrowth && g.diff.isSurvival {
		timeSince := g.elapsedTime - g.lastPaddleHitTime
		if timeSince >= survSpeedIncInt {
			g.speedMultiplier += survSpeedIncAmt
			g.speedChanges++
			if g.speedMultiplier > g.maxSpeedReached {
				g.maxSpeedReached = g.speedMultiplier
			}
			g.lastPaddleHitTime = g.elapsedTime
			g.updateBallSpeed()
		}
	}

	if !g.diff.isSurvival && g.elapsedTime >= testDuration {
		g.writeResults()
		g.state = "done"
	}
}

func (g *game) onPaddleHit() {
	if g.diff.hasSpeedGrowth && !g.diff.isSurvival {
		g.speedMultiplier = 1.0
		g.lastPaddleHitTime = g.elapsedTime
		g.updateBallSpeed()
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(common.Black)
	cx := float64(common.ScreenW) / 2
	cy := float64(common.ScreenH) / 2

	switch g.state {
	case "welcome":
		common.DrawTextCentered(screen,
			"PING PONG – Test Koordynacji\n\n"+
				"Twoim zadaniem jest odbijanie piłki za pomocą dwóch paletek.\n\n"+
				"LEWA PALETKA: klawisze W (góra) i S (dół)\n"+
				"PRAWA PALETKA: strzałki góra i dół lub klawisze O i L\n\n"+
				"Test trwa 2 minuty (lub bez limitu w trybie Przetrwania).\n"+
				"Odbijaj piłkę jak najdłużej!\n\n"+
				"Naciśnij SPACJĘ, aby wybrać poziom trudności\n"+
				"ESC – wyjście bez zapisu",
			cx, cy, common.FontSmall, common.White)
	case "difficulty":
		common.DrawTextCentered(screen,
			"WYBIERZ POZIOM TRUDNOŚCI\n\n"+
				"1 – ŁATWY       (wolniejsza piłka, większe paletki)\n"+
				"2 – NORMALNY    (standardowa prędkość)\n"+
				"3 – TRUDNY      (prędkość rośnie z czasem)\n"+
				"4 – PRZETRWANIE (jeden błąd = koniec, bez limitu czasu)\n\n"+
				"Naciśnij 1, 2, 3 lub 4",
			cx, cy, common.FontSmall, common.White)
	case "playing":
		g.drawPlaying(screen)
	case "done":
		g.drawDone(screen)
	}
}

func (g *game) drawPlaying(screen *ebiten.Image) {
	cx := float64(common.ScreenW) / 2

	wallPx := common.HS(wallThickness)
	lwCx, _ := common.H(-wallX, 0)
	common.DrawRect(screen, lwCx-wallPx/2, 0, wallPx, float64(common.ScreenH), common.Red)
	rwCx, _ := common.H(wallX, 0)
	common.DrawRect(screen, rwCx-wallPx/2, 0, wallPx, float64(common.ScreenH), common.Red)

	lpW := common.HS(paddleWidth)
	lpH := common.HS(g.diff.paddleHeight)
	lpCx, lpCy := common.H(-paddleX, g.leftPaddleY)
	common.DrawRect(screen, lpCx-lpW/2, lpCy-lpH/2, lpW, lpH, common.White)

	rpCx, rpCy := common.H(paddleX, g.rightPaddleY)
	common.DrawRect(screen, rpCx-lpW/2, rpCy-lpH/2, lpW, lpH, common.White)

	bR := common.HS(ballRadius)
	bCx, bCy := common.H(g.ballX, g.ballY)
	common.DrawCircleFilled(screen, bCx, bCy, bR, common.White)

	var displayTime float64
	if g.diff.isSurvival {
		displayTime = g.elapsedTime
	} else {
		displayTime = testDuration - g.elapsedTime
	}
	mins := int(displayTime) / 60
	secs := int(displayTime) % 60
	timerStr := fmt.Sprintf("%02d:%02d", mins, secs)

	_, timerCy := common.H(0, 0.45)
	common.DrawTextCentered(screen, timerStr, cx, timerCy, common.FontMedium, common.White)
}

func (g *game) drawDone(screen *ebiten.Image) {
	cx := float64(common.ScreenW) / 2
	cy := float64(common.ScreenH) / 2

	if g.diff.isSurvival {
		st := g.survivalTime
		sMins := int(st) / 60
		sSecs := int(st) % 60
		common.DrawTextCentered(screen,
			fmt.Sprintf("KONIEC TESTU – TRYB PRZETRWANIA\n\n"+
				"Czas przeżycia: %02d:%02d\n"+
				"Odbicia paletką: %d\n\n"+
				"Naciśnij SPACJĘ, aby zakończyć",
				sMins, sSecs, g.paddleHits),
			cx, cy, common.FontSmall, common.White)
	} else {
		common.DrawTextCentered(screen,
			fmt.Sprintf("KONIEC TESTU\n\n"+
				"Poziom: %s\n"+
				"Odbicia paletką: %d\n"+
				"Przepuszczone (lewa): %d\n"+
				"Przepuszczone (prawa): %d\n"+
				"Przepuszczone razem: %d\n\n"+
				"Naciśnij SPACJĘ, aby zakończyć",
				g.diff.label, g.paddleHits, g.leftWallHits, g.rightWallHits, g.totalWallHits),
			cx, cy, common.FontSmall, common.White)
	}
}

func (g *game) writeResults() {
	wallHits := g.totalWallHits

	var scoreText string
	var czasTrwania int
	var iloscBlednych int

	if g.diff.isSurvival {
		scoreText = fmt.Sprintf("Poziom: %s | Czas: %.1fs | Odbicia paletką: %d",
			g.diff.label, g.survivalTime, g.paddleHits)
		czasTrwania = int(math.Round(g.survivalTime))
		iloscBlednych = 0
	} else {
		scoreText = fmt.Sprintf("Poziom: %s | Odbicia paletką: %d | Przepuszczone: %d | Czas: %ds",
			g.diff.label, g.paddleHits, wallHits, int(math.Round(g.elapsedTime)))
		czasTrwania = int(math.Round(g.elapsedTime))
		iloscBlednych = wallHits
	}

	total := g.paddleHits
	if !g.diff.isSurvival {
		total = g.paddleHits + wallHits
	}

	data := resultsJSON{
		TestID:          "PingPong",
		SubjectID:       g.subjectId,
		Timestamp:       g.timestamp,
		IloscPoprawnych: g.paddleHits,
		IloscBlednych:   iloscBlednych,
		OgolnaIlosc:     total,
		PoziomTrudnosci: g.diff.label,
		CzasTrwaniaSek:  czasTrwania,
		Score:           scoreText,
		Statystyki: statsJSON{
			LewaSciana:      g.leftWallHits,
			PrawaSciana:     g.rightWallHits,
			OdbiciaPaletka:  g.paddleHits,
			MaxPredkoscX:    math.Round(g.maxSpeedReached*100) / 100,
			ZmianyPredkosci: g.speedChanges,
		},
	}

	if err := common.WriteResults(".", data); err != nil {
		log.Printf("Error writing results: %v", err)
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
		state:     "welcome",
		subjectId: common.RandomID(),
		timestamp: common.Timestamp(),
		prevTime:  time.Now(),
	}

	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetWindowTitle("PingPong - Test Koordynacji")
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
