# Listing Quality Plan

Design record for the listing-quality improvement effort. Append-only; later supersessions live as notes below the original entry (mirroring the PLAN.md / NEXT-STEPS.md convention from CLAUDE.md).

Scope: improve the *content* of generated listings — search-optimized titles, Vinted condition vocabulary, marketplace boilerplate, price ranges, selling-oriented descriptions, and product enrichment via web search. Behaviour outside listing content (CLI flags, concurrency, evals, CI) is out of scope here and tracked in `NEXT-STEPS.md`.

The body uses a concrete vocabulary: when something says "add field X to Item", it means the exact Go field name, JSON tag, schema entry, and template binding listed in §2. Where SDK behaviour or API constraints are claimed, the verification source is named.

---

## Section 1 — `prompt.txt` changes

### 1.1 Field-list summary (what the model must now return)

After the changes in §2, the schema has these top-level fields (✱ = new, removed = struck through):

```
title_en              (kept)
title_de              (kept — descriptive German title, used as H1 in markdown)
title_vinted_de ✱     (search-optimized Vinted DE title, ≤80 chars)
category              (kept)
condition             (kept — internal enum; Vinted vocab is rendered by template)
flaws                 (kept)
description_vinted_en (kept — rewritten tone, see §1.3)
description_vinted_de (kept — rewritten tone, see §1.3)
description_kleinanzeigen_de  (kept — disclaimer constraint unchanged)
price_min_eur ✱       (negotiation floor, ≥0)
price_max_eur ✱       (asking price, ≥ price_min_eur)
~~price_estimate_eur~~ (REMOVED — see §2 for migration of existing tests)
attributes            (kept; new optional keys documented in §1.4)
ocr_notes             (kept)
```

Rationale for removing `price_estimate_eur`: a single midpoint cannot encode the negotiation envelope the user actually needs. Keeping it alongside min/max creates three numbers the model must keep consistent, which is exactly the kind of redundancy that drifts. The solo-repo constraint from memory `[[feedback_solo_repo_no_pr]]` means we can break the field without a deprecation cycle.

### 1.2 `prompt.txt` — before/after

The file is currently 45 lines. Below: line ranges from the current file → new content. Replace each region verbatim. The Kleinanzeigen disclaimer instruction stays exactly as-is (line 41-42) — validation in `validateItem` enforces it and the test gauntlet at vision_test.go:133-146 locks the contract in.

#### 1.2.1 Replace the opening paragraph (current line 1)

Before:

```
You are generating a draft marketplace listing for a second-hand item being sold by a private seller in Germany. The seller will manually copy the descriptions to Vinted (DE) and Kleinanzeigen, so accuracy and honesty matter more than persuasive language.
```

After:

```
You are generating a draft marketplace listing for a second-hand item being sold by a private seller in Germany. The seller will manually copy the descriptions to Vinted (DE) and Kleinanzeigen. Honesty about condition and flaws matters most — second-hand buyers reward accuracy. But within those bounds, your output should sell: front-load search keywords in titles, name specific product features when visible, and use the casual marketplace tone Vinted buyers respond to (not academic, not formal).
```

Reason: the current prompt explicitly tells the model to under-sell ("accuracy and honesty matter more than persuasive language"), which contributed to the bland Affenzahn output. The new wording keeps the honesty constraint but removes the implicit instruction to be bland.

#### 1.2.2 Add a new rule between current step 2 and step 3 (after line 11)

Insert as new step 3, renumber the rest:

```
3. Identify model name and design/colour name when visible. Many products carry an internal model name on the insole, hangtag, or care label (e.g. "Knit Happy Bear" on Affenzahn shoes) and a design name distinct from the colour (e.g. "Papagei", "Paradiesvogel"). When you can see such a name, include it under `attributes.model_name` and `attributes.design_name`. If only one is visible, set the other to "unknown". Do not invent.
```

Reason: the Affenzahn gap example identified model name and design name as the two missing pieces that grounding cannot necessarily recover from photos alone — but a careful read of the photos sometimes can.

#### 1.2.3 Replace the attribute key list (current line 13-19)

Before:

```
3. Populate the `attributes` map with category-relevant keys:
   - clothing: brand, size, age_range (if children's), material, color, gender
   - shoes: brand, size_eu, size_us, size_cm, material, color
   - books: title, author, isbn, publisher, year, language, edition
   - 3d-printed: material (PLA/PETG/ABS/resin — guess from appearance), color, approximate_dimensions, use_case
   - household: brand, type, dimensions, material
   - electronics: brand, model, year, condition_of_screen, accessories_included
```

After (now step 4 after the insertion above):

```
4. Populate the `attributes` map with category-relevant keys. Always include `model_name` and `design_name` from step 3 when applicable. Keys per category:
   - clothing: brand, size, age_range (if children's), material, color, gender, model_name, design_name, fastener (zip, button, snap, hook-eye, none)
   - shoes: brand, size_eu, size_us, size_cm, material, color, model_name, design_name, fastener (velcro, lace, slip-on, zip), sole_type (barefoot, cushioned, rubber), waterproof_membrane (Gore-Tex, SympaTex, tex, none, unknown), reflectors (yes, no, unknown)
   - books: title, author, isbn, publisher, year, language, edition
   - 3d-printed: material (PLA/PETG/ABS/resin — guess from appearance), color, approximate_dimensions, use_case
   - household: brand, type, dimensions, material
   - electronics: brand, model, year, condition_of_screen, accessories_included
```

Reason: the Affenzahn example listed *Klettverschluss* (velcro fastener), *flexible Sohle* / *barfuss* (barefoot sole), and *Reflektoren an der Ferse* as the differentiators that sold the listing. These need named slots so the description writer (step 7 below) can reach for them by key. The added keys for clothing (`fastener`) and shoes (`fastener`, `sole_type`, `waterproof_membrane`, `reflectors`) close the gap.

Per `[[skill: add-listing-field]]`, per-category attribute keys only require edits to `prompt.txt`, not the Go struct or schema.

#### 1.2.4 Replace the price estimation rule (current line 23)

Before:

```
5. Estimate a realistic second-hand price in EUR for the German market. Children's clothing: typically 4-15 EUR. Children's shoes: 8-25 EUR. Books: 2-8 EUR. 3D-printed parts: 3-15 EUR depending on size and complexity. Household: highly variable. Be conservative — sellers can adjust upward, not downward, after seeing the draft.
```

After (renumbered to step 6):

```
6. Estimate a realistic second-hand price RANGE in EUR for the German market. Output two integers: `price_min_eur` is the lowest you would accept after typical Vinted negotiation, `price_max_eur` is the asking price you would list at. The seller lists at `price_max_eur` and accepts down to `price_min_eur`. A typical Vinted range is min ≈ 0.8 × max for items priced 10-30 EUR. Reference bands (asking price, i.e. `price_max_eur`): children's clothing 5-18 EUR, children's shoes 10-28 EUR, branded barefoot/quality shoes 15-35 EUR, books 2-8 EUR, 3D-printed parts 3-15 EUR, household highly variable, electronics highly variable. Be honest about flaws-driven discounts but do not under-ask — sellers can lower a list price, not raise it after negotiation has anchored. `price_min_eur` must be ≥ 0 and ≤ `price_max_eur`.
```

Reason: encodes the two-number range, captures the user's observed pricing (Affenzahn listed at 17 EUR sold ~15 EUR — a min/max pair, not a single estimate), bumps the children's shoes ceiling so the model isn't anchored at 15 EUR for premium brands.

#### 1.2.5 Replace the field constraints block (current line 31-35)

Before:

```
Field constraints:

- `category`: one of clothing, shoes, books, 3d-printed, household, electronics, other.
- `condition`: one of new-with-tags, used-excellent, used-good, used-fair.
- `title_en` and `title_de`: 70 characters or fewer.
```

After:

```
Field constraints:

- `category`: one of clothing, shoes, books, 3d-printed, household, electronics, other.
- `condition`: one of new-with-tags, used-excellent, used-good, used-fair. The downstream template maps these to Vinted DE vocabulary; do not output Vinted vocabulary here.
- `title_en` and `title_de`: 70 characters or fewer. `title_de` is a *descriptive* German title used as the document heading.
- `title_vinted_de`: 80 characters or fewer. Format: `{Brand} {German category noun} Gr. {Size} {Design or colour name} {Fastener if relevant}`. Examples: `Affenzahn Barfußschuhe Gr. 23 Papagei Klett`; `Zara Blumenkleid Gr. 104 rosa`; `H&M T-Shirt Gr. M weiß mit Print`. German throughout, brand first, size early, design/colour and fastener as keywords. Omit segments that don't apply but never invent.
- `price_min_eur`, `price_max_eur`: see step 6.
```

Reason: introduces `title_vinted_de` as a separately constrained field, names the Vinted DE search convention explicitly (brand-first, size early), and tells the model that the internal condition enum is *not* the Vinted-facing string (which is rendered downstream — see §3).

#### 1.2.6 Replace the Descriptions block (current line 37-42)

Before:

```
Descriptions:

- description_vinted_en: 2-4 sentences in English. Honest, concise, mention flaws and any non-visible details that need the seller to confirm.
- description_vinted_de: same content in German, natural marketplace tone (du-neutral, not overly formal).
- description_kleinanzeigen_de: 3-5 sentences in German, marketplace-natural. MUST end with this exact text:
  "Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme."
```

After:

```
Descriptions:

Each description should sell within honesty bounds. Lead with the strongest concrete detail you can see: model name, design name, a named feature (Reflektoren, flexible Sohle, Gore-Tex, neuwertig). Mention specific features by name from `attributes` rather than generic adjectives. Acknowledge flaws clearly using the noun ("kleine Macke an der Spitze", "leichtes Pilling am Saum") rather than hedged adjectives ("etwas getragen"). Casual tone, du-form is fine, no Sie-form, no sales-pitch superlatives ("perfekt!", "MUSS HABEN!!!"). Boilerplate phrases like "Nichtraucherhaushalt", "Versand möglich", or "Bitte Fotos beachten" are added downstream by the template — do NOT include them yourself.

- description_vinted_en: 2-4 sentences in English. Lead with model/design name if known, then condition and the most material flaw.
- description_vinted_de: 2-4 sentences in German, same content philosophy, casual marketplace tone.
- description_kleinanzeigen_de: 3-5 sentences in German, marketplace-natural. MUST end with this exact text:
  "Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme."
```

Reason: the current "honest, concise" rule produces correct but lifeless output. The new wording keeps the no-invention rule (rooted in step 2 and the unchanged "Critical rules" section) but explicitly directs the model to name features from `attributes` and use marketplace vocabulary. Adds the explicit "do not include boilerplate" rule so the template can own those strings cleanly (§3.3).

#### 1.2.7 Keep verbatim

These regions stay byte-for-byte identical and must not be touched:

- Line 25 "Critical rules:" through line 29 (the no-invention rules — same enforcement as today).
- Line 41-42 the Kleinanzeigen disclaimer string (validated in `validateItem`).
- Line 44 "Return ONLY valid JSON matching the requested schema. No markdown fences, no preamble, no explanation outside the JSON."

This preserves the invariants enforced by the existing `edit-prompt` skill rule: Kleinanzeigen disclaimer stays verbatim, enum lists stay aligned, no inline comments inside `prompt.txt`.

### 1.3 Tone shift — checklist of what to keep vs rewrite

| Concept | Status | Why |
|---|---|---|
| "accuracy and honesty matter" | **Rewritten** (1.2.1) — honesty kept, "more than persuasive language" dropped | Was the root cause of bland output |
| "Read every label, tag, ISBN…" (line 11) | **Keep** | Drove Gore-Tex / SympaTex behaviour; unchanged |
| Attribute keys per category | **Expanded** (1.2.3) | Added model_name, design_name, fastener, sole_type, waterproof_membrane, reflectors |
| Flaw detection rule (line 21) | **Keep** | "heavy pilling" not "slight pilling" is the right calibration |
| Price estimate (single number) | **Replaced** (1.2.4) — now a range | Negotiation envelope matters |
| "If a brand…not visible, set to unknown" | **Keep** (line 27) | Same no-invention rule |
| Folder-hint-vs-photos rule (line 29) | **Keep** | Eval case 10 depends on it |
| Title rules | **Expanded** (1.2.5) — added `title_vinted_de` | New search-optimized field |
| Description tone | **Rewritten** (1.2.6) | Concrete features, casual tone, no boilerplate from model |
| Kleinanzeigen disclaimer string | **Keep verbatim** (line 41-42) | Validated by `validateItem`; test gauntlet locks it in |
| "Return ONLY valid JSON" (line 44) | **Keep** | Structured-output discipline |

---

## Section 2 — `Item` struct and schema changes

### 2.1 `item.go` — exact edits

Before (item.go:6-18):

```go
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

After:

```go
type Item struct {
	TitleEN                    string            `json:"title_en"`
	TitleDE                    string            `json:"title_de"`
	TitleVintedDE              string            `json:"title_vinted_de"`
	Category                   string            `json:"category"`
	Condition                  string            `json:"condition"`
	Flaws                      []string          `json:"flaws"`
	DescriptionVintedEN        string            `json:"description_vinted_en"`
	DescriptionVintedDE        string            `json:"description_vinted_de"`
	DescriptionKleinanzeigenDE string            `json:"description_kleinanzeigen_de"`
	PriceMinEUR                int               `json:"price_min_eur"`
	PriceMaxEUR                int               `json:"price_max_eur"`
	Attributes                 map[string]string `json:"attributes"`
	OCRNotes                   string            `json:"ocr_notes"`
}
```

`validCategories` and `validConditions` are unchanged.

### 2.2 `vision.go` — schema edits

Before (vision.go:163-199, the `itemJSONSchema` function):

The `properties` map currently has `title_en`, `title_de`, `category`, `condition`, `flaws`, three descriptions, `price_estimate_eur`, `attributes`, `ocr_notes`. The `required` slice lists all of those.

After:

```go
func itemJSONSchema() any {
	stringType := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title_en":        stringType,
			"title_de":        stringType,
			"title_vinted_de": stringType,
			"category": map[string]any{
				"type": "string",
				"enum": []string{"clothing", "shoes", "books", "3d-printed", "household", "electronics", "other"},
			},
			"condition": map[string]any{
				"type": "string",
				"enum": []string{"new-with-tags", "used-excellent", "used-good", "used-fair"},
			},
			"flaws": map[string]any{
				"type":  "array",
				"items": stringType,
			},
			"description_vinted_en":        stringType,
			"description_vinted_de":        stringType,
			"description_kleinanzeigen_de": stringType,
			"price_min_eur":                map[string]any{"type": "integer"},
			"price_max_eur":                map[string]any{"type": "integer"},
			"attributes": map[string]any{
				"type":                 "object",
				"description":          "Category-relevant attribute keys (brand, size, isbn, material, model_name, design_name, fastener, sole_type, waterproof_membrane, reflectors, etc.) mapped to string values.",
				"additionalProperties": stringType,
			},
			"ocr_notes": stringType,
		},
		"required": []string{
			"title_en", "title_de", "title_vinted_de",
			"category", "condition", "flaws",
			"description_vinted_en", "description_vinted_de", "description_kleinanzeigen_de",
			"price_min_eur", "price_max_eur",
			"attributes", "ocr_notes",
		},
	}
}
```

Diff vs current: add `title_vinted_de` after `title_de` in properties and required; replace `price_estimate_eur` with `price_min_eur` and `price_max_eur` in both properties and required; expand the `attributes` description string for documentation.

### 2.3 `validateItem` changes

Before (vision.go:128-149) checks: title_en ≤70, title_de ≤70, category in set, condition in set, price_estimate_eur ≥0, disclaimer suffix.

After: add `title_vinted_de` length check, replace the single price check with the min/max pair, keep everything else verbatim.

```go
func validateItem(item Item) error {
	if len([]rune(item.TitleEN)) > 70 {
		return fmt.Errorf("title_en exceeds 70 chars")
	}
	if len([]rune(item.TitleDE)) > 70 {
		return fmt.Errorf("title_de exceeds 70 chars")
	}
	if len([]rune(item.TitleVintedDE)) > 80 {
		return fmt.Errorf("title_vinted_de exceeds 80 chars")
	}
	if !validCategories[item.Category] {
		return fmt.Errorf("invalid category %q", item.Category)
	}
	if !validConditions[item.Condition] {
		return fmt.Errorf("invalid condition %q", item.Condition)
	}
	if item.PriceMinEUR < 0 {
		return fmt.Errorf("price_min_eur is negative")
	}
	if item.PriceMaxEUR < 0 {
		return fmt.Errorf("price_max_eur is negative")
	}
	if item.PriceMinEUR > item.PriceMaxEUR {
		return fmt.Errorf("price_min_eur (%d) exceeds price_max_eur (%d)", item.PriceMinEUR, item.PriceMaxEUR)
	}
	const kleinanzeigenDisclaimer = "Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme."
	if !strings.HasSuffix(strings.TrimSpace(item.DescriptionKleinanzeigenDE), kleinanzeigenDisclaimer) {
		return fmt.Errorf("description_kleinanzeigen_de must end with the required disclaimer")
	}
	return nil
}
```

The `title_vinted_de exceeds 80 chars` check is new. The min/max ordering check (`PriceMinEUR > PriceMaxEUR`) catches model laziness where the model copies the same number into both fields when uncertain, then drifts on one.

### 2.4 Test impacts

Two existing test files reference the changing fields:

**`markdown_test.go`** — three test cases (lines 13-63 fragment shown earlier) each set `PriceEstimateEUR: <int>`. Each must be replaced with the new pair, e.g.:

```go
// Before
PriceEstimateEUR: 10,
// After
PriceMinEUR: 8,
PriceMaxEUR: 12,
```

For all three cases. Additionally each `Item` literal now needs a `TitleVintedDE` field — recommended value: a plausible Vinted-formatted title derived from the existing `TitleDE` (e.g. case 1 → `"Reima Sneaker Gr. 24 rosa"`). The exact strings don't matter for the markdown render test — only that the field exists and propagates to the rendered output. After the test changes, the table-driven assertions in markdown_test.go need to gain checks for the new template sections (§3): `wantContains` for the new "Vinted title", "Vinted condition", "Price range" output.

**`vision_test.go`** — the `validateItem` test scaffold at line 102+ has:

```go
validDisclaimer := "Abholung in Panketal oder…"
// ...
PriceEstimateEUR: 5,
```

Replace with `PriceMinEUR: 4, PriceMaxEUR: 6` and add new sub-test cases:

| New sub-test | Mutation | Want error |
|---|---|---|
| `title_vinted_de exactly 80 ASCII chars` | `i.TitleVintedDE = strings.Repeat("a", 80)` | no |
| `title_vinted_de 81 ASCII chars` | `i.TitleVintedDE = strings.Repeat("a", 81)` | yes |
| `title_vinted_de 81 multibyte runes` | `i.TitleVintedDE = strings.Repeat("ä", 81)` | yes |
| `negative price_min_eur` | `i.PriceMinEUR = -1` | yes |
| `negative price_max_eur` | `i.PriceMaxEUR = -1` | yes |
| `price_min equals price_max ok` | `i.PriceMinEUR = 10; i.PriceMaxEUR = 10` | no |
| `price_min greater than price_max` | `i.PriceMinEUR = 20; i.PriceMaxEUR = 10` | yes |
| `zero prices ok` | `i.PriceMinEUR = 0; i.PriceMaxEUR = 0` | no |

Remove the old `negative price` and `zero price ok` sub-tests (lines 131-132). The disclaimer sub-tests stay untouched.

Per `[[repo-test-conventions]]`: keep the table-driven `wantContains`/`wantNotContains` shape; use `t.Setenv` for any env vars the new code reads (none, in this section); use `t.TempDir` for filesystem work (none, in this section).

### 2.5 Test cases that need *no* change

- `slugify_test.go`, `config_test.go`, `main_test.go` — none of them touch Item field shape.
- `processFolder` skip-path tests — they don't construct an Item literal; nothing to update.

---

## Section 3 — Template changes (`markdown.go`)

### 3.1 New template data fields

`tmplData` (markdown.go:10-17) currently has Date / Slug / FolderName / PhotoBase / Photos / Item. Add no new top-level fields; everything new is derived from `Item` inside the template via helper funcs registered on the template, or via fields on `tmplData` populated by `Render`.

Concretely, extend `tmplData`:

```go
type tmplData struct {
	Date              string
	Slug              string
	FolderName        string
	PhotoBase         string
	Photos            []string
	Item              Item
	VintedConditionDE string // mapped Vinted DE condition vocabulary
}
```

`Render` (markdown.go:99-112) populates `VintedConditionDE` by looking up `Item.Condition` in a new package-level map:

```go
// markdown.go
var vintedConditionDE = map[string]string{
	"new-with-tags":  "Neu mit Etikett",
	"used-excellent": "Sehr gut",
	"used-good":      "Gut",
	"used-fair":      "Befriedigend",
}
```

The condition vocabulary lives in code, not the prompt. Reasons:

1. The mapping is mechanical — every internal value has exactly one Vinted DE equivalent, no judgement involved.
2. The model already commits to the internal enum (validated). A second mapping inside the prompt would burn tokens and risk the model returning a Vinted string into the wrong field.
3. If Vinted ever introduces a new vocabulary term, the change is one map entry, not a prompt edit + eval re-run.

Open gap: Vinted DE also has a "Neu ohne Etikett" tier (between new-with-tags and used-excellent), which our internal enum doesn't represent. Recommendation in §6 below: defer adding it until a real listing demands it. When added, it goes through the `[[skill: add-category]]` skill since it's an enum value addition.

### 3.2 Template edits — exact diffs

The current frontmatter (markdown.go:23-43) and body (lines 45-95) need three edits. Below shows each region before/after; everything not shown is unchanged.

#### 3.2.1 Frontmatter — price field

Before (line 29):

```
price_estimate_eur: {{.Item.PriceEstimateEUR}}
```

After:

```
price_min_eur: {{.Item.PriceMinEUR}}
price_max_eur: {{.Item.PriceMaxEUR}}
vinted_title: "{{.Item.TitleVintedDE}}"
vinted_condition: "{{.VintedConditionDE}}"
```

The Vinted title and Vinted-vocabulary condition are emitted in the frontmatter so any downstream automation can read them without re-mapping.

#### 3.2.2 Body — add a Vinted block right after the H1

Before (line 45-47):

```
# {{.Item.TitleEN}}

## Vinted (English)
```

After:

```
# {{.Item.TitleEN}}

## Vinted listing — copy these

**Title (DE):** {{.Item.TitleVintedDE}}
**Condition (DE):** {{.VintedConditionDE}}
**List price:** {{.Item.PriceMaxEUR}} EUR (accept down to {{.Item.PriceMinEUR}} EUR)

## Vinted (English)
```

This is the section the seller will actually copy into Vinted. The existing English / German / Kleinanzeigen description sections stay below it for context and Kleinanzeigen copy.

#### 3.2.3 Body — add boilerplate to the German descriptions

Before (line 51-57):

```
## Vinted (Deutsch)

{{.Item.DescriptionVintedDE}}

## Kleinanzeigen (Deutsch)

{{.Item.DescriptionKleinanzeigenDE}}
```

After:

```
## Vinted (Deutsch)

{{.Item.DescriptionVintedDE}}

Nichtraucherhaushalt. Bitte Fotos beachten.

## Kleinanzeigen (Deutsch)

{{.Item.DescriptionKleinanzeigenDE}}

Nichtraucherhaushalt. Bitte Fotos beachten.
```

The two boilerplate sentences are appended below the model output, separated by a blank line. The seller copies the whole block including the boilerplate. The boilerplate is a constant — the model never emits it, the prompt in §1.2.6 explicitly forbids it from doing so.

The Kleinanzeigen disclaimer ("Abholung in Panketal…") is still part of `DescriptionKleinanzeigenDE` because (a) validation enforces it as a suffix of that field, (b) splitting the disclaimer into template space would require relaxing validation, which churn outweighs benefit. The boilerplate sentences "Nichtraucherhaushalt" and "Bitte Fotos beachten" are *additional* — not in the disclaimer — and live cleanly in template space.

Note: "Versand möglich" is mentioned in the Kleinanzeigen disclaimer already ("oder Versand gegen Aufpreis möglich"). For Vinted, shipping is platform-handled and saying "Versand möglich" is meaningless. So that boilerplate string from the task brief isn't added — the existing copy already covers it.

#### 3.2.4 Body — replace the Details section price line

Before (line 61-63):

```
- Category: {{.Item.Category}}
- Condition: {{.Item.Condition}}
- Price estimate: {{.Item.PriceEstimateEUR}} EUR
```

After:

```
- Category: {{.Item.Category}}
- Condition: {{.Item.Condition}} (Vinted DE: {{.VintedConditionDE}})
- Price: {{.Item.PriceMinEUR}}-{{.Item.PriceMaxEUR}} EUR
```

### 3.3 What does *not* change

- The H1 (`# {{.Item.TitleEN}}`) — `TitleEN` is the document heading, not a Vinted-facing field. Keeping it stable means existing markdown files render the same H1.
- The Photos section and image rendering.
- The Checklist section.
- The Flaws section and the empty-fallback "None visible.".
- The OCR notes conditional block.
- The Kleinanzeigen disclaimer string and its position at the end of `DescriptionKleinanzeigenDE`.

---

## Section 4 — Gemini grounding integration

### 4.1 SDK / API compatibility — verified

Verified against `google.golang.org/genai v1.57.0` (the version pinned in go.mod) via `go doc`:

- `genai.Tool` exposes `GoogleSearch *GoogleSearch` and the legacy `GoogleSearchRetrieval *GoogleSearchRetrieval` fields. For Gemini 2.5 the documented entry point is `GoogleSearch`.
- `genai.GenerateContentConfig` exposes `Tools []*Tool`, `ResponseSchema *Schema`, `ResponseJsonSchema any`, and `ResponseMIMEType string` — *all settable simultaneously at the SDK level*. The SDK does not validate exclusivity client-side; `models.go` request-construction unconditionally serializes whichever fields are non-nil (verified by reading `responseMimeType` / `responseSchema` / `responseJsonSchema` write paths at models.go:1148-1166 and 1359-1376; no compatibility check).

That is the *SDK* situation. The *Gemini API* is the constraint that matters:

> Google Search grounding (`googleSearch` tool) is documented as incompatible with structured output. When both `tools.googleSearch` and `response_mime_type=application/json` (or `response_schema` / `response_json_schema`) are set, the API rejects the request.

That constraint has been stable across Gemini 1.5 and Gemini 2.5 in the public documentation (a single in-band check would surface this empirically — see verification step in §4.4). The SDK accepts the combination; the service does not.

**Conclusion: a single-call solution combining grounding + structured output is not supported under the current Gemini API.** Plan the two-stage pipeline as the primary path.

### 4.2 Two-stage pipeline — architecture

Stage 1 (existing): `Describe` runs unchanged — no tools, `ResponseJsonSchema=itemSchema`, returns a validated `Item` from photos alone.

Stage 2 (new): `Enrich` — takes the Stage 1 `Item`, runs a text-only Gemini call with `GoogleSearch` enabled and no `ResponseSchema`, fed a prompt that names the brand + category + visible identifying details and asks for model name, design name, and features. Output is parsed loosely into a small `Enrichment` struct and merged into the original `Item.Attributes` and (optionally) descriptions.

Gating:

- New config flag `EnableEnrichment` (env: `ENABLE_ENRICHMENT`, CLI: `-enrich`, default `false`). Verifies behaviour before turning on by default.
- Enrichment only runs for categories where it has plausible value: `shoes`, `clothing`, `electronics`. Skipped for `books` (ISBN already structured), `3d-printed` (no upstream product), `household` (too generic), `other` (no category prior).
- Enrichment skips if `attributes.brand == "unknown"` or absent — without a brand, grounding has nothing to ground on.

Failure of Stage 2 never fails Stage 1: a Stage 2 error logs `slog.Warn("enrichment failed", "folder", …, "error", err)` and the listing is rendered from the Stage 1 `Item` as-is. Enrichment is additive.

### 4.3 `vision.go` — exact code additions

A new file `enrich.go` (preferred to keeping `vision.go` from drifting past 500 LOC per NEXT-STEPS.md §6.1 trigger) contains:

```go
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

//go:embed enrich_prompt.txt
var enrichSystemPrompt string

// Enrichment is the structured payload Stage 2 returns. Fields are best-effort
// — Stage 2 may fill some and leave others empty.
type Enrichment struct {
	ModelName  string   `json:"model_name"`
	DesignName string   `json:"design_name"`
	Features   []string `json:"features"`
	Sources    []string `json:"sources"` // grounding citation URIs, for ocr_notes
}

// shouldEnrich reports whether the item is a candidate for Stage 2.
func shouldEnrich(item Item) bool {
	switch item.Category {
	case "shoes", "clothing", "electronics":
	default:
		return false
	}
	brand := strings.TrimSpace(item.Attributes["brand"])
	if brand == "" || strings.EqualFold(brand, "unknown") {
		return false
	}
	return true
}

// Enrich runs Stage 2 (grounded text call) and folds the result into item.
// On error, logs and returns the original item unchanged.
func Enrich(ctx context.Context, gen contentGenerator, item Item, cfg Config) Item {
	if !cfg.EnableEnrichment || !shouldEnrich(item) {
		return item
	}

	query := buildEnrichQuery(item)
	contents := []*genai.Content{
		genai.NewContentFromParts(
			[]*genai.Part{genai.NewPartFromText(query)},
			genai.RoleUser,
		),
	}
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(enrichSystemPrompt, ""),
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
		// Do NOT set ResponseMIMEType / ResponseSchema / ResponseJsonSchema —
		// the API rejects those alongside GoogleSearch (see §4.1).
	}

	var resp *genai.GenerateContentResponse
	err := retryOnTransient(ctx, func() error {
		var callErr error
		resp, callErr = gen.GenerateContent(ctx, cfg.GeminiModel, contents, config)
		return callErr
	})
	if err != nil {
		slog.Warn("enrichment failed", "error", err)
		return item
	}

	enr, err := parseEnrichment(resp.Text())
	if err != nil {
		slog.Warn("enrichment parse failed", "error", err)
		return item
	}
	return mergeEnrichment(item, enr, resp)
}

// buildEnrichQuery composes the user-side prompt for Stage 2.
func buildEnrichQuery(item Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Identify this second-hand product so I can write a better marketplace listing.\n\n")
	fmt.Fprintf(&b, "Category: %s\n", item.Category)
	if v := item.Attributes["brand"]; v != "" {
		fmt.Fprintf(&b, "Brand: %s\n", v)
	}
	for _, k := range []string{"size_eu", "size", "color", "material", "fastener", "sole_type"} {
		if v := item.Attributes[k]; v != "" && v != "unknown" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	if item.TitleDE != "" {
		fmt.Fprintf(&b, "Best guess title (DE): %s\n", item.TitleDE)
	}
	fmt.Fprintf(&b, "\nSearch the web (otto.de, the brand's own site, Amazon DE) for the exact product. ")
	fmt.Fprintf(&b, "Return the model name, the design/colour name (often distinct from the colour), ")
	fmt.Fprintf(&b, "and up to 5 named features that would help the listing sell ")
	fmt.Fprintf(&b, "(e.g. \"Reflektoren an der Ferse\", \"flexible Sohle\", \"Klettverschluss\", \"Gore-Tex\"). ")
	fmt.Fprintf(&b, "If you cannot identify the product with reasonable confidence, return empty strings — do not invent.\n")
	return b.String()
}

var jsonBlockRE = regexp.MustCompile(`(?s)\{[^{}]*"model_name"[^{}]*\}`)

// parseEnrichment extracts an Enrichment from a model response that may be
// preceded by prose. The grounded model cannot use structured output, so it
// returns text containing a JSON block — we recover it with a permissive regex.
func parseEnrichment(raw string) (Enrichment, error) {
	match := jsonBlockRE.FindString(raw)
	if match == "" {
		return Enrichment{}, fmt.Errorf("no JSON block in enrichment response")
	}
	var enr Enrichment
	if err := json.Unmarshal([]byte(match), &enr); err != nil {
		return Enrichment{}, fmt.Errorf("parse enrichment JSON: %w", err)
	}
	return enr, nil
}

// mergeEnrichment folds enr into item. Existing attributes win over enrichment
// — Stage 1 (photo-grounded) is more authoritative than Stage 2 (web-grounded)
// when both have an opinion. Sources from grounding metadata are appended to
// ocr_notes for the seller to spot-check.
func mergeEnrichment(item Item, enr Enrichment, resp *genai.GenerateContentResponse) Item {
	if item.Attributes == nil {
		item.Attributes = map[string]string{}
	}
	if enr.ModelName != "" && item.Attributes["model_name"] == "" {
		item.Attributes["model_name"] = enr.ModelName
	}
	if enr.DesignName != "" && item.Attributes["design_name"] == "" {
		item.Attributes["design_name"] = enr.DesignName
	}
	if len(enr.Features) > 0 {
		existing := item.Attributes["features"]
		joined := strings.Join(enr.Features, "; ")
		if existing == "" {
			item.Attributes["features"] = joined
		} else {
			item.Attributes["features"] = existing + "; " + joined
		}
	}
	// Append grounding sources to ocr_notes for verifiability.
	if sources := groundingURIs(resp); len(sources) > 0 {
		note := "Enrichment sources: " + strings.Join(sources, ", ")
		if item.OCRNotes == "" {
			item.OCRNotes = note
		} else {
			item.OCRNotes = item.OCRNotes + "\n" + note
		}
	}
	return item
}

// groundingURIs extracts citation URIs from response metadata.
func groundingURIs(resp *genai.GenerateContentResponse) []string {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil
	}
	gm := resp.Candidates[0].GroundingMetadata
	if gm == nil {
		return nil
	}
	var out []string
	for _, c := range gm.GroundingChunks {
		if c.Web != nil && c.Web.URI != "" {
			out = append(out, c.Web.URI)
		}
	}
	return out
}
```

A companion file `enrich_prompt.txt` (embedded via `//go:embed`):

```
You are a product identification assistant. The user is selling a second-hand item and has provided what they can see in the photos (brand, size, visible features). Your job is to search the web and identify the exact product so the user's marketplace listing can name the model, design/colour, and key features accurately.

Output format: a single JSON object inline in your response, with the following keys. Do not wrap it in markdown fences.

{
  "model_name": "string — product/model name as the manufacturer uses it, e.g. 'Knit Happy Bear'. Empty string if unsure.",
  "design_name": "string — design or colour name distinct from a generic colour, e.g. 'Papagei' or 'Paradiesvogel'. Empty string if unsure.",
  "features": ["array of up to 5 short German feature phrases naming concrete attributes, e.g. 'Reflektoren an der Ferse', 'flexible Sohle', 'Klettverschluss', 'Gore-Tex Membran'. Empty array if unsure."]
}

Search for the exact product on otto.de, the manufacturer's website, and Amazon DE. Prefer the manufacturer's own naming. Do NOT invent — if the search results don't give you a confident match, return empty values.

Do not include any explanation outside the JSON object. Cite sources using your grounding tool — the citations will be recovered separately.
```

### 4.4 Pipeline wire-up

`main.go`'s `processFolder` calls `Describe` (vision.go:25) then, after success, calls `Enrich` and uses the returned `Item` for `Render`. The change is a one-line insertion plus the contentGenerator threading from NEXT-STEPS.md §3.1 (which is a prerequisite — see §5 below for ordering).

`Config` (config.go) gains:

```go
EnableEnrichment bool // ENABLE_ENRICHMENT env / -enrich flag, default false
```

### 4.5 Verification step before committing the two-stage path

Before merging the enrichment code, run a one-off probe against the real Gemini API to confirm the documented incompatibility still holds (Google has been softening it case-by-case). The probe is a 10-line program that sends a trivial text prompt with both `Tools: [{GoogleSearch: ...}]` and `ResponseMIMEType: "application/json"`; if the API now accepts both, the plan changes — a single-call approach becomes viable and the two-stage code is redundant.

Output of that probe is the gating evidence for §6 decision G ("two-stage vs single-call"). Until it's run, treat two-stage as the planned path.

### 4.6 Fallback if enrichment quality is poor

If after a few real folders the enrichment frequently returns wrong model names or hallucinates features, two options exist:

1. **Tighten the enrichment prompt** — add concrete examples (Affenzahn, Reima, Affenzahn Knit Happy Bear) and require a confidence threshold ("if you would not bet 80% confidence, return empty"). Cheap.
2. **Disable enrichment by default** for shoes/clothing and keep it on only for electronics, where brand+model lookup is more deterministic. Cheaper.
3. **Drop the feature**. The base prompt has been improved enough in §1 that the listing quality may be sufficient without grounding. If enrichment doesn't pay for itself within ~10 real listings, remove it.

The third option is on the table because the §1 changes alone close most of the quality gap from the Affenzahn example — model name and design name are the only fields enrichment uniquely contributes, and the §1.2.2 step 3 rule may be enough to recover them from the photos when they're physically present.

---

## Section 5 — Migration and backward compatibility

### 5.1 Existing generated `.md` files

The frontmatter shape changes (`price_estimate_eur` → `price_min_eur` + `price_max_eur`; new `vinted_title` and `vinted_condition` keys). Existing files keep parsing as Markdown — YAML frontmatter readers don't object to missing keys — but any downstream tool that reads `price_estimate_eur` from a generated file will see the field is gone.

No such downstream tool exists in this repo today. The only consumer is the seller, who copy-pastes from the body.

Recommendation: **leave existing files alone**. They render fine, and the seller has already used the price estimates that were in them. Re-running on a folder that already has a `.md` is currently a skip (per `processFolder`'s "already exists" path), so no regeneration happens unless the seller deletes the file first.

### 5.2 Eval suite (`NEXT-STEPS.md` §1) impact

The eval suite from NEXT-STEPS.md hasn't been built yet, so it gets built against the new schema directly — no migration cost. The 12 fixture cases need their `expected.json` files updated to include:

- `title_vinted_de` (a plausible Vinted-formatted German title for each case)
- `price_min` and `price_max` per fixture (currently `price_min` / `price_max` are eval-only band bounds — those stay, but the model's `price_min_eur` and `price_max_eur` outputs must both fall inside the per-fixture band, both checked)
- `attributes.model_name` and `attributes.design_name` where the photo set physically shows them
- `attributes.fastener` / `attributes.sole_type` / `attributes.reflectors` for the shoes cases

The forbidden-content assertions from NEXT-STEPS.md §1 stay relevant — "no fabricated brand" is now reinforced by the new no-invention rule for model/design names.

### 5.3 Order of implementation

The changes split into three independent slices and one dependent slice. Each slice is a single commit using the project's `type(scope): subject` convention (CLAUDE.md).

| Slice | Independent? | Files | Commit example |
|---|---|---|---|
| A. Prompt rewrite only | yes | `prompt.txt` | `prompt: rewrite for selling tone + new attribute keys` |
| B. Schema rename `price_estimate_eur` → min/max + add `title_vinted_de` | depends on A's prompt changes for the model to emit them | `item.go`, `vision.go` (schema + validation), `prompt.txt` (constraints block), `vision_test.go`, `markdown_test.go` | `feat(item): add title_vinted_de and price_min/max range` |
| C. Template — Vinted block, boilerplate, condition mapping | depends on B (uses new fields) | `markdown.go`, `markdown_test.go` | `feat(markdown): add Vinted block + boilerplate + condition map` |
| D. Two-stage enrichment | depends on the contentGenerator interface extraction from NEXT-STEPS.md §3.1; depends on C for the merged-attributes to surface in the template | `enrich.go` (new), `enrich_prompt.txt` (new), `config.go` (EnableEnrichment), `main.go` (call site), `enrich_test.go` (new) | `feat(enrich): add Stage 2 grounded product enrichment behind -enrich flag` |

Suggested order: A → B → C → D. A alone meaningfully improves output (the tone rewrite is the single largest lever). B + C complete the schema/template story. D is gated behind a flag and can land last — or never, if §4.6 fallback applies.

Per memory `[[feedback_solo_repo_no_pr]]`: each slice is a direct commit to `main`, not a branch + PR.

CHANGELOG.md (per NEXT-STEPS.md §7.2): each slice gets a line under `[Unreleased]`. Slice B is a minor bump candidate (schema break for the model contract); slice A and C are patches; slice D is a feature behind a flag.

### 5.4 What this plan does *not* touch

To keep scope honest:

- No CLI flag changes beyond `-enrich` in slice D.
- No concurrency, no rate-limit changes, no retry-strategy edits (NEXT-STEPS.md §2 owns those).
- No `add-category` or new condition enum (e.g. "Neu ohne Etikett" — flagged as decision F).
- No multi-marketplace expansion (Vinted FR / NL / PL stays in NEXT-STEPS.md §5).
- No structured `attributes` per-category validation (NEXT-STEPS.md §6.2 owns it — but if it lands before slice B, the required-key tables there should be updated to include `model_name`, `design_name`, `fastener`, `sole_type` for the relevant categories).

---

## Section 6 — Decisions needed

Each entry: the question, the opinionated default, and the rationale. All defaults are non-blocking — overriding any of them changes wording or one line of code, not the plan's shape.

| # | Decision | Default | Rationale |
|---|---|---|---|
| A | Remove `PriceEstimateEUR` entirely vs keep alongside min/max | **Remove** | Solo repo, no external consumers (`[[feedback_solo_repo_no_pr]]`); three numbers drift in three places, two stay consistent |
| B | Condition vocabulary lives in template vs prompt | **Template** | Mechanical 1:1 mapping; no judgement; cheaper to evolve |
| C | Boilerplate location for "Nichtraucherhaushalt" / "Bitte Fotos beachten" | **Template** (appended below each German description) | Constant strings; tokens are free here, drift-proof |
| D | Keep `TitleDE` separately from `TitleVintedDE` | **Keep both** | `TitleDE` is the markdown H1 / descriptive title; `TitleVintedDE` is the search-optimized copy-paste target — different jobs |
| E | Vinted title length cap | **80 chars** | Vinted DE truncates around 80; the existing 70-char `TitleDE` rule stays for the descriptive title |
| F | Add "Neu ohne Etikett" as a new internal condition enum | **Defer** | No real listing has demanded it; adding now means the prompt must distinguish "tags absent but unworn" from "lightly used" purely from photos, which is unreliable. Revisit when a fixture forces the issue |
| G | Grounding via single call vs two-stage pipeline | **Two-stage** | Gemini API documented incompatibility between Google Search tool and structured output; SDK doesn't enforce it but the service does (§4.1). Verification probe (§4.5) gates a possible reversal |
| H | Enrichment default state | **Off** (`-enrich` flag, env `ENABLE_ENRICHMENT=true`) | New surface, needs real-world validation; failure mode of stale/wrong model names would be worse than no enrichment |
| I | Categories enriched | **shoes, clothing, electronics** | Where model name / design name are real differentiators; books are already structured (ISBN), 3d-printed has no upstream product to ground on, household is too generic |
| J | What to do with grounding source URIs | **Append to `OCRNotes`** as a single line | Lets the seller spot-check provenance without adding a top-level field; if it gets verbose, promote to a dedicated `Sources []string` field in a future slice |
| K | Existing `.md` files | **Leave untouched** | Re-running skips existing outputs; the seller has already used them; cost of regeneration is real API spend |
| L | Order of slices | **A → B → C → D** | A is independent and high-leverage; B/C are coupled; D depends on §3.1 interface extraction from NEXT-STEPS.md and on flag gating |
| M | Eval suite update timing | **Update fixtures at the same time as slice B lands** | The schema change invalidates fixtures; do them together so the eval suite is never broken in `main` |

End of plan.
