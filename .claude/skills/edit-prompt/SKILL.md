---
name: edit-prompt
description: Use when changing, tweaking, or updating prompt.txt, the embedded system prompt, or anything about what the Gemini Vision model is instructed to return. Enforces three invariants — Kleinanzeigen disclaimer remains verbatim, enum lists stay aligned with item.go and the JSON Schema, no inline comments inside prompt.txt.
---

# Edit prompt.txt

prompt.txt is `//go:embed`-ed into the binary (vision.go:18). Every edit is a behavioural change with no compile-time check. The only guardrails are `validateItem` and (eventually) the eval suite. Three invariants are easy to break.

## Invariant 1: Kleinanzeigen disclaimer is character-exact

`validateItem` (vision.go:128) asserts the disclaimer as a literal `HasSuffix` (vision.go:145) against `kleinanzeigenDisclaimer` (vision.go:144) after a `TrimSpace`. The exact required string lives at `references/disclaimer.txt` — diff against that before changing prompt.txt:42.

To change the disclaimer wording itself (rare):

1. Update the literal `kleinanzeigenDisclaimer` const at vision.go:144.
2. Update the quoted string in prompt.txt:42 to match exactly (em-dash, umlauts, spacing).
3. Update the hardcoded disclaimer in vision_test.go (the `validDisclaimer` const around vision_test.go:102 and any case strings).
4. Update the disclaimer that appears in markdown_test.go fixtures.

If only the *guidance around* the disclaimer is changing — not the disclaimer itself — touch only prompt.txt.

## Invariant 2: Enum lists must match three places

`category` and `condition` values appear in:

- `item.go:21` (validCategories) and `item.go:32` (validConditions)
- `vision.go:172` (schema enum for category) and `vision.go:176` (schema enum for condition)
- `prompt.txt:33` (category list) and `prompt.txt:34` (condition list)

Telling the prompt about a value that isn't in `validCategories` / `validConditions` causes the validator to reject every output that uses it. Use the `add-category` skill for enum changes — don't edit prompt.txt's enum lines in isolation.

## Invariant 3: No inline comments

Everything inside prompt.txt is sent to the model as system instruction. Comments inside the file get interpreted as content (worst: included in output) or as meta-instructions (next worst: the model reasons about them and gets distracted). NEXT-STEPS §7.1 forbids them.

Rationale for prompt design goes in:

- The commit message (`prompt: ...`)
- A sibling `prompt-notes.md` that is *not* `//go:embed`-ed

## Verification (before `go test`)

1. Did the change touch the disclaimer wording? → diff char-by-char against `references/disclaimer.txt`.
2. Did the change touch an enum? → stop, use the `add-category` skill instead.
3. Did the change add lines that look like comments (`#`, `//`, `Note:`, `(this is for...)`)? → remove them or move to `prompt-notes.md`.
4. Run `go test ./...`. `TestValidateItem` is the canary — if it fails, the prompt and validator have drifted apart.
