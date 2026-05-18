---
name: add-listing-field
description: Use when adding, renaming, or removing a field on listings — top-level fields on the Item struct (threaded across item.go, the JSON Schema, prompt.txt, validateItem, the markdown template, and tests) or per-category attributes inside Item.Attributes (which only touch prompt.txt). Does not apply to enum value additions — use add-category for those.
---

# Add a listing field

A top-level field on `Item` must land in 5 source sites + 2 test sites. Missing any site produces silent drift: the model returns data the renderer ignores, or the validator rejects valid output.

## Top-level field checklist

1. **item.go** — add the field to the `Item` struct (item.go:6) with a `json` tag matching what the model returns
2. **vision.go** — add the property to `itemJSONSchema()` (vision.go:163) and append the key to the `required` slice (vision.go:193)
3. **prompt.txt** — instruct the model when and how to populate it; if the field is an enum or has a length cap, add the rule under `Field constraints:` (prompt.txt:31)
4. **vision.go** — if the field has a constraint (enum membership, length, format, non-negative number, etc.), add a check in `validateItem` (vision.go:128)
5. **markdown.go** — render the field in `tmplRaw` (markdown.go:23): YAML frontmatter, Details section, or wherever it belongs semantically
6. **vision_test.go** — if step 4 added a constraint, add a `TestValidateItem` case (vision_test.go:101) covering both acceptance and rejection
7. **markdown_test.go** — populate the field on the existing test items and add a `wantContains` assertion (markdown_test.go:67)

See `references/worked-example.md` for a complete trace adding a hypothetical `weight_grams int`.

## Per-category attribute (different path)

Keys like `brand`, `size_eu`, `isbn`, `material` live in `Item.Attributes` (a `map[string]string`). To add one:

1. **prompt.txt** — add the key to the matching category's attribute list (prompt.txt:13-19)
2. *(future)* — when the `requiredAttributes` map from NEXT-STEPS §6.2 lands, add the key there too

No struct change. No schema change. No validator change. The map is open by design.

## Rename or remove

- Rename: same 7 sites, same change. Search for the old JSON tag name across the repo first to catch test fixtures and markdown assertions.
- Remove: drop from all 7 sites. Required-array entry in `itemJSONSchema` and template line in `tmplRaw` are easiest to forget.

## Verification

```
go vet ./... && go test ./...
```

If you added a `validateItem` check, the new vision_test.go case should cover the *rejection* path explicitly — acceptance alone doesn't prove the check fires.
