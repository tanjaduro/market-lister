# Test skeletons

Copy-pasteable shapes for the four recurring patterns. Drop in and adapt.

## 1. Env-isolated config test

```go
func TestSomethingThatReadsEnv(t *testing.T) {
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
}
```

Set every env var that `LoadConfig` reads, even if the test cares only about one. Don't let the host shell leak in.

## 2. Filesystem test

```go
func TestSomethingThatTouchesFiles(t *testing.T) {
    tmp := t.TempDir()
    folderPath := filepath.Join(tmp, "myfolder")
    if err := os.Mkdir(folderPath, 0o755); err != nil {
        t.Fatal(err)
    }
    if err := os.WriteFile(filepath.Join(folderPath, "file.txt"), []byte("x"), 0o644); err != nil {
        t.Fatal(err)
    }
    // ... call function under test, assert against tmp / folderPath ...
}
```

`t.TempDir()` is cleaned up automatically. Never hardcode `/tmp/...` or `.`.

## 3. Package-var mutation

```go
func TestUsingPackageVar(t *testing.T) {
    orig := retryDelays
    retryDelays = []time.Duration{0, 0}
    t.Cleanup(func() { retryDelays = orig })

    // test body that depends on the faster retry timing
}
```

Pattern from vision_test.go:16. The `t.Cleanup` runs even if the test panics — safer than `defer`.

## 4. Table-driven text-output assertion

```go
cases := []struct {
    name            string
    item            Item
    wantContains    []string
    wantNotContains []string
}{
    {
        name: "happy path",
        item: someItem,
        wantContains:    []string{`id: "..."`, `category: shoes`},
        wantNotContains: []string{"None visible."},
    },
}

for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        got := Render(tc.item, /* ... */)
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
```

Shape from markdown_test.go:67. Always include the full output in the error message — substring failures are unreadable without it.
