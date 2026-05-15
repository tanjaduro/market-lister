package main

import (
	"context"
	"errors"
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
