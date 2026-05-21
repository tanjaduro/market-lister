package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"google.golang.org/genai"
)

// result is the per-folder outcome tallied by main.
type result int

const (
	resultDone result = iota
	resultSkipped
	resultFailed
)

func main() {
	folderFlag := flag.String("folder", "", "process a single named subfolder")
	outputFlag := flag.String("output", "", "write .md files here instead of source folder")
	dryRun := flag.Bool("dry-run", false, "list folders without calling the API")
	enrichFlag := flag.Bool("enrich", false, "enable Stage 2 product enrichment via web search")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cfg, err := LoadConfig()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if *outputFlag != "" {
		cfg.OutputDir = *outputFlag
	}
	if *enrichFlag {
		cfg.EnableEnrichment = true
	}

	if _, err := os.Stat(cfg.InputDir); err != nil {
		slog.Error("INPUT_DIR not accessible", "path", cfg.InputDir, "error", err)
		os.Exit(1)
	}
	if cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
			slog.Error("cannot create OUTPUT_DIR", "path", cfg.OutputDir, "error", err)
			os.Exit(1)
		}
	}

	folders, err := collectFolders(cfg.InputDir, *folderFlag)
	if err != nil {
		slog.Error("cannot scan input dir", "error", err)
		os.Exit(1)
	}

	// Build the Gemini client once. Reused across all folders so HTTP/2 connections
	// and credential setup are amortised across the batch.
	var client *genai.Client
	if !*dryRun {
		client, err = genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey:  cfg.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			slog.Error("cannot create genai client", "error", err)
			os.Exit(1)
		}
	}

	var done, skipped, failed int
	for _, folder := range folders {
		if *dryRun {
			slog.Info("dry-run", "folder", filepath.Base(folder))
			continue
		}
		switch processFolder(cfg, client.Models, folder) {
		case resultDone:
			done++
		case resultSkipped:
			skipped++
		case resultFailed:
			failed++
		}
	}

	if !*dryRun {
		slog.Info("summary", "done", done, "skipped", skipped, "failed", failed)
	}
}

// collectFolders returns the absolute paths of item subfolders inside inputDir.
// When singleName is non-empty, only the matching subfolder is returned.
func collectFolders(inputDir, singleName string) ([]string, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}
	var folders []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if singleName != "" && e.Name() != singleName {
			continue
		}
		folders = append(folders, filepath.Join(inputDir, e.Name()))
	}
	return folders, nil
}

// processFolder runs one folder end-to-end and returns its outcome.
// Failures are logged and counted by main; never call os.Exit here.
func processFolder(cfg Config, gen contentGenerator, folderPath string) result {
	hint := filepath.Base(folderPath)
	slug := slugify(hint)
	if slug == "" {
		slog.Warn("skipping, slug empty after sanitization", "folder", hint)
		return resultSkipped
	}
	outputPath := outputMDPath(cfg, folderPath, slug)

	// Only proceed when the output path is confirmed absent. On any other Stat
	// error (permission denied, EIO, stale handle, etc.) we skip the item rather
	// than risk clobbering an existing file we couldn't observe.
	if _, err := os.Stat(outputPath); err == nil {
		slog.Warn("skipping, already processed", "folder", hint, "path", outputPath)
		return resultSkipped
	} else if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("skipping, cannot stat output path", "folder", hint, "path", outputPath, "error", err)
		return resultSkipped
	}

	today := time.Now()
	slog.Info("processing", "folder", hint)

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.RequestTimeoutSeconds)*time.Second)
	defer cancel()

	item, photos, err := Describe(ctx, gen, folderPath, hint, cfg)
	if err != nil {
		slog.Error("vision failed", "folder", hint, "duration_ms", time.Since(today).Milliseconds(), "error", err)
		return resultFailed
	}

	// Stage 2 gets a fresh context so a slow or retrying Describe cannot starve
	// enrichment of its deadline. Derived from Background, not ctx, by design.
	enrichCtx, enrichCancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.EnrichTimeoutSeconds)*time.Second)
	defer enrichCancel()

	item = Enrich(enrichCtx, gen, item, cfg)

	photoBase := ""
	if cfg.OutputDir != "" {
		photoBase = folderPath + string(filepath.Separator)
	}

	md := Render(item, slug, hint, photos, photoBase, today)

	if err := os.WriteFile(outputPath, []byte(md), 0o644); err != nil {
		slog.Error("write failed", "folder", hint, "path", outputPath, "duration_ms", time.Since(today).Milliseconds(), "error", err)
		return resultFailed
	}
	slog.Info("done", "folder", hint, "duration_ms", time.Since(today).Milliseconds())
	return resultDone
}

// outputMDPath returns the absolute path where the listing markdown will be written.
func outputMDPath(cfg Config, folderPath, slug string) string {
	if cfg.OutputDir != "" {
		return filepath.Join(cfg.OutputDir, slug+".md")
	}
	return filepath.Join(folderPath, slug+".md")
}

var (
	reNonAlnum  = regexp.MustCompile(`[^a-z0-9-]+`)
	reMultiDash = regexp.MustCompile(`-{2,}`)
)

// slugify converts a folder name to a lowercase ASCII hyphen-separated slug.
// Spaces and underscores become hyphens; characters outside [a-z0-9-] are dropped
// (non-ASCII letters are not transliterated). Multiple consecutive hyphens collapse.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ' || r == '_':
			b.WriteRune('-')
		case r < unicode.MaxASCII:
			b.WriteRune(r)
		}
	}
	out := reNonAlnum.ReplaceAllString(b.String(), "")
	out = reMultiDash.ReplaceAllString(out, "-")
	return strings.Trim(out, "-")
}
