package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetryOnTransient(t *testing.T) {
	orig := retryDelays
	retryDelays = []time.Duration{0, 0}
	t.Cleanup(func() { retryDelays = orig })

	t.Run("retries on 503 then succeeds", func(t *testing.T) {
		calls := 0
		err := retryOnTransient(context.Background(), func() error {
			calls++
			if calls < 2 {
				return errors.New("googleapi: Error 503: model overloaded, UNAVAILABLE")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("expected success after retry, got %v", err)
		}
		if calls != 2 {
			t.Fatalf("expected 2 calls, got %d", calls)
		}
	})

	t.Run("does not retry on 400", func(t *testing.T) {
		calls := 0
		want := errors.New("googleapi: Error 400: bad request, INVALID_ARGUMENT")
		err := retryOnTransient(context.Background(), func() error {
			calls++
			return want
		})
		if err != want {
			t.Fatalf("expected error %v, got %v", want, err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call (no retry), got %d", calls)
		}
	})

	t.Run("returns last error after exhausting attempts", func(t *testing.T) {
		calls := 0
		errs := []error{
			errors.New("attempt 1: 503 UNAVAILABLE"),
			errors.New("attempt 2: 503 UNAVAILABLE"),
			errors.New("attempt 3: 503 UNAVAILABLE"),
		}
		err := retryOnTransient(context.Background(), func() error {
			e := errs[calls]
			calls++
			return e
		})
		if calls != 3 {
			t.Fatalf("expected 3 calls (1 + 2 retries), got %d", calls)
		}
		if err == nil || err.Error() != "attempt 3: 503 UNAVAILABLE" {
			t.Fatalf("expected last error returned, got %v", err)
		}
	})

	t.Run("returns ctx.Err when cancelled during backoff", func(t *testing.T) {
		// Override with a non-zero delay so the cancellation window is observable.
		prev := retryDelays
		retryDelays = []time.Duration{time.Hour, time.Hour}
		t.Cleanup(func() { retryDelays = prev })

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		calls := 0
		start := time.Now()
		err := retryOnTransient(ctx, func() error {
			calls++
			return errors.New("503 UNAVAILABLE")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call before cancellation, got %d", calls)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("backoff did not abort on cancel; elapsed %v", elapsed)
		}
	})
}

func TestValidateItem(t *testing.T) {
	valid := Item{
		TitleEN:          "ok",
		TitleDE:          "ok",
		Category:         "books",
		Condition:        "used-good",
		PriceEstimateEUR: 5,
	}
	mutate := func(f func(*Item)) Item {
		i := valid
		f(&i)
		return i
	}

	cases := []struct {
		name    string
		item    Item
		wantErr bool
	}{
		{"valid item", valid, false},
		{"zero-value item (empty category)", Item{}, true},
		{"title_en exactly 70 ASCII chars", mutate(func(i *Item) { i.TitleEN = strings.Repeat("a", 70) }), false},
		{"title_en 71 ASCII chars", mutate(func(i *Item) { i.TitleEN = strings.Repeat("a", 71) }), true},
		{"title_en 70 multibyte runes (bytes > 70)", mutate(func(i *Item) { i.TitleEN = strings.Repeat("ä", 70) }), false},
		{"title_en 71 multibyte runes", mutate(func(i *Item) { i.TitleEN = strings.Repeat("ä", 71) }), true},
		{"title_de 71 chars", mutate(func(i *Item) { i.TitleDE = strings.Repeat("b", 71) }), true},
		{"invalid category", mutate(func(i *Item) { i.Category = "bogus" }), true},
		{"invalid condition", mutate(func(i *Item) { i.Condition = "almost-new" }), true},
		{"negative price", mutate(func(i *Item) { i.PriceEstimateEUR = -1 }), true},
		{"zero price ok", mutate(func(i *Item) { i.PriceEstimateEUR = 0 }), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateItem(tc.item)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateItem err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestDetectMIME(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		wantMIME string
		wantOK   bool
	}{
		{"jpeg magic", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, "image/jpeg", true},
		{"png magic", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A}, "image/png", true},
		{"gif magic rejected", []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, "", false},
		{"webp magic rejected", []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}, "", false},
		{"bmp magic rejected", []byte{0x42, 0x4D, 0x00, 0x00}, "", false},
		{"jpeg truncated to 2 bytes", []byte{0xFF, 0xD8}, "", false},
		{"png truncated to 3 bytes", []byte{0x89, 0x50, 0x4E}, "", false},
		{"empty slice", []byte{}, "", false},
		{"nil slice", nil, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMIME, gotOK := detectMIME(tc.data)
			if gotMIME != tc.wantMIME || gotOK != tc.wantOK {
				t.Errorf("detectMIME = (%q, %v), want (%q, %v)", gotMIME, gotOK, tc.wantMIME, tc.wantOK)
			}
		})
	}
}

func TestListImages(t *testing.T) {
	tmp := t.TempDir()

	files := []string{"b.jpg", "a.PNG", "c.JPEG", "d.txt", "e.gif", "f.jpg"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmp, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	subdir := filepath.Join(tmp, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "inside.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := listImages(tmp)
	if err != nil {
		t.Fatalf("listImages: %v", err)
	}

	want := []string{
		filepath.Join(tmp, "a.PNG"),
		filepath.Join(tmp, "b.jpg"),
		filepath.Join(tmp, "c.JPEG"),
		filepath.Join(tmp, "f.jpg"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d\ngot=%v\nwant=%v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsRetryableError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain non-retryable", errors.New("boom"), false},
		{"503 marker", errors.New("503 Service Unavailable"), true},
		{"429 marker", errors.New("429 Too Many Requests"), true},
		{"UNAVAILABLE marker", errors.New("rpc error: code = UNAVAILABLE"), true},
		{"RESOURCE_EXHAUSTED marker", errors.New("RESOURCE_EXHAUSTED: quota"), true},
		{"lowercase unavailable not matched", errors.New("unavailable"), false},
		{"400 not retryable", errors.New("googleapi: Error 400: bad request"), false},
		{"wrapped 503 still matched", fmt.Errorf("call failed: %w", errors.New("got 503")), true},
		{"wrapped non-retryable", fmt.Errorf("wrap: %w", errors.New("400 bad")), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryableError(tc.err)
			if got != tc.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
