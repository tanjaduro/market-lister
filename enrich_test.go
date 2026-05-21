package main

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/genai"
)

// fakeGenerator is a contentGenerator stub for enrichment tests.
type fakeGenerator struct {
	resp  *genai.GenerateContentResponse
	err   error
	calls int
}

func (f *fakeGenerator) GenerateContent(_ context.Context, _ string, _ []*genai.Content, _ *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	f.calls++
	return f.resp, f.err
}

// textResponse builds a minimal response whose Text() yields s.
func textResponse(s string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{Content: &genai.Content{Parts: []*genai.Part{{Text: s}}}},
		},
	}
}

func TestShouldEnrich(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want bool
	}{
		{"shoes with brand", Item{Category: "shoes", Attributes: map[string]string{"brand": "Affenzahn"}}, true},
		{"clothing with brand", Item{Category: "clothing", Attributes: map[string]string{"brand": "Zara"}}, true},
		{"electronics with brand", Item{Category: "electronics", Attributes: map[string]string{"brand": "Samsung"}}, true},
		{"books excluded", Item{Category: "books", Attributes: map[string]string{"brand": "Penguin"}}, false},
		{"household excluded", Item{Category: "household", Attributes: map[string]string{"brand": "Ikea"}}, false},
		{"3d-printed excluded", Item{Category: "3d-printed", Attributes: map[string]string{"brand": "x"}}, false},
		{"other excluded", Item{Category: "other", Attributes: map[string]string{"brand": "x"}}, false},
		{"shoes no brand key", Item{Category: "shoes", Attributes: map[string]string{}}, false},
		{"shoes brand unknown", Item{Category: "shoes", Attributes: map[string]string{"brand": "unknown"}}, false},
		{"shoes brand Unknown mixed case", Item{Category: "shoes", Attributes: map[string]string{"brand": "Unknown"}}, false},
		{"shoes nil attributes", Item{Category: "shoes"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldEnrich(tc.item); got != tc.want {
				t.Errorf("shouldEnrich = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseEnrichment(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantErr    bool
		wantModel  string
		wantDesign string
		wantFeats  int
	}{
		{
			name:       "clean json",
			raw:        `{"model_name":"Knit Happy Bear","design_name":"Papagei","features":["Klett","Reflektoren"]}`,
			wantModel:  "Knit Happy Bear",
			wantDesign: "Papagei",
			wantFeats:  2,
		},
		{
			name:      "json wrapped in prose",
			raw:       "Here is what I found:\n{\"model_name\":\"Lucky Bird\",\"design_name\":\"\",\"features\":[]}\nHope that helps.",
			wantModel: "Lucky Bird",
			wantFeats: 0,
		},
		{name: "no json block", raw: "I could not identify the product.", wantErr: true},
		{name: "malformed json", raw: `{"model_name": not-valid}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enr, err := parseEnrichment(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if enr.ModelName != tc.wantModel {
				t.Errorf("ModelName = %q, want %q", enr.ModelName, tc.wantModel)
			}
			if enr.DesignName != tc.wantDesign {
				t.Errorf("DesignName = %q, want %q", enr.DesignName, tc.wantDesign)
			}
			if len(enr.Features) != tc.wantFeats {
				t.Errorf("len(Features) = %d, want %d", len(enr.Features), tc.wantFeats)
			}
		})
	}
}

func TestMergeEnrichment(t *testing.T) {
	t.Run("fills empty and unknown attributes", func(t *testing.T) {
		item := Item{Attributes: map[string]string{"brand": "Affenzahn", "model_name": "unknown"}}
		enr := Enrichment{ModelName: "Knit Happy Bear", DesignName: "Papagei", Features: []string{"Klett"}}
		got := mergeEnrichment(item, enr, nil)
		if got.Attributes["model_name"] != "Knit Happy Bear" {
			t.Errorf("model_name = %q, want enriched value to overwrite \"unknown\"", got.Attributes["model_name"])
		}
		if got.Attributes["design_name"] != "Papagei" {
			t.Errorf("design_name = %q, want Papagei", got.Attributes["design_name"])
		}
		if got.Attributes["features"] != "Klett" {
			t.Errorf("features = %q, want Klett", got.Attributes["features"])
		}
	})
	t.Run("stage 1 real value wins over enrichment", func(t *testing.T) {
		item := Item{Attributes: map[string]string{"model_name": "Real Model"}}
		got := mergeEnrichment(item, Enrichment{ModelName: "Web Model"}, nil)
		if got.Attributes["model_name"] != "Real Model" {
			t.Errorf("model_name = %q, want Stage 1 value 'Real Model'", got.Attributes["model_name"])
		}
	})
	t.Run("features append to existing", func(t *testing.T) {
		item := Item{Attributes: map[string]string{"features": "Mesh"}}
		got := mergeEnrichment(item, Enrichment{Features: []string{"Klett", "Reflektoren"}}, nil)
		if got.Attributes["features"] != "Mesh; Klett; Reflektoren" {
			t.Errorf("features = %q, want 'Mesh; Klett; Reflektoren'", got.Attributes["features"])
		}
	})
	t.Run("nil attributes map is created", func(t *testing.T) {
		got := mergeEnrichment(Item{}, Enrichment{ModelName: "X"}, nil)
		if got.Attributes["model_name"] != "X" {
			t.Errorf("model_name = %q, want X", got.Attributes["model_name"])
		}
	})
}

func TestEnrich_DisabledReturnsItemUnchanged(t *testing.T) {
	fake := &fakeGenerator{}
	item := Item{Category: "shoes", Attributes: map[string]string{"brand": "Affenzahn"}}
	got := Enrich(context.Background(), fake, item, Config{EnableEnrichment: false})
	if fake.calls != 0 {
		t.Errorf("generator called %d times, want 0 when enrichment disabled", fake.calls)
	}
	if got.Attributes["model_name"] != "" {
		t.Error("item mutated while enrichment disabled")
	}
}

func TestEnrich_SkipsUnenrichableCategory(t *testing.T) {
	fake := &fakeGenerator{}
	item := Item{Category: "books", Attributes: map[string]string{"brand": "Penguin"}}
	Enrich(context.Background(), fake, item, Config{EnableEnrichment: true})
	if fake.calls != 0 {
		t.Errorf("generator called %d times, want 0 for books category", fake.calls)
	}
}

func TestEnrich_HappyPath(t *testing.T) {
	fake := &fakeGenerator{
		resp: textResponse(`{"model_name":"Knit Happy Bear","design_name":"Papagei","features":["Klettverschluss"]}`),
	}
	item := Item{Category: "shoes", Attributes: map[string]string{"brand": "Affenzahn"}}
	got := Enrich(context.Background(), fake, item, Config{EnableEnrichment: true, GeminiModel: "gemini-2.5-flash"})
	if fake.calls != 1 {
		t.Fatalf("generator called %d times, want 1", fake.calls)
	}
	if got.Attributes["model_name"] != "Knit Happy Bear" {
		t.Errorf("model_name = %q, want Knit Happy Bear", got.Attributes["model_name"])
	}
	if got.Attributes["design_name"] != "Papagei" {
		t.Errorf("design_name = %q, want Papagei", got.Attributes["design_name"])
	}
}

func TestEnrich_CallFailureReturnsItemUnchanged(t *testing.T) {
	fake := &fakeGenerator{err: errors.New("network down")}
	item := Item{Category: "shoes", Attributes: map[string]string{"brand": "Affenzahn"}}
	got := Enrich(context.Background(), fake, item, Config{EnableEnrichment: true, GeminiModel: "x"})
	if got.Attributes["model_name"] != "" {
		t.Error("item should be unchanged when the enrichment call fails")
	}
}

func TestEnrich_UnparseableResponseReturnsItemUnchanged(t *testing.T) {
	fake := &fakeGenerator{resp: textResponse("I could not find this product anywhere.")}
	item := Item{Category: "shoes", Attributes: map[string]string{"brand": "Affenzahn"}}
	got := Enrich(context.Background(), fake, item, Config{EnableEnrichment: true, GeminiModel: "x"})
	if fake.calls != 1 {
		t.Fatalf("generator called %d times, want 1", fake.calls)
	}
	if got.Attributes["model_name"] != "" {
		t.Error("item should be unchanged when the response has no JSON block")
	}
}
