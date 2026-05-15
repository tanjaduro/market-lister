# Claude Code prompt for market-lister (v4)

Source build prompt, preserved with one edit: the system-prompt body in §"prompt.txt" is extracted to a sibling `prompt.txt` file (the runtime artifact) rather than duplicated here, to keep a single source of truth. All other content is verbatim from the original v4 paste. PLAN.md derives all design decisions from this document. If this file and PLAN.md disagree, PLAN.md wins for execution — but flag the discrepancy.

---

After:

```bash
mkdir ~/market-lister && cd ~/market-lister
git init
go mod init github.com/tanjaduro/market-lister
claude
```

Paste everything below the line into Claude Code.

---

Build a Go CLI tool: product photos → Gemini Vision API → marketplace listing markdown for Vinted DE and Kleinanzeigen.

The tool is generic across categories (children's clothing, adult clothing, shoes, books, 3D-printed parts, household items, electronics). The LLM determines the category from the photos and populates a flexible attribute map.

## Capability contract

If you cannot read files or execute bash/go commands, output `BLOCKED` and halt.

Before writing any code that touches the Gemini SDK, verify the exact API surface from current official docs: function names, struct names, import paths, the structured-output config field names, and the inline-image content-part constructor. The Gemini Go SDK was unified in 2025 into `google.golang.org/genai` (replacing the older `github.com/google/generative-ai-go/genai`). Use the unified package. Do NOT guess from training data — quote the relevant doc snippet in your audit summary before generating SDK code.

Output an audit summary to stdout before writing files: list which files you will create, in which order, and quote the Gemini SDK functions and types you plan to use.

After all files are created, run `go mod tidy && go vet ./... && go build .` and report the output verbatim. Do not run the binary — the user will do that with their API key.

## What the tool does

Input: a directory of subfolders. Each subfolder represents one item for sale. Each contains 3–15 phone photos named like `20260513_101843.jpg`. The subfolder name is a human-readable hint:

- `reima pink 24` → kids shoes
- `book stephen king it` → book
- `3d-printed bracket black petg` → 3D part
- `zara dress floral 104` → kids clothing
- `no brand glass vase` → household

Output: one `{slug}.md` file per item. By default written INSIDE the source subfolder next to the photos. If `OUTPUT_DIR` is set (env or `-output` flag), written there instead, and photo paths in the markdown are adjusted to point back to the source folder.

Slug rule: lowercase, ASCII only, spaces and underscores → hyphens, drop anything outside `[a-z0-9-]`, collapse multiple hyphens. Example: `H&M jacket  blue 104` → `hm-jacket-blue-104`.

Idempotency: if `{slug}.md` already exists at the target path, skip the item with a log message.

## Architecture

Flat `package main`. No cmd/, no internal/, no interfaces, no DI.

Files in build order:

1. `item.go` — `Item` struct.
2. `config.go` — `Config` struct, env loading via `github.com/joho/godotenv`.
3. `prompt.txt` — Gemini system prompt (verbatim content provided below).
4. `vision.go` — `Describe(ctx, folderPath, hint string, cfg Config) (Item, []string, error)`. Returns item + photo basenames.
5. `markdown.go` — `Render(item Item, slug, folderName string, photos []string, photoBase string, today time.Time) string`. Pure. `photoBase` is prepended to each photo filename in the markdown (empty string when `.md` is co-located with photos).
6. `markdown_test.go` — one table-driven test for Render covering: clothing item with attributes, item with no flaws, item with multiple flaws.
7. `main.go` — CLI flags, directory scan, orchestration, file writes.
8. `.env.example`, `.gitignore`, `LICENSE` (MIT, copyright Tatjana Duro 2026), `README.md`.

## Dependencies (only these)

```
google.golang.org/genai
github.com/joho/godotenv
```

No image preprocessing. Gemini 2.5 Flash accepts inline images up to 20MB — phone photos fit.

## Item struct

```go
// Item is the structured listing draft returned by the vision model.
// Category-specific fields go in Attributes so the same struct
// handles clothing, shoes, books, 3D-printed parts, household, electronics.
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
```

Allowed `Category` values: `clothing`, `shoes`, `books`, `3d-printed`, `household`, `electronics`, `other`.

Allowed `Condition` values: `new-with-tags`, `used-excellent`, `used-good`, `used-fair`.

Titles capped at 70 characters (Vinted's limit is the strictest, use it for both).

## Vision flow

1. List files matching `*.jpg`, `*.jpeg`, `*.png` in the folder, sorted by filename.
2. Read bytes for each. Detect MIME type from magic bytes, not extension:
   - `0xFF 0xD8 0xFF` → `image/jpeg`
   - `0x89 0x50 0x4E 0x47` → `image/png`
   - Otherwise: log warning, exclude that file.
3. Build a `GenerateContent` request to model `gemini-2.5-flash`:
   - System instruction: contents of `prompt.txt`
   - Content parts (single user message): text part `Folder name hint: {hint}` followed by all images as inline data parts.
   - Generation config: `ResponseMIMEType: "application/json"` and `ResponseSchema` matching the Item struct. Verify the exact field/type names against current SDK docs.
4. Send the request with `context.Context` from main (timeout configurable, default 120s per item).
5. Parse response text as JSON into Item. If parse fails, log raw response truncated to 500 chars and return error.
6. Validate: enforce title length ≤70, category in allowed set, condition in allowed set, price ≥0. On validation failure, log and return error.
7. Return Item + slice of photo basenames.

## prompt.txt

See the `prompt.txt` file in the repository (verbatim source).

## markdown.go template (use Go text/template, this exact structure)

The template uses Go `text/template` syntax. Define it as a const and parse once at package init.

```go
const tmpl = `---
id: "{{.Date}}-{{.Slug}}"
status: draft
platforms: [vinted, kleinanzeigen]
category: {{.Item.Category}}
condition: {{.Item.Condition}}
price_estimate_eur: {{.Item.PriceEstimateEUR}}
source_folder: "{{.FolderName}}"
date_added: {{.Date}}
attributes:
{{- range $k, $v := .Item.Attributes }}
  {{$k}}: "{{$v}}"
{{- end }}
flaws:
{{- if .Item.Flaws }}
{{- range .Item.Flaws }}
  - "{{.}}"
{{- end }}
{{- else }} []
{{- end }}
---

# {{.Item.TitleEN}}

## Vinted (English)

{{.Item.DescriptionVintedEN}}

## Vinted (Deutsch)

{{.Item.DescriptionVintedDE}}

## Kleinanzeigen (Deutsch)

{{.Item.DescriptionKleinanzeigenDE}}

## Details

- Category: {{.Item.Category}}
- Condition: {{.Item.Condition}}
- Price estimate: {{.Item.PriceEstimateEUR}} EUR
{{- range $k, $v := .Item.Attributes }}
- {{$k}}: {{$v}}
{{- end }}

## Flaws

{{- if .Item.Flaws }}
{{- range .Item.Flaws }}
- {{.}}
{{- end }}
{{- else }}
None visible.
{{- end }}

## Checklist

- [ ] Verify key details on labels / packaging
- [ ] Set final price
- [ ] Choose pickup Panketal or shipping
{{- if .Item.OCRNotes }}

## OCR notes (verify these)

{{.Item.OCRNotes}}
{{- end }}

## Photos

{{- range .Photos }}
![](<{{$.PhotoBase}}{{.}}>)
{{- end }}
`
```

**Amendment (post-paste):** Image links use angle-bracket syntax (`![](<path>)`) to tolerate spaces in folder paths like `reima pink 24`. Bare `![](path)` breaks in most Markdown renderers when paths contain spaces.

Template data struct:

```go
type tmplData struct {
    Date       string // YYYY-MM-DD
    Slug       string
    FolderName string
    PhotoBase  string // "" if .md is in same dir as photos; otherwise relative or absolute path with trailing /
    Photos     []string
    Item       Item
}
```

## Photo path handling

- If `OUTPUT_DIR` is empty (default): `.md` is written into the source folder. `PhotoBase` is empty. Markdown contains `![](20260513_101843.jpg)`.
- If `OUTPUT_DIR` is set: `.md` is written there. `PhotoBase` is the absolute path of the source folder with a trailing slash, e.g. `/mnt/d/Obsidian/.../reima pink 24/`. Markdown contains `![](/mnt/d/.../20260513_101843.jpg)`. Use absolute paths — relative paths break when folders move.

## CLI

```
go run .                            # process all subfolders in INPUT_DIR
go run . -folder "reima pink 24"    # process one specific folder
go run . -output /path/to/dir       # write .md elsewhere instead of source folder
go run . -dry-run                   # list folders that would be processed, no API calls
```

Use `log/slog` (stdlib structured logging) for progress, JSON or text handler — pick text for terminal readability:

```go
slog.Info("processing", "folder", folderName, "photos", len(photos))
slog.Info("done", "folder", folderName, "duration_ms", elapsed.Milliseconds())
slog.Warn("skipping, already processed", "folder", folderName, "path", outputPath)
slog.Error("vision failed", "folder", folderName, "error", err)
```

## Config (env vars and .env)

```
GEMINI_API_KEY=                                                # required, from https://aistudio.google.com/apikey
INPUT_DIR=/mnt/d/Obsidian/market-vault/_inbox/RAW              # default
OUTPUT_DIR=                                                    # empty = write next to photos
GEMINI_MODEL=gemini-2.5-flash                                  # override for testing newer models
REQUEST_TIMEOUT_SECONDS=120
```

Use `godotenv.Load()` (not `Overload`) — env vars set in the shell win over .env. If `GEMINI_API_KEY` is missing, exit immediately with: `GEMINI_API_KEY not set. Get a free key at https://aistudio.google.com/apikey`.

## Error policy

- Invalid JSON from Gemini → log warning with raw snippet, skip, continue.
- Folder with no usable images → log, skip.
- `.md` already exists at target → log skip, continue (idempotent re-runs).
- One image fails MIME detection or read → exclude that image only, proceed with the rest. If zero remain, skip the item.
- API error → log, skip item, continue. No retries in v1.
- Filesystem write failure → log, exit non-zero. Something is fundamentally wrong.
- Validation failure (title too long, invalid category) → log raw Item, skip item, continue.
- One failed item NEVER crashes the full batch.

## Tests

`markdown_test.go` is the only test file. Table-driven, three cases:

```go
func TestRender(t *testing.T) {
    cases := []struct{
        name string
        item Item
        photos []string
        wantContains []string
        wantNotContains []string
    }{
        {name: "clothing with attributes and flaws", ...},
        {name: "item with no flaws", ...},
        {name: "item with OCR notes", ...},
    }
    // Assert generated markdown contains/doesn't contain expected strings.
}
```

Verify: YAML frontmatter is valid, all attribute keys appear, flaws section renders correctly empty and non-empty, photos use the PhotoBase prefix correctly.

## Code style

- Go 1.24+. All identifiers, comments, doc comments, README in English.
- Explicit error handling. No panic except in `main` for unrecoverable startup.
- No interfaces, no factories, no DI.
- `gofmt` clean, `go vet` clean.
- Doc comment on every exported identifier explaining intent, not mechanics.
- One exported type per file where reasonable.

## .gitignore

```
.env
market-lister
*.exe
*.log
.vscode/
.idea/
```

## LICENSE (MIT)

Standard MIT text with `Copyright (c) 2026 Tatjana Duro`.

## README structure (≤150 lines, English, no emoji)

1. Title: `market-lister`
2. One-line tagline: `Generate marketplace listing drafts from product photos using Gemini Vision.`
3. Badges: Go version, license (MIT).
4. "What it does" — 4 bullets.
5. "How it works" — ASCII pipeline:
   ```
   photo folders → Gemini 2.5 Flash → JSON → markdown
   ```
6. "Setup" — Go install, clone, `cp .env.example .env`, free key from AI Studio.
7. "Usage" — four CLI commands with one example output line each.
8. "Output format" — paste a real example of generated markdown (one short item).
9. "Supported categories" — bullet list.
10. "Design notes" — 5 short paragraphs:
    - Go with `google.golang.org/genai` for type-safe vision API integration.
    - Gemini 2.5 Flash chosen for free tier (1500 requests/day) and 20MB inline image support, eliminating preprocessing.
    - Single universal `Item` struct with category-flexible `Attributes` map instead of per-category types — keeps the codebase flat and the vision prompt simple.
    - Plain markdown output (no Obsidian-specific syntax) so files work in any viewer.
    - `.md` defaults to writing next to source photos; opt-in `OUTPUT_DIR` separation for cleaner archives.
11. "Roadmap" — two bullets:
    - Prompt evaluation suite (separate `evals/` directory, promptfoo).
    - Optional mdbook catalog renderer (separate repo).

Portfolio code. Clean, scannable, professional. No marketing fluff.

## Concrete example (use for your test fixtures and README)

Folder name: `reima pink 24`
Photos: 8 phone shots showing pink kids sneakers, full views from multiple angles, sole, insole label reading `reima 24 US 8 CM 16.00`, defect close-up of scuffed white midsole.

Expected Item:

```json
{
  "title_en": "Reima pink kids sneakers, EU 24 / US 8 (16 cm)",
  "title_de": "Reima rosa Kinder-Sneaker, Gr. 24 / US 8 (16 cm)",
  "category": "shoes",
  "condition": "used-good",
  "flaws": [
    "white midsole scuffed and yellowed at toe area",
    "moderate outsole tread wear"
  ],
  "description_vinted_en": "Reima kids sneakers in pink, size EU 24 (US 8 / 16 cm insole). Velcro closure, mesh upper, reflective heel pull tab. Good used condition — white midsole is scuffed at the toes and slightly yellowed. From a smoke-free home.",
  "description_vinted_de": "Reima Kinder-Sneaker in Rosa, Größe EU 24 (US 8 / 16 cm Innensohle). Klettverschluss, Mesh-Obermaterial, reflektierende Lasche an der Ferse. Guter gebrauchter Zustand — weiße Sohle an den Zehen abgerieben und leicht vergilbt. Aus tierfreiem Nichtraucherhaushalt.",
  "description_kleinanzeigen_de": "Verkaufe Reima Kinder-Sneaker in Rosa, Größe 24 (US 8 / 16 cm Innensohle). Klettverschluss und atmungsaktives Mesh-Obermaterial. Reflektierende Lasche an der Ferse für Sichtbarkeit. Guter gebrauchter Zustand — weiße Sohle an den Zehen abgerieben und leicht vergilbt, Profil moderat abgenutzt. Aus tierfreiem Nichtraucherhaushalt. Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme.",
  "price_estimate_eur": 10,
  "attributes": {
    "brand": "Reima",
    "size_eu": "24",
    "size_us": "8",
    "size_cm": "16.00",
    "material": "mesh upper, rubber outsole",
    "color": "pink"
  },
  "ocr_notes": "Insole text reads: reima 24 / US 8 / CM 16.00 / CN170(1.5)"
}
```

This is what `vision.go` returns. `markdown.go` then renders it through the template above. Use this exact example for one of the test cases in `markdown_test.go`.

## Build verification (run after all files created)

```
go mod tidy
go vet ./...
go build .
go test ./...
```

Report each command's output verbatim. Fix any issues and re-run until clean.
