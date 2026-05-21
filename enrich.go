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

// Enrichment is the Stage 2 payload. Fields are best-effort — the model fills
// what it can confidently identify from web search and leaves the rest empty.
type Enrichment struct {
	ModelName  string   `json:"model_name"`
	DesignName string   `json:"design_name"`
	Features   []string `json:"features"`
}

// shouldEnrich reports whether item is a candidate for Stage 2. Enrichment only
// helps categories with a discoverable upstream product and a brand to ground on.
func shouldEnrich(item Item) bool {
	switch item.Category {
	case "shoes", "clothing", "electronics":
	default:
		return false
	}
	return !attributeEmpty(item.Attributes["brand"])
}

// Enrich runs Stage 2 — a Google Search-grounded text call — and folds the
// result into item. Grounding cannot be combined with structured output (the
// Gemini API rejects a tool call alongside response_mime_type application/json),
// so the model returns a JSON block inside free text that parseEnrichment
// recovers. Any failure is logged and the original item is returned unchanged:
// enrichment is purely additive.
func Enrich(ctx context.Context, gen contentGenerator, item Item, cfg Config) Item {
	if !cfg.EnableEnrichment || !shouldEnrich(item) {
		return item
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(
			[]*genai.Part{genai.NewPartFromText(buildEnrichQuery(item))},
			genai.RoleUser,
		),
	}
	config := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(enrichSystemPrompt, ""),
		Tools:             []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}},
		// No ResponseMIMEType / ResponseSchema here: the Gemini API rejects a
		// tool call combined with application/json structured output.
	}

	var resp *genai.GenerateContentResponse
	err := retryOnTransient(ctx, func() error {
		var callErr error
		resp, callErr = gen.GenerateContent(ctx, cfg.GeminiModel, contents, config)
		return callErr
	})
	if err != nil {
		slog.Warn("enrichment call failed", "error", err)
		return item
	}

	enr, err := parseEnrichment(resp.Text())
	if err != nil {
		slog.Warn("enrichment parse failed", "error", err)
		return item
	}
	return mergeEnrichment(item, enr, resp)
}

// buildEnrichQuery composes the Stage 2 user prompt from what Stage 1 already saw.
func buildEnrichQuery(item Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Identify this second-hand product so I can write a better marketplace listing.\n\n")
	fmt.Fprintf(&b, "Category: %s\n", item.Category)
	if v := strings.TrimSpace(item.Attributes["brand"]); v != "" {
		fmt.Fprintf(&b, "Brand: %s\n", v)
	}
	for _, k := range []string{"size_eu", "size", "color", "material", "fastener", "sole_type"} {
		if v := strings.TrimSpace(item.Attributes[k]); v != "" && !strings.EqualFold(v, "unknown") {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	if item.TitleDE != "" {
		fmt.Fprintf(&b, "Best guess title (DE): %s\n", item.TitleDE)
	}
	fmt.Fprintf(&b, "\nSearch the web (otto.de, the brand's own site, Amazon DE) for the exact product. ")
	fmt.Fprintf(&b, "Return the model name, the design/colour name (often distinct from the colour), ")
	fmt.Fprintf(&b, "and up to 5 named features that would help the listing sell.\n")
	return b.String()
}

// jsonBlockRE finds the first brace-delimited block containing a model_name key.
// The grounded model cannot use structured output, so its reply is free text
// that may carry prose around the JSON object.
var jsonBlockRE = regexp.MustCompile(`(?s)\{[^{}]*"model_name"[^{}]*\}`)

// parseEnrichment extracts an Enrichment from a possibly-prose-wrapped response.
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

// mergeEnrichment folds enr into item. Stage 1 (photo-grounded) wins over Stage 2
// (web-grounded) when both have a real value for the same attribute. Grounding
// source URIs are appended to ocr_notes so the seller can spot-check provenance.
func mergeEnrichment(item Item, enr Enrichment, resp *genai.GenerateContentResponse) Item {
	if item.Attributes == nil {
		item.Attributes = map[string]string{}
	}
	if enr.ModelName != "" && attributeEmpty(item.Attributes["model_name"]) {
		item.Attributes["model_name"] = enr.ModelName
	}
	if enr.DesignName != "" && attributeEmpty(item.Attributes["design_name"]) {
		item.Attributes["design_name"] = enr.DesignName
	}
	if len(enr.Features) > 0 {
		joined := strings.Join(enr.Features, "; ")
		if existing := item.Attributes["features"]; existing != "" {
			item.Attributes["features"] = existing + "; " + joined
		} else {
			item.Attributes["features"] = joined
		}
	}
	if uris := groundingURIs(resp); len(uris) > 0 {
		note := "Enrichment sources: " + strings.Join(uris, ", ")
		if item.OCRNotes != "" {
			item.OCRNotes += "\n" + note
		} else {
			item.OCRNotes = note
		}
	}
	return item
}

// attributeEmpty reports whether an attribute value should be treated as absent.
// The vision prompt writes the literal "unknown" when a value is not visible, so
// "unknown" must count as empty or enrichment could never fill model_name.
func attributeEmpty(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || strings.EqualFold(v, "unknown")
}

// groundingURIs pulls citation URIs out of the response's grounding metadata.
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
