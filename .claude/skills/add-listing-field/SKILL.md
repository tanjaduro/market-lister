---
name: add-listing-field
description: Use when adding, renaming, or removing a field on listings — top-level fields on the Item struct (threaded across item.go, the JSON Schema, prompt.txt, validateItem, the markdown template, and tests) or per-category attributes inside Item.Attributes (which only touch prompt.txt). Does not apply to enum value additions — use add-category for those.
---

# Add a listing field

A top-level field on `Item` must land in 5 source sites + 2 test sites. Missing any site produces silent drift: the model returns data the renderer ignores, or the validator rejects valid output.

Line hints (`~file:NN`) are navigation aids only — they drift as the files change. The named symbol is the real anchor; `grep` for it.

## Top-level field checklist

1. **item.go** — add the field to `type Item struct` (~item.go:6) with a `json` tag matching what the model returns
2. **vision.go** — add the property inside the `properties` map of `func itemJSONSchema()` (~vision.go:188) and append the key to its `required` slice (~vision.go:216)
3. **prompt.txt** — instruct the model when and how to populate it; if the field is an enum or has a length cap, add the rule under the `Field constraints:` header (~prompt.txt:33)
4. **vision.go** — if the field has a constraint (enum membership, length, format, numeric bound), add a check in `func validateItem` (~vision.go:140)
5. **markdown.go** — render the field in the `tmplRaw` template constant (~markdown.go:34). A raw field is `{{.Item.FieldName}}` directly; a *derived* value (computed from the Item, like `VintedConditionDE`) also needs a `tmplData` struct field and a line in `Render`
6. **vision_test.go** — if step 4 added a constraint, add a `TestValidateItem` case (~vision_test.go:101) covering both acceptance and rejection
7. **markdown_test.go** — populate the field on the existing test items and add a `wantContains` assertion in the `cases` table of `TestRender` (~markdown_test.go:73)

See `references/worked-example.md` for a complete trace adding a hypothetical `weight_grams int`.

## Per-category attribute (different path)

Keys like `brand`, `size_eu`, `isbn`, `model_name` live in `Item.Attributes` (a `map[string]string`). To add one:

1. **prompt.txt** — add the key to the matching category's line under step 4 "Keys per category" (~prompt.txt:15-21)
2. *(future)* — when the `requiredAttributes` map from NEXT-STEPS §6.2 lands, add the key there too

No struct change. No schema change. No validator change. The map is open by design.

## Rename or remove

- Rename: same 7 sites, same change. Search for the old JSON tag name across the repo first to catch test fixtures and markdown assertions.
- Remove: drop from all 7 sites. The `required` array entry in `itemJSONSchema` and the template line in `tmplRaw` are easiest to forget.

## Verification

```
go vet ./... && go test ./...
```

If you added a `validateItem` check, the new vision_test.go case should cover the *rejection* path explicitly — acceptance alone doesn't prove the check fires.
