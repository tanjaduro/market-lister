# market-lister — Build Plan (v4)

Source spec: an earlier `prompt-v4.md` draft (since removed; its content was distilled into `prompt.txt`). This plan freezes the design and skeleton before any code is written. Edit this file to change decisions; the build will read from here.

---

## Decisions taken

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Keep existing `LICENSE` as `Copyright (c) 2026 Tanja Duro` | Intended name per user. Spec's "Tatjana" is a typo in the spec. |
| 2 | `slugify` drops non-ASCII letters (no transliteration) | Matches spec literally: "drop anything outside `[a-z0-9-]`" |
| 3 | `prompt.txt` embedded via `//go:embed` | Self-contained binary, no runtime path lookup |
| 4 | Template parsed once via `template.Must` at package load | Spec: "parse once at package init" |
| 5 | `validateItem` uses rune count for 70-char title cap | UTF-8 safe; ä/ö/ü/ß count as 1 |
| 6 | Superseded — retry with exponential backoff added for transient errors (503, 429, UNAVAILABLE, RESOURCE_EXHAUSTED). | Per error policy |
| 7 | Gemini SDK call sites are **not drafted** in this plan — to be written only after `go doc google.golang.org/genai` at build time | Spec capability contract: "Do NOT guess from training data" |
| 8 | `markdown_test.go` includes the spec's Reima example verbatim as case #1 | Spec explicitly mandates this |
| 9 | Folders whose slug sanitises to `""` (all non-ASCII) are skipped with a warning | Prevents writing a bare `.md` file with no name; logged so the user can rename |
| 10 | Per-item failures (incl. write failure) are logged + counted, never `os.Exit`. `os.Exit(1)` is reserved for startup errors (missing API key, INPUT_DIR doesn't exist, OUTPUT_DIR can't be created) | Spec error policy: "one failed item NEVER crashes the full batch". Losing 30 processed items because item 31 failed to write is unacceptable. |
| 11 | `OUTPUT_DIR` is created with `os.MkdirAll` at startup if it doesn't exist; abort with clear error if mkdir fails | Avoid per-item `os.WriteFile` failures when the user supplies a not-yet-existing output dir. Fail fast at startup, not 30 items in. |
| 12 | Markdown image links use angle-bracket syntax `![](<path>)` instead of bare `![](path)` | Folder names contain spaces (e.g. "reima pink 24"); bare paths break in Markdown renderers. Angle brackets are simpler than URL-encoding and stay human-readable. |
| 13 | `processFolder` computes `today := time.Now()` once and uses it for both elapsed-time logging and `Render()`'s date | Two `time.Now()` calls in the same function is sloppy and can produce inconsistent timestamps across midnight. |
| 14 | Minimum Go version is **1.25** (was 1.24, originally 1.22 in the spec) | Bumped from 1.24 to 1.25 to clear GO-2026-4918 (HTTP/2 infinite-loop CVE) reachable via `genai`. The fix lives in `golang.org/x/net@v0.53.0`, which requires Go 1.25. `go.mod` declares `go 1.25.0` with `toolchain go1.25.10`; README badge and Setup reflect this. `genai` v1.57+ still works (originally needed 1.24); do not downgrade below 1.25 without finding a different x/net fix. |

---

## File tree (build order)

```
market-lister/
├── go.mod                  step 0   go mod init github.com/tanjaduro/market-lister
├── go.sum                  step 0   go mod tidy (after sources written)
├── item.go                 step 1   Item struct + valid{Categories,Conditions}
├── config.go               step 2   Config + LoadConfig
├── prompt.txt              step 3   verbatim system prompt
├── vision.go               step 4   Describe() — AFTER SDK doc audit
├── markdown.go             step 5   Render() + template
├── markdown_test.go        step 6   TestRender (table-driven, 3 cases)
├── main.go                 step 7   flags + orchestration + slugify
├── .env.example            step 8
├── .gitignore              step 8
├── README.md               step 8
└── LICENSE                 (exists, untouched)
```

---

## SDK audit (executed at step 4, before any `vision.go` code)

```
go doc google.golang.org/genai
go doc google.golang.org/genai.Client
go doc google.golang.org/genai.GenerateContentConfig
go doc google.golang.org/genai.Part
```

Findings to quote into the build log before writing code:
- Client constructor exact signature
- **Inline-image part constructor exact name** — likely `genai.NewPartFromBytes(data []byte, mimeType string)` or similar. NOT a `Blob` literal; the convenience constructor is the API entry point.
- `ResponseMIMEType` field name and type
- **`ResponseSchema` field type — MUST be `*genai.Schema` (typed struct), NOT a JSON string or `map[string]any`.** This is a common SDK mistake. The struct uses `genai.TypeObject` / `genai.TypeString` / `genai.TypeArray` constants for the `Type` field. Build the Item schema from these.
- How `GenerateContent` is invoked (`client.Models.GenerateContent(...)` vs other)
- How system instruction is passed (likely `GenerateContentConfig.SystemInstruction` accepting `*genai.Content` or `[]*genai.Part`)

No `vision.go` code is written until those six answers are quoted in the build log.

---

## Skeleton

### `item.go` — complete

```go
package main

// Item is the structured listing draft returned by the vision model.
// Category-specific fields go in Attributes so the same struct
// handles clothing, shoes, books, 3D-printed parts, household, and electronics.
type Item struct {
	TitleEN                    string            `json:"title_en"`
	TitleDE                    string            `json:"title_de"`
	Category                   string            `json:"category"`
	Condition                  string            `json:"condition"`
	Flaws                      []string          `json:"flaws"`
	DescriptionVintedEN        string            `json:"description_vinted_en"`
	DescriptionVintedDE        string            `json:"description_vinted_de"`
	DescriptionKleinanzeigenDE string            `json:"description_kleinanzeigen_de"`
	PriceEstimateEUR           int               `json:"price_estimate_eur"`
	Attributes                 map[string]string `json:"attributes"`
	OCRNotes                   string            `json:"ocr_notes"`
}

var validCategories = map[string]bool{
	"clothing": true, "shoes": true, "books": true,
	"3d-printed": true, "household": true, "electronics": true, "other": true,
}

var validConditions = map[string]bool{
	"new-with-tags": true, "used-excellent": true,
	"used-good": true, "used-fair": true,
}
```

### `config.go` — complete

```go
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration loaded from environment and .env.
type Config struct {
	GeminiAPIKey          string
	InputDir              string
	OutputDir             string
	GeminiModel           string
	RequestTimeoutSeconds int
}

// LoadConfig loads .env if present (shell env wins), then validates required fields.
func LoadConfig() (Config, error) {
	_ = godotenv.Load()

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return Config{}, fmt.Errorf(
			"GEMINI_API_KEY not set. Get a free key at https://aistudio.google.com/apikey")
	}

	inputDir := os.Getenv("INPUT_DIR")
	if inputDir == "" {
		inputDir = "/mnt/d/Obsidian/market-vault/_inbox/RAW"
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}
	timeout := 120
	if s := os.Getenv("REQUEST_TIMEOUT_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			timeout = n
		}
	}

	return Config{
		GeminiAPIKey:          key,
		InputDir:              inputDir,
		OutputDir:             os.Getenv("OUTPUT_DIR"),
		GeminiModel:           model,
		RequestTimeoutSeconds: timeout,
	}, nil
}
```

### `prompt.txt`

Verbatim from spec §"prompt.txt". No changes.

### `vision.go` — skeleton; SDK calls deferred to build time

```go
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	// SDK import — exact symbols confirmed from `go doc` at build time
	"google.golang.org/genai"
)

//go:embed prompt.txt
var systemPrompt string

// Describe sends all readable images in folderPath to Gemini Vision and returns
// a validated Item plus the basenames of photos that were sent.
func Describe(ctx context.Context, folderPath, hint string, cfg Config) (Item, []string, error) {
	// 1. glob *.jpg, *.jpeg, *.png; sort by filename
	// 2. read bytes; detectMIME; drop unknowns with slog.Warn
	//    if zero survive → error "no usable images"
	// 3. build client (exact ctor TBD from SDK audit)
	// 4. assemble parts: text "Folder name hint: {hint}" + inline image parts
	// 5. call GenerateContent with ResponseMIMEType=application/json + ResponseSchema for Item
	// 6. json.Unmarshal response → Item; on fail log raw[:500]
	// 7. validateItem; on fail log raw Item
	// 8. return Item, basenames, nil
	panic("TODO")
}

// detectMIME returns ("image/jpeg" | "image/png", true) from magic bytes, else ("", false).
func detectMIME(data []byte) (string, bool) {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", true
	}
	if len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png", true
	}
	return "", false
}

// validateItem enforces title length, category/condition closed sets, and non-negative price.
func validateItem(item Item) error {
	if len([]rune(item.TitleEN)) > 70 {
		return fmt.Errorf("title_en exceeds 70 chars")
	}
	if len([]rune(item.TitleDE)) > 70 {
		return fmt.Errorf("title_de exceeds 70 chars")
	}
	if !validCategories[item.Category] {
		return fmt.Errorf("invalid category %q", item.Category)
	}
	if !validConditions[item.Condition] {
		return fmt.Errorf("invalid condition %q", item.Condition)
	}
	if item.PriceEstimateEUR < 0 {
		return fmt.Errorf("price_estimate_eur is negative")
	}
	return nil
}
```

### `markdown.go` — complete except for `tmplRaw` body (verbatim from spec)

```go
package main

import (
	"bytes"
	"text/template"
	"time"
)

type tmplData struct {
	Date       string
	Slug       string
	FolderName string
	PhotoBase  string
	Photos     []string
	Item       Item
}

var listingTmpl = template.Must(template.New("listing").Parse(tmplRaw))

const tmplRaw = `---
id: "{{.Date}}-{{.Slug}}"
... [verbatim from spec §"markdown.go template"]
`

// Render produces the complete markdown listing for one item. Pure function.
func Render(item Item, slug, folderName string, photos []string, photoBase string, today time.Time) string {
	var buf bytes.Buffer
	if err := listingTmpl.Execute(&buf, tmplData{
		Date:       today.Format("2006-01-02"),
		Slug:       slug,
		FolderName: folderName,
		PhotoBase:  photoBase,
		Photos:     photos,
		Item:       item,
	}); err != nil {
		panic(err) // template is a const; failure is a programmer error
	}
	return buf.String()
}
```

### `markdown_test.go` — three cases

```go
package main

import (
	"strings"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	fixedDate := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name            string
		item            Item
		slug, photoBase string
		photos          []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "reima shoes — spec example (attributes + flaws + OCR notes)",
			// item populated verbatim from spec JSON example
			// asserts: yaml id/category/condition/price, all 6 attribute keys,
			//         both flaw strings, ## OCR notes section, photo with empty photoBase,
			//         Kleinanzeigen closing line
		},
		{
			name: "item with no flaws (clothing)",
			// asserts: "None visible.", "flaws: []", no "## OCR notes" section
		},
		{
			name: "OUTPUT_DIR set — photoBase is absolute path with spaces, angle-bracket escaped",
			// asserts: image link is ![](</abs/path/reima pink 24/cover.jpg>)
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.item, tc.slug, "test-folder", tc.photos, tc.photoBase, fixedDate)
			for _, w := range tc.wantContains {
				if !strings.Contains(got, w) {
					t.Errorf("want %q in output\n--- got ---\n%s", w, got)
				}
			}
			for _, w := range tc.wantNotContains {
				if strings.Contains(got, w) {
					t.Errorf("want %q NOT in output\n--- got ---\n%s", w, got)
				}
			}
		})
	}
}
```

### `main.go` — skeleton

```go
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// result is the per-folder outcome tallied in main().
type result int

const (
	resultDone result = iota
	resultSkipped
	resultFailed
)

func main() {
	folderFlag := flag.String("folder", "", "process a single named subfolder")
	outputFlag := flag.String("output", "", "write .md files here instead of source folder")
	dryRun := flag.Bool("dry-run", false, "list folders without calling the API")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := LoadConfig()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if *outputFlag != "" {
		cfg.OutputDir = *outputFlag
	}

	// Startup validation: INPUT_DIR exists, OUTPUT_DIR is created if missing.
	if _, err := os.Stat(cfg.InputDir); err != nil {
		slog.Error("INPUT_DIR not accessible", "path", cfg.InputDir, "error", err)
		os.Exit(1)
	}
	if cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
			slog.Error("cannot create OUTPUT_DIR", "path", cfg.OutputDir, "error", err)
			os.Exit(1)
		}
	}

	folders, err := collectFolders(cfg.InputDir, *folderFlag)
	if err != nil {
		slog.Error("cannot scan input dir", "error", err)
		os.Exit(1)
	}

	var done, skipped, failed int
	for _, folder := range folders {
		if *dryRun {
			slog.Info("dry-run", "folder", filepath.Base(folder))
			continue
		}
		switch processFolder(cfg, folder) {
		case resultDone:
			done++
		case resultSkipped:
			skipped++
		case resultFailed:
			failed++
		}
	}

	if !*dryRun {
		slog.Info("summary", "done", done, "skipped", skipped, "failed", failed)
	}
}

// collectFolders returns absolute paths of item subfolders to process.
func collectFolders(inputDir, singleName string) ([]string, error) { panic("TODO") }

// processFolder runs one folder end-to-end and returns its outcome.
// Failures are logged and counted by main; never call os.Exit here.
func processFolder(cfg Config, folderPath string) result {
	hint := filepath.Base(folderPath)
	slug := slugify(hint)
	if slug == "" {
		slog.Warn("skipping, slug empty after sanitization", "folder", hint)
		return resultSkipped
	}
	outputPath := outputMDPath(cfg, folderPath, slug)

	if _, err := os.Stat(outputPath); err == nil {
		slog.Warn("skipping, already processed", "folder", hint, "path", outputPath)
		return resultSkipped
	}

	today := time.Now()
	slog.Info("processing", "folder", hint)

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.RequestTimeoutSeconds)*time.Second)
	defer cancel()

	item, photos, err := Describe(ctx, folderPath, hint, cfg)
	if err != nil {
		slog.Error("vision failed", "folder", hint, "error", err)
		return resultFailed
	}

	photoBase := ""
	if cfg.OutputDir != "" {
		photoBase = folderPath + string(filepath.Separator)
	}

	md := Render(item, slug, hint, photos, photoBase, today)

	if err := os.WriteFile(outputPath, []byte(md), 0o644); err != nil {
		slog.Error("write failed", "folder", hint, "path", outputPath, "error", err)
		return resultFailed
	}
	slog.Info("done", "folder", hint, "duration_ms", time.Since(today).Milliseconds())
	return resultDone
}

func outputMDPath(cfg Config, folderPath, slug string) string {
	if cfg.OutputDir != "" {
		return filepath.Join(cfg.OutputDir, slug+".md")
	}
	return filepath.Join(folderPath, slug+".md")
}

var (
	reNonAlnum  = regexp.MustCompile(`[^a-z0-9-]+`)
	reMultiDash = regexp.MustCompile(`-{2,}`)
)

// slugify lowercases, maps spaces/underscores to hyphens, drops non-ASCII-alnum,
// and collapses consecutive hyphens. Non-ASCII letters are dropped, not transliterated.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ' || r == '_':
			b.WriteRune('-')
		case r < unicode.MaxASCII:
			b.WriteRune(r)
		}
	}
	out := reNonAlnum.ReplaceAllString(b.String(), "")
	out = reMultiDash.ReplaceAllString(out, "-")
	return strings.Trim(out, "-")
}
```

### `.env.example`

```
GEMINI_API_KEY=
INPUT_DIR=/mnt/d/Obsidian/market-vault/_inbox/RAW
OUTPUT_DIR=
GEMINI_MODEL=gemini-2.5-flash
REQUEST_TIMEOUT_SECONDS=120
```

### `.gitignore`

```
.env
market-lister
*.exe
*.log
.vscode/
.idea/
```

### `README.md`

Follows the 11-section structure from spec §"README structure" (≤150 lines, English, no emoji).

---

## Build sequence (when explicitly approved)

```
step 0   go mod init github.com/tanjaduro/market-lister
         go get google.golang.org/genai@latest
         go get github.com/joho/godotenv@latest
step 1-3 item.go, config.go, prompt.txt
step 4   `go doc google.golang.org/genai*` → quote findings → write vision.go
step 5-7 markdown.go, markdown_test.go, main.go
step 8   .env.example, .gitignore, README.md
step 9   go mod tidy && go vet ./... && go build . && go test ./...
         (binary not executed — needs GEMINI_API_KEY)
```
