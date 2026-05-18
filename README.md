# market-lister

Generate marketplace listing drafts from product photos using Gemini Vision.

![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8) ![License](https://img.shields.io/badge/license-MIT-blue)

## What it does

- Takes a folder of phone photos for one item and produces a ready-to-edit Markdown listing.
- Generates honest descriptions in English and German for both Vinted DE and Kleinanzeigen.
- Detects the category automatically (clothing, shoes, books, 3D-printed, household, electronics).
- Reads labels, ISBNs, insole text, and visible flaws straight from the photos via Gemini Vision.

## How it works

```
photo folders -> Gemini 2.5 Flash -> JSON -> markdown
```

Each subfolder in `INPUT_DIR` represents one item. The folder name is passed to the model as a hint, the photos are sent as inline image parts, and the model returns a structured JSON Item that is rendered into a `{slug}.md` file.

## Setup

Requires Go 1.25+.

```bash
git clone https://github.com/tanjaduro/market-lister && cd market-lister
cp .env.example .env
# Edit .env: set GEMINI_API_KEY (free key at https://aistudio.google.com/apikey)
go build .
```

## Usage

```bash
./market-lister                            # process all subfolders in INPUT_DIR
./market-lister -folder "reima pink 24"    # process one specific folder
./market-lister -output /path/to/dir       # write .md elsewhere (image links use absolute paths)
./market-lister -dry-run                   # list folders that would be processed, no API calls
```

Each run prints a summary at the end: `done=N skipped=M failed=K`. Items that already have a `{slug}.md` at the target are skipped, so re-running the same command is safe and idempotent.

## Output format

For a folder named `reima pink 24` containing 8 photos, the generated `reima-pink-24.md` looks like:

```markdown
---
id: "2026-05-14-reima-pink-24"
status: draft
platforms: [vinted, kleinanzeigen]
category: shoes
condition: used-good
price_estimate_eur: 10
source_folder: "reima pink 24"
date_added: 2026-05-14
attributes:
  brand: "Reima"
  size_eu: "24"
  size_cm: "16.00"
  color: "pink"
flaws:
  - "white midsole scuffed and yellowed at toe area"
---

# Reima pink kids sneakers, EU 24 / US 8 (16 cm)

## Vinted (English)

Reima kids sneakers in pink, size EU 24 (US 8 / 16 cm insole). ...

## Kleinanzeigen (Deutsch)

Verkaufe Reima Kinder-Sneaker in Rosa, Größe 24 ... Abholung in Panketal
oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme.

## Photos

![](<20260513_101843.jpg>)
```

## Supported categories

- `clothing` (adult and children's, including kids' size hints)
- `shoes` (EU/US/CM sizes read from insole)
- `books` (title, author, ISBN, publisher)
- `3d-printed` (material guessed from appearance)
- `household` (vases, kitchen items, decor)
- `electronics` (brand, model, screen condition, accessories)
- `other` (fallback)

## Design notes

The Gemini Go SDK was unified in 2025 into `google.golang.org/genai`. Inline images via `genai.NewPartFromBytes`, system prompt via `GenerateContentConfig.SystemInstruction`, and structured output via a typed `*genai.Schema` (not a JSON string) — these are the API points worth getting right and the easiest to get wrong.

Gemini 2.5 Flash was chosen for its free tier (1,500 requests/day) and 20 MB inline image limit. That headroom means raw phone photos can be sent without any preprocessing.

A single universal `Item` struct with a category-flexible `Attributes map[string]string` keeps the codebase flat. There are no per-category subtypes, no interfaces, no factories — the vision prompt names the keys it should populate for each category, and the renderer treats them uniformly.

Output is plain Markdown with no Obsidian-specific syntax, so generated files render in any viewer. Image links use angle-bracket syntax (`![](<path>)`) because folder names often contain spaces.

By default, the `.md` is written into the source photo folder. Setting `OUTPUT_DIR` separates listings from photos and rewrites image links to absolute paths back into the source folder.

## Troubleshooting

**`context deadline exceeded` in the log.** A single folder took longer than `REQUEST_TIMEOUT_SECONDS` (default 180 s) to upload and complete inference. The other folders in the batch are unaffected; only that one is counted as `failed`. If it happens repeatedly, raise the value in your `.env` (e.g. `REQUEST_TIMEOUT_SECONDS=300`) — usually a slow upload on a thin connection, not a Gemini-side problem.

## Roadmap

- Prompt evaluation suite under a sibling `evals/` directory using promptfoo.
- Optional mdbook catalog renderer in a separate repo for browsing generated listings.
