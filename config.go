package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration derived from environment variables and .env.
// Shell-set variables always win over .env values.
type Config struct {
	GeminiAPIKey          string
	InputDir              string
	OutputDir             string
	GeminiModel           string
	RequestTimeoutSeconds int
	EnableEnrichment      bool
}

// LoadConfig loads .env if present (shell env wins via godotenv.Load, not Overload)
// and returns a validated Config. An absent GEMINI_API_KEY is treated as fatal.
func LoadConfig() (Config, error) {
	_ = godotenv.Load()

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return Config{}, fmt.Errorf(
			"GEMINI_API_KEY not set. Get a free key at https://aistudio.google.com/apikey")
	}

	inputDir := os.Getenv("INPUT_DIR")
	if inputDir == "" {
		inputDir = "/mnt/d/Obsidian/market-vault/_inbox/RAW"
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}

	timeout := 180
	if s := os.Getenv("REQUEST_TIMEOUT_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			timeout = n
		}
	}

	enableEnrichment, _ := strconv.ParseBool(os.Getenv("ENABLE_ENRICHMENT"))

	return Config{
		GeminiAPIKey:          key,
		InputDir:              inputDir,
		OutputDir:             os.Getenv("OUTPUT_DIR"),
		GeminiModel:           model,
		RequestTimeoutSeconds: timeout,
		EnableEnrichment:      enableEnrichment,
	}, nil
}
