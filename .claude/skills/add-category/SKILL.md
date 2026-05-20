---
name: add-category
description: Use when adding, removing, or renaming a category or condition enum value (e.g. adding "toys" to categories, or "for-parts" to conditions). Enum threading across item.go, the JSON Schema, and prompt.txt — plus the markdown.go vintedConditionDE map when the value is a condition. No struct field changes. Does not apply to adding new top-level fields or attribute keys — use add-listing-field for those.
---

# Add a category or condition

Adding a new enum value is enum threading — no struct field changes. Categories stay out of the markdown template; conditions touch one template-side map (the `vintedConditionDE` Vinted DE vocabulary mapping).

## Category — 4 sites

1. **item.go** — add the value to `validCategories` (item.go:23)
2. **vision.go** — add the value to the `enum` slice for `category` inside `itemJSONSchema()` (vision.go:183)
3. **prompt.txt** — add the value to the category list under `Field constraints:` (prompt.txt:35)
4. **prompt.txt** — add a line to the per-category attribute block under step 4 "Keys per category" (prompt.txt:15-21) listing the attribute keys the model should populate for items in this category

## Condition — 4 sites

1. **item.go** — add the value to `validConditions` (item.go:34)
2. **vision.go** — add the value to the `enum` slice for `condition` inside `itemJSONSchema()` (vision.go:187)
3. **prompt.txt** — add the value to the condition list under `Field constraints:` (prompt.txt:36)
4. **markdown.go** — add the value to the `vintedConditionDE` map (markdown.go:23), mapped to its Vinted DE picker vocabulary string ("Neu mit Etikett", "Neu ohne Etikett", "Sehr gut", "Gut", "Befriedigend"). Without this, the template renders an empty `vinted_condition`.

There is no attribute-guidance line for conditions — they have no per-condition attribute set.

## Future: requiredAttributes map

NEXT-STEPS §6.2 proposes a `requiredAttributes` map in item.go that `validateItem` will check per category. When that lands, adding a category becomes a 5th site — the new key in that map (with a list of mandatory attribute keys for that category). Skip until the map exists.

## Removing or renaming

- **Remove:** drop from all 4 sites. Before deleting, `grep -r "<value>" .` to catch any markdown test fixtures, eval fixtures, or doc references that still mention it.
- **Rename:** same sites, plus any places the old name appears as a string literal (test fixtures, markdown_test.go expected substrings, NEXT-STEPS / PLAN references).

## Verification

```
go vet ./... && go test ./...
```

`TestValidateItem` should still pass on existing items. If you added a value, add a `TestValidateItem` case asserting an item with that category/condition is accepted.

When adding a **condition**, `TestVintedConditionDECoversAllConditions` (markdown_test.go) fails until site 4 is done — that is the guard catching a missing Vinted DE mapping.
