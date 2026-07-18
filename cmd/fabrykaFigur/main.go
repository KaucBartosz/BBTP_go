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
	stateInstructions = iota
	stateDifficulty
	stateDuration
	stateRunning
	stateDraining
	stateDone
)

const (
	sizeTop        = 0.22
	sizeBottom     = 0.14
	baseSpeed      = 0.25
	targetRatio    = 0.4
	targetInterval = 20.0
	flashDuration  = 1.5
	numRows        = 2
	shapesPerRow   = 6
	seqLen         = 500
)

var (
	allShapes  = []string{"kw", "ko", "tr", "gw", "pk"}
	allColors  = []string{"RED", "BLU", "GRE", "YEL", "BLA"}
	diffNames  = []string{"Łatwy", "Średni", "Trudny"}
	diffShapes = []int{3, 4, 5}
	diffColors = []int{3, 4, 5}
	diffSpeeds = []float64{1.0, 1.2, 1.35}
	durations  = []int{40, 180, 300}
)

type scrollShape struct {
	x       float64
	y       float64
	img     *ebiten.Image
	imgName string
	alpha   float64
	target  bool
	active  bool
	counted bool
	drained bool
}

type detailedResult struct {
	Time      float64 `json:"time"`
	StimImage string  `json:"stim_image"`
	IsCorrect int     `json:"is_correct"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}

type statsData struct {
	Poprawne              int     `json:"poprawne"`
	Bledne                int     `json:"bledne"`
	WszystkieKliki        int     `json:"wszystkie_kliki"`
	ObiektyDoKlikniecia   int     `json:"obiekty_do_klikniecia"`
	PominieteCele         int     `json:"pominiete_cele"`
	SkutecznoscKlikniec   float64 `json:"skutecznosc_klikniec"`
	SkutecznoscWykrywania float64 `json:"skutecznosc_wykrywania"`
	PoziomTrudnosci       string  `json:"poziom_trudnosci"`
	CzasTrwaniaSek        int     `json:"czas_trwania_sek"`
}

type resultsData struct {
	TestID            string           `json:"testId"`
	SubjectID         string           `json:"subjectId"`
	Timestamp         string           `json:"timestamp"`
	IloscPoprawnych   int              `json:"ilosc_poprawnych_nacisniec"`
	IloscBlednych     int              `json:"ilosc_blednych_nacisniec"`
	OgolnaIlosc       int              `json:"ogolna_ilosc_nacisniec"`
	IloscObiektow     int              `json:"ilosc_obiektow_do_klikniecia"`
	PominieteCele     int              `json:"pominiete_cele"`
	PoziomTrudnosci   string           `json:"poziom_trudnosci"`
	CzasTrwania       int              `json:"czas_trwania"`
	Score             string           `json:"score"`
	Statystyki        statsData        `json:"statystyki"`
	WynikiSzczegolowe []detailedResult `json:"wyniki_szczegolowe"`
}

type game struct {
	state      int
	difficulty int
	duration   int
	speedMult  float64

	images map[string]*ebiten.Image

	target1    string
	target2    string
	targetPool []string
	nonTargets []string

	shapes   [numRows][shapesPerRow]*scrollShape
	sequence [numRows][]string
	seqIdx   [numRows]int

	shapeSize float64
	topSize   float64

	targetX [2]float64
	targetY float64
	rowY    [numRows]float64

	testStart        time.Time
	lastUpdateTime   time.Time
	lastTargetChange time.Time
	flashStart       time.Time
	flashActive      bool
	doneTime         time.Time

	correct           int
	incorrect         int
	totalClicks       int
	targetAppearances int
	missedTargets     int
	detailedResults   []detailedResult

	drainForEnd  bool
	drainedCount int
	startupTicks int
}

func loadImageCache() map[string]*ebiten.Image {
	cache := make(map[string]*ebiten.Image)
	for _, s := range allShapes {
		for _, c := range allColors {
			name := s + c + ".png"
			cache[name] = common.LoadImageFS(resources, name)
		}
	}
	return cache
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if err := common.InitFonts(); err != nil {
		log.Fatal(err)
	}

	images := loadImageCache()

	g := &game{
		state:  stateInstructions,
		images: images,
	}

	g.shapeSize = common.HS(sizeBottom)
	g.topSize = common.HS(sizeTop)

	txLeft, _ := common.H(-0.25, 0.32)
	txRight, _ := common.H(0.25, 0.32)
	_, ty := common.H(0, 0.32)
	g.targetX[0] = txLeft - g.topSize/2
	g.targetX[1] = txRight - g.topSize/2
	g.targetY = ty - g.topSize/2

	yPsy := []float64{-0.10, -0.30}
	for r := 0; r < numRows; r++ {
		_, ry := common.H(0, yPsy[r])
		g.rowY[r] = ry - g.shapeSize/2
	}

	g.lastUpdateTime = time.Now()

	ebiten.SetWindowTitle("Fabryka Figur")
	ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
	ebiten.SetFullscreen(true)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func (g *game) selectTargets() {
	g.targetPool = nil
	nS := diffShapes[g.difficulty]
	nC := diffColors[g.difficulty]

	for i := 0; i < nS; i++ {
		for j := 0; j < nC; j++ {
			name := allShapes[i] + allColors[j] + ".png"
			g.targetPool = append(g.targetPool, name)
		}
	}

	idx := rand.Perm(len(g.targetPool))
	g.target1 = g.targetPool[idx[0]]
	g.target2 = g.targetPool[idx[1]]

	g.nonTargets = nil
	for _, name := range g.targetPool {
		if name != g.target1 && name != g.target2 {
			g.nonTargets = append(g.nonTargets, name)
		}
	}
	if len(g.nonTargets) == 0 {
		for _, s := range allShapes {
			for _, c := range allColors {
				name := s + c + ".png"
				if name != g.target1 && name != g.target2 {
					g.nonTargets = append(g.nonTargets, name)
				}
			}
		}
	}
}

func (g *game) generateSequences() {
	for r := 0; r < numRows; r++ {
		seq := make([]string, seqLen)
		targetCount := int(float64(seqLen) * targetRatio)
		for i := 0; i < targetCount; i++ {
			if rand.Float64() < 0.5 {
				seq[i] = g.target1
			} else {
				seq[i] = g.target2
			}
		}
		for i := targetCount; i < seqLen; i++ {
			seq[i] = g.nonTargets[rand.Intn(len(g.nonTargets))]
		}
		rand.Shuffle(len(seq), func(i, j int) { seq[i], seq[j] = seq[j], seq[i] })
		g.sequence[r] = seq
		g.seqIdx[r] = 0
	}
}

func (g *game) spawnShapes() {
	for r := 0; r < numRows; r++ {
		for i := 0; i < shapesPerRow; i++ {
			name := g.sequence[r][g.seqIdx[r]]
			g.seqIdx[r]++
			
			slotX := (-0.525 + float64(i)*0.21)*1080.0 + 960.0
			spawnX := slotX - 1944.0
			
			g.shapes[r][i] = &scrollShape{
				x:       spawnX,
				y:       g.rowY[r],
				img:     g.images[name],
				imgName: name,
				alpha:   1.0,
				target:  name == g.target1 || name == g.target2,
				active:  true,
				counted: false,
				drained: false,
			}
		}
	}
}

func (g *game) startTest() {
	g.correct = 0
	g.incorrect = 0
	g.totalClicks = 0
	g.targetAppearances = 0
	g.missedTargets = 0
	g.detailedResults = nil

	g.speedMult = diffSpeeds[g.difficulty]

	g.selectTargets()
	g.generateSequences()
	g.spawnShapes()

	g.testStart = time.Now()
	g.lastTargetChange = time.Now()
	g.flashStart = time.Now()
	g.flashActive = true
	g.lastUpdateTime = time.Now()
	g.state = stateRunning
}

func (g *game) nextImage(r int) string {
	idx := g.seqIdx[r] % len(g.sequence[r])
	name := g.sequence[r][idx]
	g.seqIdx[r]++
	return name
}

func (g *game) drainComplete() {
	if g.drainForEnd {
		g.state = stateDone
		g.doneTime = time.Now()
		g.writeResults()
		return
	}

	g.selectTargets()
	g.generateSequences()

	// Re-spawn all shapes off-screen left
	for r := 0; r < numRows; r++ {
		for i := 0; i < shapesPerRow; i++ {
			name := g.nextImage(r)
			slotX := (-0.525 + float64(i)*0.21)*1080.0 + 960.0
			spawnX := slotX - 1944.0
			
			g.shapes[r][i] = &scrollShape{
				x:       spawnX,
				y:       g.rowY[r],
				img:     g.images[name],
				imgName: name,
				alpha:   1.0,
				target:  name == g.target1 || name == g.target2,
				active:  true,
				counted: false,
				drained: false,
			}
		}
	}

	g.lastTargetChange = time.Now()
	g.flashStart = time.Now()
	g.flashActive = true
	g.lastUpdateTime = time.Now()
	g.state = stateRunning
}

func (g *game) Update() error {
	g.startupTicks++
	if g.startupTicks < 30 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.state == stateRunning || g.state == stateDraining {
			g.writeResults()
		}
		return ebiten.Termination
	}

	now := time.Now()
	dt := now.Sub(g.lastUpdateTime).Seconds()
	if dt > 0.1 {
		dt = 0.1
	}
	g.lastUpdateTime = now

	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	switch g.state {
	case stateInstructions:
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.state = stateDifficulty
		}

	case stateDifficulty:
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.difficulty = 0
			g.state = stateDuration
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.difficulty = 1
			g.state = stateDuration
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.difficulty = 2
			g.state = stateDuration
		}

	case stateDuration:
		if inpututil.IsKeyJustPressed(ebiten.Key1) {
			g.duration = durations[0]
			g.startTest()
		} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
			g.duration = durations[1]
			g.startTest()
		} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
			g.duration = durations[2]
			g.startTest()
		}

	case stateRunning, stateDraining:
		elapsed := time.Since(g.testStart).Seconds()
		if g.state == stateRunning {
			if elapsed >= float64(g.duration) {
				g.drainForEnd = true
				g.state = stateDraining
				g.drainedCount = 0
				for r := 0; r < numRows; r++ {
					for i := 0; i < shapesPerRow; i++ {
						g.shapes[r][i].drained = false
					}
				}
				break
			}

			if time.Since(g.lastTargetChange).Seconds() >= targetInterval {
				g.drainForEnd = false
				g.state = stateDraining
				g.drainedCount = 0
				for r := 0; r < numRows; r++ {
					for i := 0; i < shapesPerRow; i++ {
						g.shapes[r][i].drained = false
					}
				}
				break
			}
		}

		if g.flashActive && time.Since(g.flashStart).Seconds() >= flashDuration {
			g.flashActive = false
		}

		speed := common.HS(baseSpeed) * g.speedMult
		for r := 0; r < numRows; r++ {
			for i := 0; i < shapesPerRow; i++ {
				s := g.shapes[r][i]
				if s.drained {
					continue
				}
				prevX := s.x
				s.x += speed * dt

				// Appearance crossing: center-line crossing 960.0
				if prevX < 960.0 && s.x >= 960.0 && !s.counted {
					if s.target && s.active {
						g.targetAppearances++
					}
					s.counted = true
				}

				// Wrap threshold checking (1753.8 pixels)
				if s.x > 1753.8 {
					if !s.drained && s.active && s.counted && s.target {
						g.missedTargets++
					}

					if g.state == stateDraining {
						if !s.drained {
							s.alpha = 0.0
							s.imgName = "__blank__"
							s.active = false
							s.counted = true
							s.drained = true
							s.x = -10000.0
							g.drainedCount++
						}
					} else {
						// Normal wrap
						name := g.nextImage(r)
						s.x -= 1360.8
						s.img = g.images[name]
						s.imgName = name
						s.alpha = 1.0
						s.target = name == g.target1 || name == g.target2
						s.active = true
						s.counted = false
						s.drained = false
					}
				}
			}
		}

		if g.state == stateDraining && g.drainedCount >= 12 {
			g.drainComplete()
			break
		}

		if (g.state == stateRunning || g.state == stateDraining) && clicked {
			mx, my := ebiten.CursorPosition()
			mxf, myf := float64(mx), float64(my)
			hitR := g.shapeSize * 1.3 / 2

			// Find clicked shape
			var bestShape *scrollShape
			minDist := hitR * hitR
			for r := 0; r < numRows; r++ {
				for i := 0; i < shapesPerRow; i++ {
					s := g.shapes[r][i]
					if !s.active || s.drained {
						continue
					}
					cx := s.x + g.shapeSize/2
					cy := s.y + g.shapeSize/2
					dx := mxf - cx
					dy := myf - cy
					dist := dx*dx + dy*dy
					if dist < minDist {
						minDist = dist
						bestShape = s
					}
				}
			}

			if bestShape != nil {
				bestShape.active = false
				g.totalClicks++
				elTime := time.Since(g.testStart).Seconds()

				isCorrect := 0
				if bestShape.target {
					g.correct++
					bestShape.alpha = 0.0
					isCorrect = 1
				} else {
					g.incorrect++
					bestShape.alpha = 0.2
				}

				// Convert cursor coordinates to PsychoPy height coordinates
				clickX := (mxf - 960.0) / 1080.0
				clickY := (540.0 - myf) / 1080.0

				g.detailedResults = append(g.detailedResults, detailedResult{
					Time:      math.Round(elTime*1000) / 1000,
					StimImage: bestShape.imgName,
					IsCorrect: isCorrect,
					X:         math.Round(clickX*100) / 100,
					Y:         math.Round(clickY*100) / 100,
				})
			}
		}

	case stateDone:
		if time.Since(g.doneTime).Seconds() >= 2.5 {
			return ebiten.Termination
		}
	}

	return nil
}

func (g *game) writeResults() {
	poprawne := g.correct
	bledne := g.incorrect
	wszystkie := g.totalClicks
	obiekty := g.targetAppearances
	pominiete := g.missedTargets

	skutKlik := 0.0
	if wszystkie > 0 {
		skutKlik = math.Round(float64(poprawne)/float64(wszystkie)*10000) / 100
	}
	skutWykr := 0.0
	if obiekty > 0 {
		skutWykr = math.Round(float64(poprawne)/float64(obiekty)*10000) / 100
	}

	score := fmt.Sprintf("Poprawne: %d | Bledne: %d | Skutecznosc: %.1f%%", poprawne, bledne, skutKlik)

	data := resultsData{
		TestID:          "FabrykaFigur",
		SubjectID:       common.RandomID(),
		Timestamp:       common.Timestamp(),
		IloscPoprawnych: poprawne,
		IloscBlednych:   bledne,
		OgolnaIlosc:     wszystkie,
		IloscObiektow:   obiekty,
		PominieteCele:   pominiete,
		PoziomTrudnosci: diffNames[g.difficulty],
		CzasTrwania:     g.duration,
		Score:           score,
		Statystyki: statsData{
			Poprawne:              poprawne,
			Bledne:                bledne,
			WszystkieKliki:        wszystkie,
			ObiektyDoKlikniecia:   obiekty,
			PominieteCele:         pominiete,
			SkutecznoscKlikniec:   skutKlik,
			SkutecznoscWykrywania: skutWykr,
			PoziomTrudnosci:       diffNames[g.difficulty],
			CzasTrwaniaSek:        g.duration,
		},
		WynikiSzczegolowe: g.detailedResults,
	}

	if err := 	common.WriteResults(".", data); err != nil {
		log.Printf("Error writing results: %v", err)
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	cx := float64(common.ScreenW) / 2

	if g.state == stateRunning || g.state == stateDraining {
		screen.Fill(common.Black)
	} else {
		// Clean modern light background
		screen.Fill(color.RGBA{240, 242, 245, 255})
	}

	switch g.state {
	case stateInstructions:
		common.DrawTextCentered(screen, "Fabryka Figur", cx, 200, common.FontBig, color.RGBA{220, 80, 0, 255})

		instrText := "Za chwilę na ekranie zobaczysz serię różnych figur.\n\n" +
			"Za pomocą MYSZY, klikaj na te figury, których kształt i kolor\n" +
			"odpowiada wzorcowi przedstawionemu u góry ekranu.\n\n" +
			"Wzorzec co jakiś czas będzie się zmieniał. Zawsze należy klikać\n" +
			"na te figury, których kształt i kolor odpowiada aktualnemu wzorcowi.\n\n" +
			"Staraj się reagować najszybciej jak potrafisz.\n\n" +
			"Aby rozpocząć zadanie, wciśnij SPACJĘ."

		common.DrawTextCentered(screen, instrText, cx, 580, common.FontMedium, color.RGBA{40, 40, 48, 255})

	case stateDifficulty:
		common.DrawTextCentered(screen, "Poziom Trudności", cx, 200, common.FontBig, color.RGBA{220, 80, 0, 255})

		diffText := "Wybierz poziom trudności:\n\n" +
			"1 – Łatwy (3 figury, 3 kolory)\n" +
			"2 – Średni (4 figury, 4 kolory, +20% prędkości)\n" +
			"3 – Trudny (5 figur, 5 kolorów, +35% prędkości)\n\n" +
			"Naciśnij 1, 2 lub 3."

		common.DrawTextCentered(screen, diffText, cx, 580, common.FontMedium, color.RGBA{40, 40, 48, 255})

	case stateDuration:
		common.DrawTextCentered(screen, "Czas Trwania", cx, 200, common.FontBig, color.RGBA{220, 80, 0, 255})

		durText := "Wybierz czas trwania testu:\n\n" +
			"1 – 40 sekund\n" +
			"2 – 180 sekund\n" +
			"3 – 300 sekund\n\n" +
			"Naciśnij 1, 2 lub 3."

		common.DrawTextCentered(screen, durText, cx, 580, common.FontMedium, color.RGBA{40, 40, 48, 255})

	case stateRunning, stateDraining:
		for t := 0; t < 2; t++ {
			imgName := g.target1
			if t == 1 {
				imgName = g.target2
			}
			img := g.images[imgName]
			common.DrawImage(screen, img, g.targetX[t], g.targetY, g.topSize, g.topSize, 1.0)
		}

		if g.flashActive {
			flashAge := time.Since(g.flashStart).Seconds()
			opacity := 1.0 - (flashAge / flashDuration)
			if opacity < 0 {
				opacity = 0
			}
			yellowWithAlpha := color.RGBA{255, 255, 0, uint8(opacity * 255)}

			// Single outline enclosing both targets
			rectW := 1296.0
			rectH := 324.0
			rectX := 960.0 - rectW/2
			rectY := 194.4 - rectH/2
			common.DrawRectOutline(screen, rectX, rectY, rectW, rectH, yellowWithAlpha, 4)
		}

		for r := 0; r < numRows; r++ {
			for i := 0; i < shapesPerRow; i++ {
				s := g.shapes[r][i]
				if s == nil || s.alpha <= 0 || s.drained {
					continue
				}
				common.DrawImage(screen, s.img, s.x, s.y, g.shapeSize, g.shapeSize, s.alpha)
			}
		}

		counterText := fmt.Sprintf("Poprawne: %d", g.correct)
		common.DrawTextCentered(screen, counterText,
			float64(common.ScreenW)-180, 50, common.FontMedium, common.White)

	case stateDone:
		common.DrawTextCentered(screen, "Koniec Testu", cx, 400, common.FontBig, color.RGBA{0, 150, 0, 255})
		common.DrawTextCentered(screen, "Wyniki zostały pomyślnie zapisane.", cx, 600, common.FontMedium, color.RGBA{40, 40, 48, 255})
	}
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return common.ScreenW, common.ScreenH
}
