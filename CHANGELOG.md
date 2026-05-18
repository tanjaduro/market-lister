# Changelog

All notable changes are recorded here. Bump policy and rationale live in NEXT-STEPS.md §7.2.

## [Unreleased]

### Added
- README troubleshooting section covering `context deadline exceeded`.
- `duration_ms` on `vision failed` and `write failed` log entries (previously only on the `done` line).
- 12 additional `slugify` test cases for empty input, emoji, accents, leading/trailing hyphens, double underscores, and filename-style dots.

### Changed
- Default `REQUEST_TIMEOUT_SECONDS` raised from 120 to 180.
- Kleinanzeigen disclaimer enforcement now requires the disclaimer at the end of the description (`HasSuffix` over the trimmed string), not merely present.
- Prompt enums (`category`, `condition`) and the 70-char title cap moved into a dedicated "Field constraints" section so the numbered items flow uninterrupted.

## [0.1.0] — 2026-05-17

### Added
- Initial release: process photo folders through Gemini Vision and emit Markdown listings.
- Seven supported categories: clothing, shoes, books, 3d-printed, household, electronics, other.
- Bilingual EN/DE listing text plus Kleinanzeigen-specific German variant with the required private-sale disclaimer.
- Frontmatter metadata (`id`, `status`, `category`, `condition`, `price_estimate_eur`, attributes, flaws).
- Folder-name slug as filename; idempotent skip when output already exists.
- Retry with backoff on transient Gemini errors (429 / 503 / UNAVAILABLE / RESOURCE_EXHAUSTED).
- CLI flags: `-folder`, `-output`, `-dry-run`.
- GitHub Actions CI: `go vet`, `go test`, `govulncheck`.
