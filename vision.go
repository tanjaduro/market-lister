package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/genai"
)

//go:embed prompt.txt
var systemPrompt string

// Describe sends all readable images in folderPath to Gemini Vision and returns
// a validated Item plus the basenames of photos that were sent. The folder name
// hint is forwarded to the model as a labelling cue. The genai client is reused
// across calls — create it once in main, pass it in here.
func Describe(ctx context.Context, client *genai.Client, folderPath, hint string, cfg Config) (Item, []string, error) {
	imagePaths, err := listImages(folderPath)
	if err != nil {
		return Item{}, nil, fmt.Errorf("list images: %w", err)
	}

	parts := []*genai.Part{
		genai.NewPartFromText("Folder name hint: " + hint),
	}
	var photoNames []string
	for _, p := range imagePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			slog.Warn("skipping image, read failed", "path", p, "error", err)
			continue
		}
		mime, ok := detectMIME(data)
		if !ok {
			slog.Warn("skipping image, unsupported format", "path", p)
			continue
		}
		parts = append(parts, genai.NewPartFromBytes(data, mime))
		photoNames = append(photoNames, filepath.Base(p))
	}
	if len(photoNames) == 0 {
		return Item{}, nil, fmt.Errorf("no usable images in %s", folderPath)
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction:  genai.NewContentFromText(systemPrompt, ""),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: itemJSONSchema(),
	}

	resp, err := client.Models.GenerateContent(ctx, cfg.GeminiModel, contents, config)
	if err != nil {
		return Item{}, nil, fmt.Errorf("generate content: %w", err)
	}

	raw := resp.Text()
	var item Item
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		snippet := raw
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		slog.Warn("invalid JSON from model", "raw", snippet)
		return Item{}, nil, fmt.Errorf("parse response: %w", err)
	}

	if err := validateItem(item); err != nil {
		slog.Warn("validation failed", "item", item)
		return Item{}, nil, fmt.Errorf("validate item: %w", err)
	}

	return item, photoNames, nil
}

// listImages returns the sorted absolute paths of *.jpg, *.jpeg, *.png files
// directly inside folderPath. Subdirectories are ignored.
func listImages(folderPath string) ([]string, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".jpg", ".jpeg", ".png":
			out = append(out, filepath.Join(folderPath, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// detectMIME returns ("image/jpeg" | "image/png", true) from magic bytes,
// or ("", false) for any other format. Extension is not consulted.
func detectMIME(data []byte) (string, bool) {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg", true
	}
	if len(data) >= 4 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png", true
	}
	return "", false
}

// validateItem enforces all spec-level constraints on a freshly parsed Item:
// title length (rune count) ≤ 70, category and condition in their closed sets,
// non-negative price estimate.
func validateItem(item Item) error {
	if len([]rune(item.TitleEN)) > 70 {
		return fmt.Errorf("title_en exceeds 70 chars")
	}
	if len([]rune(item.TitleDE)) > 70 {
		return fmt.Errorf("title_de exceeds 70 chars")
	}
	if !validCategories[item.Category] {
		return fmt.Errorf("invalid category %q", item.Category)
	}
	if !validConditions[item.Condition] {
		return fmt.Errorf("invalid condition %q", item.Condition)
	}
	if item.PriceEstimateEUR < 0 {
		return fmt.Errorf("price_estimate_eur is negative")
	}
	return nil
}

// itemJSONSchema describes the Item struct as a JSON Schema document for the
// model's structured-output mode. We use ResponseJsonSchema (not ResponseSchema)
// because the OpenAPI-shaped *genai.Schema cannot express the "arbitrary
// string->string map" needed for Attributes. JSON Schema's additionalProperties
// is the canonical way to describe that, and Gemini's response_json_schema
// subset explicitly supports it.
func itemJSONSchema() any {
	stringType := map[string]any{"type": "string"}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title_en": stringType,
			"title_de": stringType,
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
			"price_estimate_eur":           map[string]any{"type": "integer"},
			"attributes": map[string]any{
				"type":                 "object",
				"description":          "Category-relevant attribute keys (brand, size, isbn, material, etc.) mapped to string values.",
				"additionalProperties": stringType,
			},
			"ocr_notes": stringType,
		},
		"required": []string{
			"title_en", "title_de", "category", "condition", "flaws",
			"description_vinted_en", "description_vinted_de", "description_kleinanzeigen_de",
			"price_estimate_eur", "attributes", "ocr_notes",
		},
	}
}
