# Worked example: adding `weight_grams int`

A hypothetical top-level integer field describing shipping weight. Walks all 7 sites.

## 1. item.go — Item struct

```go
type Item struct {
    // ... existing fields ...
    OCRNotes    string `json:"ocr_notes"`
    WeightGrams int    `json:"weight_grams"`
}
```

## 2. vision.go — itemJSONSchema()

Inside the `"properties"` map (around vision.go:167):

```go
"ocr_notes":    stringType,
"weight_grams": map[string]any{"type": "integer"},
```

Append `"weight_grams"` to the `"required"` slice (around vision.go:193).

## 3. prompt.txt

Add a line under the numbered analysis steps or near the price guidance. Keep it imperative and unambiguous:

```
Estimate the item's weight in grams based on visible size and material density. If you cannot estimate (e.g. only close-ups, no scale reference), set to 0.
```

If the field had an enum or length constraint, it would also go under `Field constraints:` (prompt.txt:31).

## 4. vision.go — validateItem (only if there's a constraint)

For a non-negative integer:

```go
if item.WeightGrams < 0 {
    return fmt.Errorf("weight_grams is negative")
}
```

If there is no constraint (any int is fine, including zero for "unknown"), skip step 4 entirely and step 6.

## 5. markdown.go — tmplRaw

YAML frontmatter:

```
weight_grams: {{.Item.WeightGrams}}
```

…and/or in the Details bullet list:

```
- Weight: {{.Item.WeightGrams}} g
```

## 6. vision_test.go

In `TestValidateItem` (vision_test.go:101), add cases covering both sides of the constraint:

```go
{"negative weight rejected", mutate(func(i *Item) { i.WeightGrams = -1 }), true},
{"zero weight ok",           mutate(func(i *Item) { i.WeightGrams = 0 }),  false},
{"positive weight ok",       mutate(func(i *Item) { i.WeightGrams = 350 }), false},
```

## 7. markdown_test.go

Populate the field on at least one existing test item:

```go
reimaItem := Item{
    // ...
    WeightGrams: 350,
}
```

And add to that case's `wantContains`:

```go
wantContains: []string{
    // ...
    `weight_grams: 350`,
},
```

## Verification

```
go vet ./... && go test ./...
```
