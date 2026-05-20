package main

// Item is the structured listing draft returned by the vision model.
// Category-specific fields go in Attributes so the same struct
// handles clothing, shoes, books, 3D-printed parts, household, and electronics.
type Item struct {
	TitleEN                    string            `json:"title_en"`
	TitleDE                    string            `json:"title_de"`
	TitleVintedDE              string            `json:"title_vinted_de"`
	Category                   string            `json:"category"`
	Condition                  string            `json:"condition"`
	Flaws                      []string          `json:"flaws"`
	DescriptionVintedEN        string            `json:"description_vinted_en"`
	DescriptionVintedDE        string            `json:"description_vinted_de"`
	DescriptionKleinanzeigenDE string            `json:"description_kleinanzeigen_de"`
	PriceMinEUR                int               `json:"price_min_eur"`
	PriceMaxEUR                int               `json:"price_max_eur"`
	Attributes                 map[string]string `json:"attributes"`
	OCRNotes                   string            `json:"ocr_notes"`
}

// validCategories is the closed set of category strings the vision model may return.
var validCategories = map[string]bool{
	"clothing":    true,
	"shoes":       true,
	"books":       true,
	"3d-printed":  true,
	"household":   true,
	"electronics": true,
	"other":       true,
}

// validConditions is the closed set of condition strings the vision model may return.
var validConditions = map[string]bool{
	"new-with-tags":  true,
	"used-excellent": true,
	"used-good":      true,
	"used-fair":      true,
}
