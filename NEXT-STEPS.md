# Next Steps

A working roadmap for market-lister beyond the v1 build. Organised by topic; each section ends with a **Decision needed** callout.

This document is opinionated. Where I recommend a default, the reasoning is stated so you can disagree from a known position.

---

## 1. Prompt evaluation suite (`evals/`)

### Why now

The whole product quality lives in `prompt.txt`. Any tweak to it (and there will be tweaks once Gore-Tex / SympaTex / "check label" rules see real photos) can silently degrade output for unrelated categories. An eval suite is the difference between editing the prompt with confidence and editing it with anxiety.

### Directory layout

```
evals/
├── promptfooconfig.yaml         # provider config + test matrix
├── fixtures/                    # local-only, gitignored
│   ├── shoes-reima-pink-24/
│   │   ├── expected.json        # ground truth Item the prompt should produce
│   │   └── *.jpg                # 4-8 real photos
│   ├── clothing-zara-floral-104/
│   ├── clothing-hm-shirt-pilling/
│   ├── books-stephen-king-it-paperback/
│   ├── books-hardcover-foxing/
│   ├── 3dprinted-petg-bracket/
│   ├── household-vase/
│   ├── electronics-android-phone-scratched/
│   ├── edge-blurry-label/
│   ├── edge-hint-mismatch/
│   ├── edge-gore-tex-label/
│   └── edge-ambiguous-other/
├── helpers/
│   └── assertions.js            # custom JS assertions promptfoo cannot express inline
└── README.md                    # how to populate fixtures, what each case proves
```

`fixtures/` is gitignored. The expected.json files are tiny and could be committed for portability, but the photos are not redistributable. README documents the expected fixture shape so the suite can be re-populated from any item folder set.

### Test cases (12)

For each case the eval encodes: folder name hint (the only text the model gets), pointers to its photos, and the assertions below. Photo descriptions are what the fixtures should depict, not what the model is told.

| # | Case | Photos depict | Category | Condition | Key attribute assertions | Critical descriptive assertions |
|---|------|---------------|----------|-----------|--------------------------|--------------------------------|
| 1 | `shoes-reima-pink-24` | Pink kids' sneaker, yellowed midsole, insole text "reima 24 / US 8 / CM 16.00" | `shoes` | `used-good` | brand=Reima, size_eu=24, size_us=8, size_cm contains "16" | flaws mentions yellowing or scuffing; price 6-15 EUR |
| 2 | `clothing-zara-floral-104` | Children's floral dress, label visible, no flaws | `clothing` | `used-excellent` or `new-with-tags` | brand=Zara, size=104, age_range mentions 3-4 years | `flaws == []`; price 5-12 EUR |
| 3 | `clothing-hm-shirt-pilling` | Adult shirt with clear pilling and a small stain | `clothing` | `used-good` or `used-fair` | brand=H&M, material set | flaws contains both "pilling" and "stain" |
| 4 | `books-stephen-king-it-paperback` | Paperback with ISBN barcode legible | `books` | `used-good` | author=Stephen King, isbn = 13-digit string, language set | price 2-7 EUR |
| 5 | `books-hardcover-foxing` | Older hardcover, dust jacket, visible foxing on page edges | `books` | `used-good` or `used-fair` | edition set; ocr_notes references the visible imprint year | flaws contains "foxing" or "yellowing" |
| 6 | `3dprinted-petg-bracket` | Small bracket, visible print lines, semi-translucent | `3d-printed` | `new-with-tags` or `used-excellent` | material in {PLA,PETG} | price 3-15 EUR |
| 7 | `household-vase-25cm` | Ceramic vase next to a 30cm ruler for scale | `household` | `used-excellent` | dimensions contains "25" or "cm" | price 5-30 EUR |
| 8 | `electronics-android-scratched` | Phone with visible screen scratches, charger in frame | `electronics` | `used-fair` | brand set, model set, accessories_included mentions charger, condition_of_screen mentions scratch | price 15-80 EUR |
| 9 | `edge-blurry-label` | Garment with blurred brand tag | `clothing` | any | brand == "unknown" (strict) | ocr_notes mentions illegibility |
| 10 | `edge-hint-mismatch` | Folder name "nike shoes" but the photos show Adidas | `shoes` | any | brand == "Adidas" | ocr_notes contains "Adidas" and references the discrepancy |
| 11 | `edge-gore-tex-label` | Boot with visible Gore-Tex membrane tag | `shoes` | any | material contains "Gore-Tex" | all three descriptions mention waterproof / Gore-Tex |
| 12 | `edge-ambiguous-other` | Bag of mixed craft buttons | `other` | any | reasonable attributes | ocr_notes explains the "other" choice |

### Image-handling strategy: which option, and why

Three options for getting images into the eval:

| Option | Pros | Cons | Recommendation |
|---|---|---|---|
| **Real curated photos (local only)** | Reflects actual model behaviour on the actual inventory; tests Gore-Tex / foxing / blurry-label cases that no stock photo will surface | Cannot be committed; suite is not portable | **Pick this.** The eval is for your own regression detection, not public demo. |
| **CC0 / stock placeholders** | Committable; CI-friendly | Stock photos are too clean — yellowed midsoles and pilling don't exist in stock libraries. Eval would only certify the trivial cases. | Skip. |
| **Mocked API responses** | No API cost, deterministic | Validates Go orchestration, not the prompt. Defeats the purpose. | Skip — use the Go unit tests for that. |

Promptfoo's `google:gemini-2.5-flash` provider already accepts inline image refs via `{{image:path}}` interpolation, so the eval talks directly to Gemini with `prompt.txt` as the system prompt — no need to shell out to the Go binary. This is intentional: the eval is testing the prompt, not the binary.

### Scoring rubric

Strict-match assertions (deterministic, score 0 or 1):

- `category` is one of the allowed enum values **and** equals the expected value for that fixture.
- `condition` is one of the allowed enum values.
- `description_kleinanzeigen_de` ends with the exact disclaimer string. Regex: `Abholung in Panketal oder Versand gegen Aufpreis möglich\. Privatverkauf — keine Garantie oder Rücknahme\.$`
- All expected attribute keys are present (use `javascript:` assertion checking `output.attributes` has each required key with a non-empty value).
- JSON parses; titles are ≤70 runes; price is a non-negative integer.

Numeric-band assertions (score 0 or 1):

- `price_estimate_eur` falls within `[expected.price_min, expected.price_max]` defined per-fixture. A miss by 1 EUR fails the same as a miss by 100 — keep it binary.

Fuzzy / semantic assertions (`llm-rubric` provider, score 0..1, threshold 0.7):

- "Description honestly mentions the visible flaw [`<flaw>`] without exaggerating severity."
- "Description does not invent details that are not visible in the photos (no fabricated brand, no guessed numerical size)."
- "German description uses natural marketplace tone, not formal Sie-form."

Forbidden-content assertions (`not-contains`):

- Each fixture has a `forbidden` list of strings the model should not produce — e.g. for the blurry-label case: `not-contains: ["Reima", "Nike", "Adidas"]` so we catch hallucination.

### Regression tracking

`npx promptfoo eval -c evals/promptfooconfig.yaml --output evals/results.latest.json` produces a structured result file. Approach:

1. Treat `evals/results.baseline.json` as a checked-in source of truth (per-case pass count and pass rate).
2. A helper script `evals/check-regression.sh` compares latest vs baseline. Fails if any case's pass count drops, or if the overall pass rate drops by more than 5 percentage points.
3. After an intentional prompt improvement, run `make evals-promote` to copy latest→baseline.

Metrics that matter and should appear in the CI summary:

- Per-case pass count (out of N assertions).
- Overall pass rate across all assertions.
- Average price-band hit rate (a sensitive proxy for whether the model still understands the German market).
- Disclaimer-match rate (this is the easiest assertion; a regression here would mean prompt drift, not model drift).

What we are deliberately not tracking: latency, token cost. Different concern.

### Makefile / scripts

```makefile
.PHONY: evals evals-view evals-baseline evals-check

evals:
	cd evals && npx promptfoo@latest eval -c promptfooconfig.yaml --output results.latest.json

evals-view:
	cd evals && npx promptfoo@latest view

evals-baseline:
	cp evals/results.latest.json evals/results.baseline.json
	@echo "Baseline updated. Commit the file."

evals-check: evals
	bash evals/check-regression.sh
```

### Decision needed — section 1

- **Image strategy** — confirm "local only, gitignored". (Recommended.) Alternative: commit a 2-3 CC0 fixtures for a public smoke run.
- **Regression threshold** — 5pp overall drop too strict / lax? Default 5pp.
- **Fuzzy rubric provider** — use the same `gemini-2.5-flash` for `llm-rubric` (cheaper, may grade itself charitably) or `gemini-2.5-pro` (more critical, more expensive)? Default: `gemini-2.5-pro` for grading only.

---

## 2. Robustness & error handling

### 2.1 Rate limiting

Gemini free tier: 15 req/min, 1,500 req/day for `gemini-2.5-flash`. The CLI is sequential, so 15 RPM is reached only if folders process in under 4 s — possible for small folders. The 1,500/day cap is real if you ever batch-import a backlog.

Add:

1. **Daily counter warning** at startup: if a `~/.cache/market-lister/usage-YYYY-MM-DD.count` file exists with a count ≥1,400, warn the user and ask `[y/N]` to continue. Increment on each successful `Describe` call.
2. **Per-minute soft limiter** using `golang.org/x/time/rate.NewLimiter(rate.Every(4*time.Second), 1)` (15 RPM). Block at the start of `Describe`. Doesn't change behaviour at concurrency=1 in practice, but matters once §2.4 lands.
3. **`-max-items N` flag** for capping a run.

Default is no quota tracking — keep current sequential behaviour and rely on the model returning 429, which `retryOnTransient` already handles. Only add the counter if you find yourself hitting the daily cap.

### 2.2 Retry strategy v2

PLAN.md decision #6 says "no retries in v1" but the actual code (vision.go:200-248) already retries with `[5s, 15s]` delays on `503/429/UNAVAILABLE/RESOURCE_EXHAUSTED` markers. Treat the decision as superseded.

Improvements to consider:

- **Replace string-match with typed errors** when the SDK exposes them. Right now `isRetryableError` greps the error string. Fragile if the SDK changes wording. Track upstream: `google.golang.org/genai` v1.57.0 wraps `googleapi.Error` — once we can `errors.As` it and read `.Code`, switch to that.
- **Jittered exponential backoff**. Current is fixed `[5s, 15s]`. With concurrency, multiple goroutines hitting the same 503 retry together is mild thundering-herd. Replace with `base * 2^attempt + jitter` where `jitter = rand.Float64() * base`.
- **Honour Retry-After**. If the 429 response includes a `retry-after` header, parse it and prefer it over our delay. Requires SDK to surface response headers.
- **Cap total wait per folder**. Current implicit cap is `len(retryDelays)+1` × per-call timeout ≤ ~6.5 min on the network path. The folder-level context deadline (default 180 s) already bounds this. Good enough.
- **Classify 400 vs 429 explicitly**. Already correct (400 is non-retryable). Worth a doc comment to lock in the intent.

Retryable in v2:
- HTTP 429 (rate limit) — back off, retry.
- HTTP 503 (overloaded) — back off, retry.
- `context.DeadlineExceeded` from a *transient* SDK timeout (where ctx is still valid) — retry once.
- Network errors (`io.EOF`, `connection reset`) — retry.

Terminal in v2:
- HTTP 400 (bad request, schema mismatch) — fail hard, log raw response.
- HTTP 401/403 (auth) — fail hard, log "check GEMINI_API_KEY".
- HTTP 404 (model not found) — fail hard.
- Folder-level `context.DeadlineExceeded` (the outer ctx is dead) — fail hard.

### 2.3 Context timeout

Default 180 s for up to 15 photos (shipped in 530981b; previously 120). At ~3 MB/photo over a typical home upload (50 Mbps = ~6 MB/s effective), upload is ~8 s. Gemini inference for `gemini-2.5-flash` on 15 inline images is ~10-25 s in practice.

Symptoms of insufficient timeout:

- `context deadline exceeded` errors in the log, one per affected folder.
- That folder gets `resultFailed`, the rest of the batch continues — good behaviour.
- `retryOnTransient` does **not** retry deadline-exceeded errors (the marker list doesn't include "deadline exceeded"). Confirmed: vision.go:237-248. Correct.

Section complete — timeout default bumped to 180 s, both failure paths log `duration_ms`, and the README has a troubleshooting entry for `context deadline exceeded`.

### 2.4 Concurrent processing

Sequential today. Each folder is independent: API call, file write, no shared state. Worker-pool is a clean fit.

Design:

```go
// In Config:
Concurrency int // default 1

// In main, after collecting folders:
sem := make(chan struct{}, cfg.Concurrency)
var wg sync.WaitGroup
counts := struct {
    sync.Mutex
    done, skipped, failed int
}{}

for _, folder := range folders {
    if *dryRun {
        slog.Info("dry-run", "folder", filepath.Base(folder))
        continue
    }
    wg.Add(1)
    sem <- struct{}{}
    go func(f string) {
        defer wg.Done()
        defer func() { <-sem }()
        r := processFolder(cfg, client, f)
        counts.Lock()
        switch r {
        case resultDone: counts.done++
        case resultSkipped: counts.skipped++
        case resultFailed: counts.failed++
        }
        counts.Unlock()
    }(folder)
}
wg.Wait()
```

Configurable via `-concurrency N` flag and `CONCURRENCY` env var. **Default 1** (preserve current behaviour). Realistic ceiling at free tier is 3 — the 15 RPM bucket fills quickly with concurrency.

Rate-limiter must wrap the SDK call so concurrent goroutines respect the bucket:

```go
var rpmLimiter = rate.NewLimiter(rate.Every(4*time.Second), 1)
// inside Describe, before the GenerateContent call:
if err := rpmLimiter.Wait(ctx); err != nil {
    return Item{}, nil, fmt.Errorf("rate limiter: %w", err)
}
```

`slog` is goroutine-safe (the default text handler uses sync writes), so concurrent logging is fine. Output ordering will interleave per folder — acceptable.

Test additions:

- Race detector on by default: change CI to `go test -race ./...`.
- A new test in `main_test.go` that builds a fake input dir with 20 subdirs and runs a no-op `processFolder` to assert the worker pool actually parallelises and that result counts match.

### Decision needed — section 2

- **Rate limit counter** — defer until you hit the daily cap, or build now? Default: defer.
- **Default concurrency** — keep 1 (safest) or 2 (twice the throughput, still within free tier)? Default: keep 1, document the flag.

---

## 3. Testing gaps

### 3.1 `vision.go` — `Describe()` is uncovered

The HTTP-shaped function is the only one without a unit test. It's also where almost all of the interesting failure modes live (model returns invalid JSON, model returns wrong schema, image read fails partway, no images survive MIME detection, ctx cancelled mid-request).

Approach: extract an interface for the one SDK method `Describe` uses, then inject a fake in tests. This is the only refactor in the whole document I'd recommend doing speculatively, because the test surface is large.

```go
// vision.go
type contentGenerator interface {
    GenerateContent(
        ctx context.Context,
        model string,
        contents []*genai.Content,
        cfg *genai.GenerateContentConfig,
    ) (*genai.GenerateContentResponse, error)
}

func Describe(ctx context.Context, gen contentGenerator, folderPath, hint string, cfg Config) (Item, []string, error) { ... }
```

`*genai.Models` already has this method signature, so the type plumbing is:

- New interface declared in `vision.go`.
- `Describe` parameter `client *genai.Client` → `gen contentGenerator`.
- `processFolder` (main.go:118) parameter changes the same way, since it forwards the client.
- `main()` at main.go:81 passes `client.Models` instead of `client`.

Four small edits, all in two files — not a refactor.

Test cases against a fake:

| Test | Setup | Expect |
|---|---|---|
| `TestDescribe_HappyPath` | Folder with one valid PNG. Fake returns canned valid Item JSON. | Returns Item, photo names, nil err |
| `TestDescribe_NoUsableImages` | Folder with one `.txt` and one corrupted `.jpg`. Fake never called. | Returns "no usable images" error; fake call count = 0 |
| `TestDescribe_InvalidJSON` | Fake returns non-JSON text. | Returns "parse response" error; logs raw snippet ≤500 chars |
| `TestDescribe_SchemaViolation` | Fake returns JSON missing the disclaimer. | Returns "validate item" error |
| `TestDescribe_RetryableFailureThenSuccess` | Fake returns 503 then valid JSON. (Combined with shortened `retryDelays`.) | Returns Item; fake called twice |
| `TestDescribe_ContextCancelled` | Fake blocks on ctx; test cancels. | Returns ctx.Err() promptly |
| `TestDescribe_ImageReadFailure` | Folder with one unreadable file (chmod 000) + one valid PNG. | Returns Item; unreadable file is warned-and-skipped; only valid photo in result |

Build tag for the real-API integration test:

```go
//go:build integration
// +build integration

package main

func TestDescribe_IntegrationSmoke(t *testing.T) {
    key := os.Getenv("GEMINI_API_KEY")
    if key == "" { t.Skip("no GEMINI_API_KEY") }
    // Run Describe against a tiny embedded test image.
    // Asserts only: returns no error, category in valid set, condition in valid set.
}
```

Run locally with `go test -tags=integration ./...`. CI runs it only on `main` branch pushes (not PRs from forks) using a low-quota key in a secret.

### 3.2 Expanded `slugify` table

Shipped — slugify_test.go now covers 17 cases (5 original + 12 added) for empty/dashes/non-ASCII/accents/emoji/leading-trailing-hyphens/double-underscore/dotted/mixed-case inputs.

### 3.3 `main.go` and `config.go`

`main_test.go` already covers `collectFolders`, `outputMDPath`, `processFolder` (3 skip paths). Worth adding:

- `TestCollectFolders_ResultIsSorted` — explicit assertion that the returned slice is alphabetical even when `os.ReadDir` returns insertion order. (`os.ReadDir` happens to sort already, but the contract should be tested.)
- `TestProcessFolder_HappyPath` — exercises the full path through `Describe` using the same fake-generator interface from §3.1. Currently impossible because `processFolder` builds the timeout ctx and calls Describe directly. After §3.1's refactor, this becomes feasible.

`config_test.go` is already thorough (4 tests, 7 sub-cases, isolated from environment via `chdirAway` + `t.Setenv`). One addition:

- `TestLoadConfig_EnvWinsOverDotEnv` — write a `.env` to the temp dir with `GEMINI_API_KEY=from-dotenv`, set shell `GEMINI_API_KEY=from-shell` via `t.Setenv`, assert the shell value wins. Confirms the comment in config.go:21 ("Shell-set variables always win") is enforced, not just hoped for.

### 3.4 Coverage target

Realistic target: **80% statement coverage** excluding the SDK network path in `Describe` (which the fake covers logically but not statement-wise once integration tests are tag-gated).

Enforcement in CI:

```yaml
- run: go test -race -coverprofile=cover.out ./...
- run: |
    pct=$(go tool cover -func=cover.out | tail -1 | awk '{print $3}' | tr -d '%')
    awk -v p="$pct" 'BEGIN{exit !(p >= 80.0)}' || (echo "coverage $pct% below 80%"; exit 1)
```

Coverage is a smoke signal, not a quality measure. Don't chase 95% — it just means writing tests for `slog.Info` lines.

### Decision needed — section 3

- **Interface extraction** for `Describe` to enable mocking — approve? (Recommended.)
- **Integration test build tag** — `integration`, or just gate on `GEMINI_API_KEY` env presence inside a regular test that t.Skip's? Default: build tag; cleaner separation, costs nothing.
- **Coverage gate** — 80% strict in CI, 80% warning-only, or no gate? Default: 80% strict.

---

## 4. CI/CD pipeline

### Current state (`.github/workflows/ci.yml`)

- Triggers: push to `main`, all PRs.
- Steps: `go vet`, `go test`, `govulncheck`.
- Missing: lint, race detector, coverage, eval suite.

### Proposed pipeline

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - uses: golangci/golangci-lint-action@v6
        with: { version: latest }
      # govet is enabled in .golangci.yml, no separate step.
      - run: go test -race -coverprofile=cover.out ./...
      - name: enforce coverage
        run: |
          pct=$(go tool cover -func=cover.out | tail -1 | awk '{print $3}' | tr -d '%')
          awk -v p="$pct" 'BEGIN{exit !(p >= 80.0)}' || (echo "coverage $pct% below 80%"; exit 1)
      - uses: actions/upload-artifact@v4
        with: { name: coverage, path: cover.out }
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: govulncheck ./...
      - run: go build .

  evals:
    runs-on: ubuntu-latest
    needs: test
    # Triggers from a separate workflow file with `on: push` + `paths: [prompt.txt]`
    # plus `workflow_dispatch`. Don't use `contains(github.event.head_commit.modified, …)`
    # — head_commit is null on pull_request events and the condition silently goes false.
    if: github.event_name == 'workflow_dispatch' || github.event_name == 'push'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - run: cd evals && npx promptfoo@latest eval -c promptfooconfig.yaml
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY_EVALS }}
```

`.golangci.yml`:

```yaml
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
```

Keep the linter list narrow. `revive`, `gocritic`, and friends will create noise on a 600-LOC codebase.

### `GEMINI_API_KEY` for integration / eval CI

- Forks cannot read repository secrets, so eval jobs would no-op for fork PRs. Restrict the eval job to `workflow_dispatch` (manual trigger) and to push-to-main events. Don't run it on every PR.
- Use a **separate API key** with a separate quota — a dedicated free-tier key tagged `market-lister-evals`. If a CI run burns through 1,500 requests, only the eval workflow stops; the user's local key still works.
- The integration tests from §3.1 can use the same `GEMINI_API_KEY_EVALS` secret on the same eval job.

### Release artifacts

`go install github.com/tanjaduro/market-lister@latest` is the canonical install path. GoReleaser adds:

- Pre-built linux/amd64, darwin/arm64, darwin/amd64, windows/amd64 binaries on git tags.
- A GitHub Release with checksums.
- No homebrew, no scoop — too much config for a solo tool.

**Recommendation: don't ship GoReleaser yet.** `go install` is fine for the one user. Revisit if someone external asks for a binary download.

### Decision needed — section 4

- **golangci-lint** — add now, or wait until something breaks? Default: add now, with the narrow linter list above.
- **Coverage gate strictness** — see §3.4. Default: 80% strict.
- **Eval workflow trigger** — `workflow_dispatch` only, or also auto-run on `prompt.txt` changes? Default: both. The auto-run is the regression net.
- **GoReleaser** — defer. (Recommended.)

---

## 5. Feature ideas — evaluated and prioritised

For each: rough effort (XS / S / M / L), user-visible value, and recommended position.

| Feature | Effort | Value | Position |
|---|---|---|---|
| Photo preprocessing: GPS strip + auto-rotate + downscale | S | High (privacy) | v0.2 |
| Eval suite (§1) | M | High | v0.2 |
| Concurrent processing (§2.4) | M | Medium | v0.3 |
| Interactive `$EDITOR` mode | S | Medium | v0.3 |
| Batch resume / lock files | S | Low | v0.4 if at all |
| Price lookup (ISBN → openlibrary) | M | Low-Medium | v0.4 |
| Multi-language descriptions (FR/NL/PL) | M | Low | v0.5 conditional |
| Web UI for browsing listings | L | High but scope-creep | v1.0+ — separate repo |
| Vinted / Kleinanzeigen API auto-post | XL | High | Drop. See below. |

### Argued ordering

**v0.2 — quality and reach.** Photo preprocessing is privacy-critical (GPS in EXIF is a real leak for marketplace photos taken at home); auto-rotation matches the model's expectation; downscale-on-overflow is the only thing standing between you and a hard failure when 20 MB inline limit is hit. The eval suite locks in the prompt before you start tweaking it for new categories. These two together are the v0.2 release.

**v0.3 — throughput and ergonomics.** Concurrency once the eval suite gives you confidence that prompt tweaks aren't breaking anything quietly. Interactive `$EDITOR` mode is small (3-line shell-out) and matters if you're hand-editing every output.

**v0.4 — niceties.** Resume / lock files only matter if mid-batch crashes become observable, which only happens once concurrency is high enough that "skipped because already exists" stops covering the gap. Price lookup is a guess-improver; defer until evals show price-band accuracy is the dominant complaint.

**v0.5+ — speculative.** Multi-language: only meaningful if you start selling on French or Polish marketplaces. The struct would gain `DescriptionVintedFR`, `DescriptionVintedNL`, `DescriptionVintedPL`, each optional and gated by a `LANGUAGES=en,de,fr` env var. The prompt would need per-language rules and per-marketplace disclaimers (only Kleinanzeigen has the Privatverkauf disclaimer; Vinted FR has a different boilerplate). Real work; don't speculate.

**Web UI — scope honesty.** A read-only browser of generated `.md` files is a weekend project: serve markdown from `OUTPUT_DIR`, render with `goldmark`, basic CSS. Maybe two days. **Inventory management** — mark listed, mark sold, sync platform IDs, track shipping — is at least a month. The way to keep this honest is to scope the weekend version explicitly and resist scope creep: it's a viewer, not a CRM. If you want a CRM, that's a separate repo.

**Marketplace API integration — drop it.** Vinted has no public API for posting; their app uses a private API that is reverse-engineered, ToS-fragile, and rate-limited per device. Kleinanzeigen (eBay Kleinanzeigen → Kleinanzeigen.de under Adevinta) has no public API at all. Browser automation via Playwright works mechanically but: requires manual login, breaks every UI change, violates ToS on both platforms, and the failure mode is account suspension. The honest answer is "copy-paste from the generated `.md`", which is also the current design. **Recommend: don't pursue.**

### Decision needed — section 5

- **v0.2 scope** — both items (photo preprocessing, evals) or just evals first? Default: both.
- **Web UI** — agree it stays a viewer in a separate repo? Default: yes.
- **Marketplace auto-post** — drop entirely? (Recommended.)

---

## 6. Code quality & refactoring

### 6.1 Package split — when?

The codebase is intentionally flat: one `main` package, no interfaces, no factories. Total ~600 LOC of source plus ~700 of tests. This is correct for the current size.

Triggers that would justify splitting:

- Any single file exceeds ~500 LOC. (Currently the largest is vision.go at 248.)
- A clear seam appears: e.g. a second binary (mdbook renderer) or a public library API.
- A test reaches into internal state to such a degree that breaking the export boundary would simplify it.

Proposed split when the time comes:

```
github.com/tanjaduro/market-lister/
├── cmd/market-lister/   main, flags, orchestration
├── internal/
│   ├── config/
│   ├── vision/          Describe, retry, schema
│   ├── listing/         Item, validation, markdown render
│   └── fs/              listImages, collectFolders, slugify
```

**Don't do this now.** No structural improvement; just more import statements.

### 6.2 Per-category attribute validation

`Attributes map[string]string` is the load-bearing piece of the flat design. The vision prompt names the keys it should populate. Today validation only checks `category`, `condition`, `title` length, and disclaimer presence — attributes are not validated.

A pragmatic validator without breaking the "no per-category subtype" rule:

```go
// item.go
var requiredAttributes = map[string][]string{
    "clothing":    {"brand", "size", "material", "color"},
    "shoes":       {"brand", "size_eu", "material", "color"},
    "books":       {"title", "author", "language"},
    "3d-printed":  {"material", "color"},
    "household":   {"type"},
    "electronics": {"brand", "model"},
    "other":       nil,
}

// vision.go, called from validateItem after the category check:
func validateAttributes(item Item) error {
    for _, key := range requiredAttributes[item.Category] {
        v, ok := item.Attributes[key]
        if !ok || v == "" {
            return fmt.Errorf("missing required attribute %q for category %q", key, item.Category)
        }
    }
    return nil
}
```

A value of `"unknown"` or `"check label"` counts as present — that's a deliberate choice. The prompt already instructs the model to use those literals when a value isn't visible (prompt.txt:29-30). The validator's job is to catch model laziness (a missing key) not model honesty (an honest "unknown").

Failure mode: a `"shoes"` listing without a `brand` key would now fail validation, get logged as `validation failed`, and the folder gets `resultFailed` — better than today's silent acceptance of an underspecified listing.

### 6.3 Structured logging

Already using `slog` with the default text handler. Worth adding:

```go
// in main(), before LoadConfig (so config errors are logged at the chosen level):
level := slog.LevelInfo
if s := os.Getenv("LOG_LEVEL"); s != "" {
    switch strings.ToLower(s) {
    case "debug": level = slog.LevelDebug
    case "warn":  level = slog.LevelWarn
    case "error": level = slog.LevelError
    }
}

var handler slog.Handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
if os.Getenv("LOG_FORMAT") == "json" {
    handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
}
slog.SetDefault(slog.New(handler))
```

JSON output is useful for piping into evals or future Web UI; text is the default. `LOG_LEVEL=debug` would surface skipped non-image files, which is currently warn-level noise.

`LOG_FILE` to redirect: marginal value. The shell can do `2> run.log`. Skip.

### Decision needed — section 6

- **Package split** — agree to defer? (Recommended.)
- **Per-category attribute validation** — add now, or wait for evals to reveal which keys actually go missing? Default: add now with the table above. Easy to relax later.
- **`LOG_LEVEL` / `LOG_FORMAT` env vars** — both, just `LOG_LEVEL`, or neither? Default: both, low cost.

---

## 7. Documentation

### 7.1 `prompt.txt` inline comments

Don't add comments inside `prompt.txt`. Anything inside the file is sent to the model as system instruction. Models tend to interpret comments as either content (worst: included in output) or as instructions about the instructions (next worst: model reasons about the meta-text and gets distracted).

If rationale for prompt design needs to live somewhere, put it in a sibling `prompt-notes.md` that is not embedded. Or: lean on git log + commit messages for the same purpose. The current prompt has clean, declarative rules — adding `# this is for waterproof labels` comments would be a regression in clarity.

### 7.2 CHANGELOG.md

Yes. Hand-maintained, SemVer-lite. Today's state is v0.1.0. Format:

```markdown
# Changelog

All notable changes are recorded here.

## [Unreleased]

## [0.1.0] — 2026-05-17

### Added
- Initial release: process photo folders to Gemini, render Markdown listings.
- Support for clothing, shoes, books, 3d-printed, household, electronics, other.
- Retry on 429/503/UNAVAILABLE/RESOURCE_EXHAUSTED.
- Kleinanzeigen disclaimer enforcement via validateItem.
```

Bump policy:

- **Major (1.0)** — when the JSON schema or the CLI flag shape becomes a public contract worth not breaking.
- **Minor (0.x)** — new categories, new flags, new file format.
- **Patch (0.x.y)** — prompt tweaks that improve quality without changing the output schema, bug fixes.

Pre-1.0 there's nothing to break. Don't promise compatibility you don't owe.

### 7.3 CONTRIBUTING.md

Skip. Solo repo; nothing to onboard. Revisit only if outside contributors materialise.

### 7.4 ARCHITECTURE.md

The README's "Design notes" section + PLAN.md already cover it. A separate ARCHITECTURE.md would duplicate without adding. PLAN.md should be updated to reflect that decision #6 is superseded, and a one-line note added that the SDK audit (§"SDK audit") has been executed and findings are in the code. After that PLAN.md is the contributor-facing architecture doc.

### Decision needed — section 7

- **Inline prompt comments** — confirm we never add them. (Recommended.)
- **CHANGELOG.md** — start one now, or wait until v0.2? Default: start now.
- **PLAN.md cleanup** — supersede decision #6 with a note, or rewrite the document? Default: append a "Superseded decisions" section, don't rewrite history.
