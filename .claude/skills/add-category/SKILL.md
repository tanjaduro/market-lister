---
name: add-category
description: Use when adding, removing, or renaming a category or condition enum value (e.g. adding "toys" to categories, or "for-parts" to conditions). Pure enum threading across item.go, the JSON Schema, and prompt.txt — no struct or template changes. Does not apply to adding new top-level fields or attribute keys — use add-listing-field for those.
---

# Add a category or condition

Adding a new enum value is pure enum threading. No struct field changes; no markdown template changes.

## Category — 4 sites

1. **item.go** — add the value to `validCategories` (item.go:21)
2. **vision.go** — add the value to the `enum` slice for `category` inside `itemJSONSchema()` (vision.go:172)
3. **prompt.txt** — add the value to the category list under `Field constraints:` (prompt.txt:33)
4. **prompt.txt** — add a line to the attribute-guidance block (prompt.txt:13-19) listing the attribute keys the model should populate for items in this category

## Condition — 3 sites

1. **item.go** — add the value to `validConditions` (item.go:32)
2. **vision.go** — add the value to the `enum` slice for `condition` inside `itemJSONSchema()` (vision.go:176)
3. **prompt.txt** — add the value to the condition list under `Field constraints:` (prompt.txt:34)

There is no attribute-guidance line for conditions — they have no per-condition attribute set.

## Future: requiredAttributes map

NEXT-STEPS §6.2 proposes a `requiredAttributes` map in item.go that `validateItem` will check per category. When that lands, adding a category becomes a 5th site — the new key in that map (with a list of mandatory attribute keys for that category). Skip until the map exists.

## Removing or renaming

- **Remove:** drop from all 4 (or 3) sites. Before deleting, `grep -r "<value>" .` to catch any markdown test fixtures, eval fixtures, or doc references that still mention it.
- **Rename:** same sites, plus any places the old name appears as a string literal (test fixtures, markdown_test.go expected substrings, NEXT-STEPS / PLAN references).

## Verification

```
go vet ./... && go test ./...
```

`TestValidateItem` should still pass on existing items. If you added a value, add a `TestValidateItem` case asserting an item with that category/condition is accepted.
