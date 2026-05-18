package main

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirAway moves the test out of the project directory so godotenv.Load() in
// LoadConfig is a no-op (no .env present in t.TempDir). Combined with t.Setenv,
// this makes the test deterministic regardless of the developer's real .env.
func chdirAway(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	chdirAway(t)
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := LoadConfig(); err == nil {
		t.Error("expected error when GEMINI_API_KEY is empty")
	}
}

func TestLoadConfig_DefaultsApplied(t *testing.T) {
	chdirAway(t)
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("INPUT_DIR", "")
	t.Setenv("OUTPUT_DIR", "")
	t.Setenv("GEMINI_MODEL", "")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GeminiAPIKey != "test-key" {
		t.Errorf("GeminiAPIKey = %q, want %q", cfg.GeminiAPIKey, "test-key")
	}
	if cfg.InputDir != "/mnt/d/Obsidian/market-vault/_inbox/RAW" {
		t.Errorf("InputDir = %q, want default", cfg.InputDir)
	}
	if cfg.GeminiModel != "gemini-2.5-flash" {
		t.Errorf("GeminiModel = %q, want default", cfg.GeminiModel)
	}
	if cfg.RequestTimeoutSeconds != 180 {
		t.Errorf("RequestTimeoutSeconds = %d, want 180", cfg.RequestTimeoutSeconds)
	}
	if cfg.OutputDir != "" {
		t.Errorf("OutputDir = %q, want empty", cfg.OutputDir)
	}
}

func TestLoadConfig_AllEnvRespected(t *testing.T) {
	chdirAway(t)
	t.Setenv("GEMINI_API_KEY", "k2")
	t.Setenv("INPUT_DIR", "/custom/in")
	t.Setenv("OUTPUT_DIR", "/custom/out")
	t.Setenv("GEMINI_MODEL", "gemini-1.5-pro")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "60")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		GeminiAPIKey:          "k2",
		InputDir:              "/custom/in",
		OutputDir:             "/custom/out",
		GeminiModel:           "gemini-1.5-pro",
		RequestTimeoutSeconds: 60,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

// TestLoadConfig_ShellEnvWinsOverDotEnv confirms the contract in config.go:21:
// when GEMINI_API_KEY is set in both the shell and a .env in the working
// directory, the shell value wins. godotenv.Load (not Overload) is the
// mechanism — verify that mechanism stays in place.
func TestLoadConfig_ShellEnvWinsOverDotEnv(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("GEMINI_API_KEY=from-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmp)
	t.Setenv("GEMINI_API_KEY", "from-shell")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GeminiAPIKey != "from-shell" {
		t.Errorf("GeminiAPIKey = %q, want %q (shell must win over .env)", cfg.GeminiAPIKey, "from-shell")
	}
}

func TestLoadConfig_TimeoutFallbacks(t *testing.T) {
	cases := []struct {
		env  string
		want int
	}{
		{"abc", 180},
		{"0", 180},
		{"-5", 180},
		{"30", 30},
	}
	for _, tc := range cases {
		t.Run("env="+tc.env, func(t *testing.T) {
			chdirAway(t)
			t.Setenv("GEMINI_API_KEY", "k")
			t.Setenv("INPUT_DIR", "")
			t.Setenv("OUTPUT_DIR", "")
			t.Setenv("GEMINI_MODEL", "")
			t.Setenv("REQUEST_TIMEOUT_SECONDS", tc.env)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.RequestTimeoutSeconds != tc.want {
				t.Errorf("env=%q: got %d, want %d", tc.env, cfg.RequestTimeoutSeconds, tc.want)
			}
		})
	}
}
