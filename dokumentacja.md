# Dokumentacja standardów testów psychologicznych — BBTP_go

## 1. Wprowadzenie

Projekt BBTP_go zawiera 13 natywnych testów psychologicznych napisanych w Go z użyciem
silnika Ebitengine v2. Testy są uruchamiane przez launcher (Electron), który po zakończeniu
testu odczytuje plik `results.json` i przekazuje wyniki do renderera.

Niniejszy dokument opisuje wszystkie wymagania, normy i standardy, jakie musi spełniać
każdy test, aby poprawnie integrować się z resztą systemu.

---

## 2. Struktura projektu

### 2.1. Układ katalogów

```
BBTP_go/
├── common/                  # Biblioteka współdzielona
│   ├── ui.go               #   Funkcje rysowania, LoadImageFS, stałe ekranu
│   ├── font.go             #   Czcionki, DrawTextCentered, H/HS
│   ├── font.ttf            #   Plik czcionki (osadzony)
│   └── results.go          #   WriteResults, RandomID, Timestamp
├── cmd/
│   ├── bystreOczko/        # Test 1
│   │   ├── main.go
│   │   └── resources/      #   Obrazy (opcjonalnie)
│   ├── corsi/              # Test 2
│   │   └── main.go
│   ├── fabrykaFigur/       # Test 3
│   │   ├── main.go
│   │   └── resources/
│   ├── goNoGo/             # Test 4
│   │   └── main.go
│   ├── goNoGoBiegacze/     # Test 5
│   │   └── main.go
│   ├── nwstecz/            # Test 6
│   │   └── main.go
│   ├── pingPong/           # Test 7
│   │   └── main.go
│   ├── samochodzik/        # Test 8
│   │   ├── main.go
│   │   └── resources/      #   trasa.png, trasa2.png, sam.png
│   ├── semafor/            # Test 9
│   │   ├── main.go
│   │   └── resources/      #   lampka*.png
│   ├── stop/               # Test 10
│   │   ├── main.go
│   │   └── resources/      #   tlo.png, car.png, stop.png, jelonek.png, drzewo.png
│   ├── stroop/             # Test 11
│   │   └── main.go
│   ├── sygnalizacja/       # Test 12
│   │   ├── main.go
│   │   └── resources/      #   sygCzer.png, sygZiel.png, sam.png
│   └── zlapSygnal/         # Test 13
│       ├── main.go
│       └── resources/      #   car.png, stop.png, sygCzer.png, sygZiel.png
├── .github/workflows/
│   └── build.yml           # CI/CD: 5 platform
├── go.mod
├── go.sum
└── .gitignore
```

### 2.2. Moduł Go

```
module BBTP_go
go 1.26.5

require (
    github.com/hajimehoshi/ebiten/v2 v2.9.9
    golang.org/x/image v0.44.0
)
```

- Każdy test to osobny katalog w `cmd/` z `package main` i funkcją `main()`.
- Wspólny kod biblioteczny znajduje się w pakiecie `common/`.
- Nie ma plików `Makefile`, `*.sh`, `*.ps1` — budowanie odbywa się wyłącznie przez
  `go build` oraz CI/CD przez GitHub Actions.

---

## 3. Minimalna struktura testu

### 3.1. Funkcja `main()`

```go
func main() {
    if err := common.InitFonts(); err != nil {
        log.Fatal(err)
    }

    g := &game{...}

    ebiten.SetWindowTitle("Nazwa Testu")
    ebiten.SetWindowSize(common.ScreenW, common.ScreenH)
    ebiten.SetFullscreen(true)

    if err := ebiten.RunGame(g); err != nil {
        log.Fatal(err)
    }
}
```

Wymagania:
- Wywołanie `common.InitFonts()` — inicjalizuje czcionki (48pt, 36pt, 28pt, 20pt).
- `ebiten.SetFullscreen(true)` — test zawsze działa na pełnym ekranie.
- `ebiten.SetWindowSize(common.ScreenW, common.ScreenH)` — 1920×1080.
- Funkcja `main()` musi znajdować się w `package main`.

### 3.2. Struktura implementująca `ebiten.Game`

```go
type game struct {
    state        string  // lub int z iota
    subjectId    string
    timestamp    string
    startupTicks int
    // ... pola specyficzne dla testu
}
```

- Nazwa struktury: `game` lub `Game` (konsekwentnie w całym teście).
- `startupTicks int` — licznik klatek pomijanych przy starcie (patrz pkt 4.2).

### 3.3. Metoda `Layout()`

```go
func (g *game) Layout(outsideW, outsideH int) (int, int) {
    return common.ScreenW, common.ScreenH
}
```

Identyczna we wszystkich testach. Zwraca stały rozmiar 1920×1080 niezależnie od
rzeczywistego rozmiaru okna (fullscreen).

### 3.4. Metoda `Update()`

```go
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
        // ...
    case "trial":
        // ...
    case "done":
        // ...
    }
    return nil
}
```

Wymagania:
- `startupTicks` — pierwsze 30 klatek (~0.5s) pomijane, aby zapobiec rejestracji
  przypadkowych klawiszy podczas uruchamiania.
- `Escape` — zawsze kończy test. Przed zakończeniem wywołuje `writeResults()`.
- `return ebiten.Termination` — NIE `os.Exit()`, NIE `log.Fatal()`.
- Implementacja maszyny stanów: `string` (7 testów) lub `int` z `iota` (6 testów).

### 3.5. Metoda `Draw()`

```go
func (g *game) Draw(screen *ebiten.Image) {
    // Rysowanie w zależności od g.state
    switch g.state {
    case "instruction":
        g.drawInstruction(screen)
    case "trial":
        g.drawTrial(screen)
    // ...
    }
}
```

---

## 4. Standardy obsługi wejścia

### 4.1. Klawisze specjalne

| Klawisz | Działanie |
|---------|-----------|
| `Escape` | Zakończenie testu z zapisem wyników |
| `Space` | Rozpoczęcie testu / przejście dalej |
| `1`, `2` | Wybór poziomu trudności (jeśli dotyczy) |
| `Enter` | Potwierdzenie (opcjonalnie) |

### 4.2. Pomijanie klatek startowych

```go
g.startupTicks++
if g.startupTicks < 30 {
    return nil
}
```

Zapobiega rejestracji klawiszy wciśniętych przed przejęciem focusa przez okno testu.
Wymagane we WSZYSTKICH testach.

### 4.3. Obsługa myszy

Do sprawdzania kliknięć myszy:
```go
if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
    mx, my := ebiten.CursorPosition()
    // ...
}
```

Współrzędne kursora są w pikselach ekranu (0–1920, 0–1080).

---

## 5. Zasoby osadzone (Embedded Resources)

### 5.1. Deklaracja

```go
import "embed"

//go:embed resources/*
var resources embed.FS
```

- Umieszczone na poziomie pakietu (package-level), zaraz za importami.
- Działa tylko dla plików w podkatalogu `resources/` względem pliku źródłowego.
- Nie wymaga `os.Open`, `filepath.Join` ani żadnej ścieżki dyskowej.

### 5.2. Ładowanie obrazów

```go
img := common.LoadImageFS(resources, "nazwa_pliku.png")
```

- Funkcja automatycznie dodaje prefix `"resources/"`.
- Zwraca `*ebiten.Image` gotowy do rysowania.
- W razie błędu loguje ostrzeżenie i zwraca obraz 1×1 (bez panic).
- Obsługuje tylko pliki PNG.

### 5.3. Zaawansowane: dostęp do surowych bajtów

Gdy potrzebny jest dostęp do pikseli (jak w `samochodzik`):

```go
data, err := resources.ReadFile("resources/trasa.png")
if err != nil {
    log.Fatal(err)
}
img, err := png.Decode(bytes.NewReader(data))
```

### 5.4. Testy bez zasobów

Testy, które nie używają obrazów (corsi, goNoGo, goNoGoBiegacze, nwstecz, pingPong,
stroop), nie mają katalogu `resources/` i nie używają `//go:embed`.

### 5.5. Czcionka

Czcionka jest osadzona raz, w `common/font.go`:
```go
//go:embed font.ttf
var fontData []byte
```

Żaden test nie powinien samodzielnie ładować czcionki. Używa się gotowych fontów
z `common`:

| Zmienna | Rozmiar | Zastosowanie |
|---------|---------|-------------|
| `common.FontBig` | 48pt | Tytuły, duże instrukcje |
| `common.FontMedium` | 36pt | Instrukcje, komunikaty |
| `common.FontSmall` | 28pt | Etykiety, podpisy |
| `common.FontTiny` | 20pt | Dane szczegółowe |

---

## 6. Układ współrzędnych (PsychoPy / Height Units)

Projekt używa systemu współrzędnych wzorowanego na PsychoPy: jednostki wysokości
(Height Units) z osią Y skierowaną w górę i środkiem ekranu w punkcie (0, 0).

### 6.1. Konwersja

```go
func H(x, y float64) (float64, float64) {
    return x*1080 + 960, -y*1080 + 540
}

func HS(s float64) float64 {
    return s * 1080
}
```

- `H(x, y)` — konwertuje współrzędne PsychoPy na piksele ekranu.
- `HS(s)` — konwertuje rozmiar w HU na piksele.
- Ekran: 1920×1080, środek w (960, 540).

### 6.2. Przykłady

```go
// Środek ekranu
cx, cy := common.H(0, 0)   // → (960, 540)

// Lewy górny róg (w HU: -0.5, 0.5)
lx, ly := common.H(-0.5, 0.5)

// Kwadrat o boku 0.2 HU
size := common.HS(0.2)     // → 216 pikseli
```

### 6.3. Rysowanie obrazów

```go
// Obrazek o rozmiarach (w, h) HU na pozycji (x, y) HU
px, py := common.H(x, y)
pw := common.HS(w)
ph := common.HS(h)
common.DrawImage(screen, img, px-pw/2, py-ph/2, pw, ph, 1.0)
```

Funkcja `common.DrawImage` rysuje obraz przeskalowany do podanych wymiarów.
Punkt (x, y) to środek obrazka (bo odejmujemy połowę wymiarów).

---

## 7. Format wyników (results.json)

### 7.1. Wymagane pola (w każdym teście)

```go
type results struct {
    TestID    string `json:"testId"`
    SubjectID string `json:"subjectId"`
    Timestamp string `json:"timestamp"`
    Score     string `json:"score"`
    // ... pola specyficzne
}
```

| Pole JSON | Typ | Źródło | Opis |
|-----------|-----|--------|------|
| `testId` | string | stała | Identyfikator testu, np. `"Stop"`, `"Corsi"` |
| `subjectId` | string | `common.RandomID()` | ID sesji testowej (czas + 4 znaki hex) |
| `timestamp` | string | `common.Timestamp()` | ISO 8601 UTC, np. `"2026-07-18T12:34:56.789Z"` |
| `score` | string | obliczane | Podsumowanie wyników do wyświetlenia |

### 7.2. Zapis wyników

```go
func (g *game) writeResults() {
    data := g.buildResults()
    if err := common.WriteResults(".", data); err != nil {
        log.Printf("Błąd zapisu wyników: %v", err)
    }
}
```

- `common.WriteResults(dir, data)` — zapisuje JSON-a z wcięciami do `dir/results.json`.
- Zawsze używać `"."` jako katalogu (CWD = folder testu ustawiony przez launcher).
- Zawsze logować błąd (`log.Printf`), nigdy nie pomijać ani nie panicować.

### 7.3. Pola specyficzne (przykłady)

**Wspólne dla większości testów:**

| Pole JSON | Typ |
|-----------|-----|
| `ilosc_poprawnych_nacisniec` | int |
| `ilosc_blednych_nacisniec` | int |
| `ogolna_ilosc_nacisniec` | int |
| `sredni_czas_reakcji` | float64/int |
| `poziom_trudnosci` | string (jeśli dotyczy) |
| `wyniki` | `[]trialResult` (tablica prób) |
| `statystyki` | object (dodatkowe metryki) |

**Próby szczegółowe (`trialResult`):** zawierają dane per-trial, np.:

```go
type trialResult struct {
    Trial     int     `json:"trial"`
    Condition string  `json:"condition,omitempty"`
    Pressed   bool    `json:"pressed"`
    Correct   bool    `json:"was_correct"`
    RT        float64 `json:"rt"`
}
```

### 7.4. Integracja z launcher

Launcher wykonuje:
1. Ustawia `cwd: testFolder` (katalog testu, np. `tests_library/stop/`).
2. Spawnuje binarkę.
3. Po zakończeniu procesu odczytuje `testFolder/results.json`.
4. Opatentowuje wynik flagą `__native_binary: true`.
5. Wysyła do renderera przez IPC `test-results-forwarded`.

Renderer w `results.js:handleTestResults()` opakowuje otrzymany obiekt:

```js
currentResultPackage = {
    test_id: raw.testId || "test",
    timestamp: new Date().toISOString(),
    hpm_used: !!raw.__hpm_context,
    researcher_uid: getResearcherUid(),
    subject_id: participantId,
    demographics: getActiveDemographics(),
    wyniki: raw,               // ← cały surowy obiekt wyników
    isTraining: getTrainingMode(),
    sync_status: "...",
};
```

**Ważne:** Wszystkie pola z `results.json` trafiają do `wyniki` w bazie danych.
Należy zachować pełną strukturę — nie usuwać, nie zmieniać nazw pól używanych
w zapytaniach.

---

## 8. Zmienne środowiskowe

Launcher ustawia przed uruchomieniem testu:

| Zmienna | Wartość | Znaczenie |
|---------|---------|-----------|
| `NOUS_LAUNCHER` | `"1"` | Test wie, że został uruchomiony przez launcher |
| `NOUS_TRAINING` | `"1"` (opcjonalnie) | Tryb treningowy (wyniki nie są zapisywane) |

Test może je odczytać przez `common.GetEnv(key, fallback)`:

```go
if common.GetEnv("NOUS_TRAINING", "") == "1" {
    // tryb treningowy
}
```

---

## 9. Czcionki i tekst

### 9.1. Inicjalizacja

```go
if err := common.InitFonts(); err != nil {
    log.Fatal(err)
}
```

Wywołana w `main()` przed `ebiten.RunGame()`.

### 9.2. Rysowanie tekstu

```go
// Wyśrodkowany (wieloliniowy)
common.DrawTextCentered(screen, "Tekst\nw dwóch liniach",
    cx, cy, common.FontMedium, common.White)

// Lewy (pionowo wyśrodkowany)
common.DrawTextLeft(screen, "Tekst", x, cy, common.FontSmall, common.Black)
```

- `DrawTextCentered` — centruje każdą linię względem `cx`, całość pionowo względem `cy`.
- `DrawTextLeft` — lewa krawędź w `x`, pionowo centrowana względem `cy`.
- Używa `text.BoundString` i dzieli przez 64.0 (konwersja fixed-point Go → piksele).
- Poziome centrowanie: `(Min.X + Max.X) / 2`, NIGDY `Dx() / 2`.

### 9.3. Kolory

```go
common.White, common.Black, common.Gray, common.DarkGray, common.LightGray
common.Red, common.Green, common.Blue, common.Yellow, common.Orange
```

---

## 10. CI/CD — GitHub Actions

### 10.1. Platformy docelowe

| Platforma | GOOS | GOARCH | Budowane na | Uwagi |
|-----------|------|--------|-------------|-------|
| Windows amd64 | windows | amd64 | windows-latest | `CGO_ENABLED=0` |
| Linux amd64 | linux | amd64 | ubuntu-latest | Wymaga `libglfw3-dev` |
| Linux arm64 | linux | arm64 | Docker + QEMU | `arm64v8/golang:1.26-bookworm` |
| macOS amd64 | darwin | amd64 | macos-13 | Intel |
| macOS arm64 | darwin | arm64 | macos-latest | Apple Silicon |

### 10.2. Przebieg

1. `actions/checkout@v4`
2. `actions/setup-go@v5` z `go-version: '1.26'`
3. Instalacja zależności systemowych (Linux: GLFW, Mesa, X11)
4. Dla linux/arm64: Docker + QEMU
5. Budowa wszystkich 13 testów
6. Pakowanie z `resources/` i `meta.json`
7. Upload jako artifact: `nous-tests-<platform>`

### 10.3. Lista testów budowanych

```
bystreOczko corsi fabrykaFigur goNoGo goNoGoBiegacze nwstecz
pingPong samochodzik semafor stop stroop sygnalizacja zlapSygnal
```

Nowy test trzeba dodać do listy w workflow (`build.yml`), w zmiennej `TESTS`.

---

## 11. Rysowanie

### 11.1. Funkcje `common/ui.go`

| Funkcja | Opis |
|---------|------|
| `DrawRect(screen, x, y, w, h, clr)` | Wypełniony prostokąt |
| `DrawRectOutline(screen, x, y, w, h, clr, lineWidth)` | Obrys prostokąta |
| `DrawImage(screen, img, x, y, w, h, alpha)` | Obraz skalowany (x,y = lewy górny róg) |
| `DrawImageRotated(screen, img, x, y, w, h, angle, alpha)` | Obraz skalowany + obrót |
| `DrawCircleFilled(screen, cx, cy, r, clr)` | Wypełnione koło (szorstkie przy krawędziach) |
| `PointInRect(px, py, rx, ry, rw, rh) bool` | Kolizja punkt-prostokąt |
| `PointInCircle(px, py, cx, cy, r) bool` | Kolizja punkt-koło |

### 11.2. Uwagi

- `DrawCircleFilled` rysuje koło jako kwadrat (ograniczenie Ebitengine). Dla gładkich
  okręgów używać `ebitenutil.DrawCircle` (wymaga importu `github.com/hajimehoshi/ebiten/v2/ebitenutil`).
- Wszystkie funkcje rysowania przyjmują współrzędne w PIKSLACH (nie HU).
- `DrawImage` rysuje od (x, y) jako lewego górnego rogu. Aby wyśrodkować obrazek
  w punkcie (cx, cy), użyj: `DrawImage(screen, img, cx-w/2, cy-h/2, w, h, alpha)`.

---

## 12. Lista kontrolna (checklista)

Przed dodaniem nowego testu należy sprawdzić:

### Struktura i budowa
- [ ] Katalog `cmd/<test>/main.go` z `package main`
- [ ] Funkcja `main()` wywołuje `common.InitFonts()`
- [ ] `ebiten.SetFullscreen(true)` i `SetWindowSize(ScreenW, ScreenH)`
- [ ] `Layout()` zwraca `common.ScreenW, common.ScreenH`
- [ ] Test kompiluje się bez błędów: `go build ./cmd/<test>/`

### Obsługa klawiszy
- [ ] `startupTicks` — pomijanie pierwszych 30 klatek
- [ ] Escape — zapis wyników i `return ebiten.Termination`
- [ ] NIGDY `os.Exit()`, NIGDY `log.Fatal()` w `Update()`

### Zasoby
- [ ] Jeśli test używa obrazów: katalog `resources/`, `//go:embed resources/*`
- [ ] Obrazy ładowane przez `common.LoadImageFS(resources, "nazwa.png")`
- [ ] Brak `os.Open`, `filepath.Join`, `ebitenutil.NewImageFromFile`

### Wyniki
- [ ] `writeResults()` zapisuje do `"."` (bieżący katalog)
- [ ] Obiekt wyników zawiera `testId`, `subjectId`, `timestamp`, `score`
- [ ] Błąd zapisu logowany: `log.Printf`
- [ ] Struktura JSON zgodna z oczekiwaniami renderera (wszystkie pola w `wyniki`)

### Kod
- [ ] Konsekwentna nazwa struktury (`game` lub `Game`) w całym pliku
- [ ] Czytelna maszyna stanów (string lub iota)
- [ ] Używane `common.H()` i `common.HS()` dla współrzędnych PsychoPy
- [ ] Używane `common.FontBig/Medium/Small/Tiny` zamiast własnych czcionek
- [ ] Używane `common.DrawTextCentered/Left` zamiast ręcznego `text.Draw`

### CI/CD
- [ ] Test dodany do listy `TESTS` w `.github/workflows/build.yml`
- [ ] Jeśli test ma `resources/` — workflow automatycznie je spakuje

### Zgodność z launcher
- [ ] Binarka uruchamia się z `cwd = folder testu`
- [ ] `results.json` jest tworzony w CWD
- [ ] Test poprawnie działa z `NOUS_LAUNCHER=1`

---

## 13. Wzorce implementacyjne

### 13.1. Stan gry jako string (prostszy)

```go
type game struct {
    state        string
    startupTicks int
    subjectId    string
    timestamp    string
    // ...
}

const (
    stateInstruction = "instruction"
    stateTrial       = "trial"
    statePause       = "pause"
    stateDone        = "done"
)
```

Stosowany w: `corsi`, `goNoGo`, `pingPong`, `samochodzik`, `stop`, `sygnalizacja`,
`zlapSygnal`.

### 13.2. Stan gry jako iota (bardziej typowane)

```go
type state int

const (
    stateMenu state = iota
    stateInstruction
    stateTrial
    stateDone
)

type game struct {
    state state
    // ...
}
```

Stosowany w: `bystreOczko`, `fabrykaFigur`, `goNoGoBiegacze`, `nwstecz`, `semafor`,
`stroop`.

### 13.3. Escape z opóźnieniem (gdy test ignoruje klawisze w trakcie trial)

```go
if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
    g.escaped = true
}

// W odpowiednim momencie:
if g.escaped {
    g.writeResults()
    return ebiten.Termination
}
```

Stosowany w: `semafor`, `zlapSygnal` (tam gdzie Escape ma być obsłużony dopiero
po zakończeniu bieżącej próby).

### 13.4. Generowanie unikalnego ID sesji

```go
subjectId: common.RandomID()
// np. "20260718123456-abcd"

timestamp: common.Timestamp()
// np. "2026-07-18T12:34:56.789Z"
```

---

## 14. Najczęstsze błędy

| Błąd | Skutek | Rozwiązanie |
|------|--------|-------------|
| `os.Exit(0)` zamiast `return ebiten.Termination` | Launcher nie odczyta wyników | Użyj `return ebiten.Termination` |
| Brak `startupTicks` | Rejestracja niechcianego klawisza na starcie | Dodaj `startupTicks` i warunek `< 30` |
| `results.json` zapisany w złym katalogu | Launcher nie znajdzie pliku | Użyj `common.WriteResults(".", data)` |
| `text.Draw` z `Dx()/2` zamiast `(Min.X+Max.X)/2` | Nieprawidłowe centrowanie | Użyj `common.DrawTextCentered` |
| Brak `common.InitFonts()` | Panic przy rysowaniu tekstu | Wywołaj w `main()` |
| Pominięcie `_ "image/png"` w imporcie | Nie uda się dekodować PNG | Dodaj import `_ "image/png"` |
| Użycie `ebitenutil.NewImageFromFile` | Nie działa z embedded FS | Użyj `common.LoadImageFS` |
| Brak obsługi błędu `WriteResults` | Cichy brak zapisu | Zawsze `log.Printf` błąd |
