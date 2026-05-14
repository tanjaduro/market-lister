package main

import (
	"strings"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	fixedDate := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)

	reimaItem := Item{
		TitleEN:  "Reima pink kids sneakers, EU 24 / US 8 (16 cm)",
		TitleDE:  "Reima rosa Kinder-Sneaker, Gr. 24 / US 8 (16 cm)",
		Category: "shoes",
		Condition: "used-good",
		Flaws: []string{
			"white midsole scuffed and yellowed at toe area",
			"moderate outsole tread wear",
		},
		DescriptionVintedEN:        "Reima kids sneakers in pink, size EU 24 (US 8 / 16 cm insole). Velcro closure, mesh upper, reflective heel pull tab. Good used condition — white midsole is scuffed at the toes and slightly yellowed. From a smoke-free home.",
		DescriptionVintedDE:        "Reima Kinder-Sneaker in Rosa, Größe EU 24 (US 8 / 16 cm Innensohle). Klettverschluss, Mesh-Obermaterial, reflektierende Lasche an der Ferse. Guter gebrauchter Zustand — weiße Sohle an den Zehen abgerieben und leicht vergilbt. Aus tierfreiem Nichtraucherhaushalt.",
		DescriptionKleinanzeigenDE: "Verkaufe Reima Kinder-Sneaker in Rosa, Größe 24 (US 8 / 16 cm Innensohle). Klettverschluss und atmungsaktives Mesh-Obermaterial. Reflektierende Lasche an der Ferse für Sichtbarkeit. Guter gebrauchter Zustand — weiße Sohle an den Zehen abgerieben und leicht vergilbt, Profil moderat abgenutzt. Aus tierfreiem Nichtraucherhaushalt. Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme.",
		PriceEstimateEUR:           10,
		Attributes: map[string]string{
			"brand":    "Reima",
			"size_eu":  "24",
			"size_us":  "8",
			"size_cm":  "16.00",
			"material": "mesh upper, rubber outsole",
			"color":    "pink",
		},
		OCRNotes: "Insole text reads: reima 24 / US 8 / CM 16.00 / CN170(1.5)",
	}

	zaraItem := Item{
		TitleEN:             "Zara floral dress, kids size 104",
		TitleDE:             "Zara Blumenkleid, Gr. 104",
		Category:            "clothing",
		Condition:           "used-excellent",
		Flaws:               nil,
		DescriptionVintedEN: "Zara floral dress, size 104. Excellent condition, no visible flaws. From a smoke-free home.",
		DescriptionVintedDE: "Zara Blumenkleid, Größe 104. Sehr guter Zustand, keine sichtbaren Mängel. Aus tierfreiem Nichtraucherhaushalt.",
		DescriptionKleinanzeigenDE: "Zara Blumenkleid, Größe 104. Sehr guter Zustand, keine sichtbaren Mängel. Aus tierfreiem Nichtraucherhaushalt. Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme.",
		PriceEstimateEUR:           8,
		Attributes: map[string]string{
			"brand":     "Zara",
			"size":      "104",
			"age_range": "3-4 years",
		},
		OCRNotes: "",
	}

	bookItem := Item{
		TitleEN:                    "Stephen King — It (paperback)",
		TitleDE:                    "Stephen King — Es (Taschenbuch)",
		Category:                   "books",
		Condition:                  "used-good",
		Flaws:                      []string{"corner of front cover bent"},
		DescriptionVintedEN:        "Stephen King — It. Good used condition.",
		DescriptionVintedDE:        "Stephen King — Es. Guter gebrauchter Zustand.",
		DescriptionKleinanzeigenDE: "Stephen King — Es. Guter gebrauchter Zustand. Abholung in Panketal oder Versand gegen Aufpreis möglich. Privatverkauf — keine Garantie oder Rücknahme.",
		PriceEstimateEUR:           4,
		Attributes:                 map[string]string{"author": "Stephen King", "language": "English"},
	}

	cases := []struct {
		name            string
		item            Item
		slug            string
		folderName      string
		photos          []string
		photoBase       string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:       "reima shoes — spec example (attributes, flaws, OCR notes)",
			item:       reimaItem,
			slug:       "reima-pink-24",
			folderName: "reima pink 24",
			photos:     []string{"20260513_101843.jpg", "20260513_101900.jpg"},
			photoBase:  "",
			wantContains: []string{
				`id: "2026-05-14-reima-pink-24"`,
				`category: shoes`,
				`condition: used-good`,
				`price_estimate_eur: 10`,
				`source_folder: "reima pink 24"`,
				`brand: "Reima"`,
				`size_eu: "24"`,
				`size_cm: "16.00"`,
				`material: "mesh upper, rubber outsole"`,
				`color: "pink"`,
				`- "white midsole scuffed and yellowed at toe area"`,
				`- "moderate outsole tread wear"`,
				`## OCR notes (verify these)`,
				`CN170(1.5)`,
				`![](<20260513_101843.jpg>)`,
				`![](<20260513_101900.jpg>)`,
				`Abholung in Panketal oder Versand gegen Aufpreis möglich.`,
			},
			wantNotContains: []string{"None visible."},
		},
		{
			name:       "no flaws — flaws renders as [], no OCR notes section",
			item:       zaraItem,
			slug:       "zara-dress-floral-104",
			folderName: "zara dress floral 104",
			photos:     []string{"front.jpg"},
			photoBase:  "",
			wantContains: []string{
				`flaws: []`,
				`None visible.`,
				`![](<front.jpg>)`,
				`age_range: "3-4 years"`,
			},
			wantNotContains: []string{
				`## OCR notes (verify these)`,
				`  - "`, // no flaw bullet entries
			},
		},
		{
			name:       "OUTPUT_DIR set — absolute path with spaces, angle-bracket escaped",
			item:       bookItem,
			slug:       "book-stephen-king-it",
			folderName: "book stephen king it",
			photos:     []string{"cover.jpg", "spine.jpg"},
			photoBase:  "/mnt/d/Obsidian/market-vault/_inbox/RAW/book stephen king it/",
			wantContains: []string{
				`![](</mnt/d/Obsidian/market-vault/_inbox/RAW/book stephen king it/cover.jpg>)`,
				`![](</mnt/d/Obsidian/market-vault/_inbox/RAW/book stephen king it/spine.jpg>)`,
				`- "corner of front cover bent"`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.item, tc.slug, tc.folderName, tc.photos, tc.photoBase, fixedDate)
			for _, w := range tc.wantContains {
				if !strings.Contains(got, w) {
					t.Errorf("missing expected substring %q\n--- got ---\n%s", w, got)
				}
			}
			for _, w := range tc.wantNotContains {
				if strings.Contains(got, w) {
					t.Errorf("unexpected substring %q present\n--- got ---\n%s", w, got)
				}
			}
		})
	}
}
