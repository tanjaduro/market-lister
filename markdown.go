package main

import (
	"bytes"
	"text/template"
	"time"
)

// tmplData is the data bag passed to the listing template.
type tmplData struct {
	Date       string // YYYY-MM-DD
	Slug       string
	FolderName string
	PhotoBase  string // "" when .md is co-located with photos; absolute path + "/" otherwise
	Photos     []string
	Item       Item
}

// listingTmpl is parsed once at package load. Execution failure is a programmer
// error since the template is a const.
var listingTmpl = template.Must(template.New("listing").Parse(tmplRaw))

const tmplRaw = `---
id: "{{.Date}}-{{.Slug}}"
status: draft
platforms: [vinted, kleinanzeigen]
category: {{.Item.Category}}
condition: {{.Item.Condition}}
price_min_eur: {{.Item.PriceMinEUR}}
price_max_eur: {{.Item.PriceMaxEUR}}
source_folder: "{{.FolderName}}"
date_added: {{.Date}}
attributes:
{{- range $k, $v := .Item.Attributes }}
  {{$k}}: "{{$v}}"
{{- end }}
flaws:
{{- if .Item.Flaws }}
{{- range .Item.Flaws }}
  - "{{.}}"
{{- end }}
{{- else }} []
{{- end }}
---

# {{.Item.TitleEN}}

## Vinted (English)

{{.Item.DescriptionVintedEN}}

## Vinted (Deutsch)

{{.Item.DescriptionVintedDE}}

## Kleinanzeigen (Deutsch)

{{.Item.DescriptionKleinanzeigenDE}}

## Details

- Category: {{.Item.Category}}
- Condition: {{.Item.Condition}}
- Price: {{.Item.PriceMinEUR}}-{{.Item.PriceMaxEUR}} EUR
{{- range $k, $v := .Item.Attributes }}
- {{$k}}: {{$v}}
{{- end }}

## Flaws

{{- if .Item.Flaws }}
{{- range .Item.Flaws }}
- {{.}}
{{- end }}
{{- else }}
None visible.
{{- end }}

## Checklist

- [ ] Verify key details on labels / packaging
- [ ] Set final price
- [ ] Choose pickup Panketal or shipping
{{- if .Item.OCRNotes }}

## OCR notes (verify these)

{{.Item.OCRNotes}}
{{- end }}

## Photos

{{- range .Photos }}
![](<{{$.PhotoBase}}{{.}}>)
{{- end }}
`

// Render produces the complete markdown listing for one item.
// Pure function: no I/O, all path logic resolved by the caller.
func Render(item Item, slug, folderName string, photos []string, photoBase string, today time.Time) string {
	var buf bytes.Buffer
	if err := listingTmpl.Execute(&buf, tmplData{
		Date:       today.Format("2006-01-02"),
		Slug:       slug,
		FolderName: folderName,
		PhotoBase:  photoBase,
		Photos:     photos,
		Item:       item,
	}); err != nil {
		panic(err)
	}
	return buf.String()
}
