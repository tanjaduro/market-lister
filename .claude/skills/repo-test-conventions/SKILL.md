---
name: repo-test-conventions
description: Use when writing, modifying, or adding cases to any Go test file in this project (*_test.go). Captures the chdirAway env-isolation pattern, t.Setenv for every env var the code reads, t.TempDir for all filesystem work, t.Cleanup for restoring package-level vars, and the table-driven wantContains/wantNotContains assertion shape.
---

# Test conventions

The codebase has a consistent test style. New tests should match it — drift makes the suite flaky across machines.

## Env isolation: chdirAway

Quoted from config_test.go:5-7:

> chdirAway moves the test out of the project directory so godotenv.Load() in LoadConfig is a no-op (no .env present in t.TempDir). Combined with t.Setenv, this makes the test deterministic regardless of the developer's real .env.

Any test that exercises `LoadConfig` (or any code path that may `godotenv.Load()`) must call `chdirAway(t)` first.

## Patterns

See `references/skeletons.md` for copy-pasteable skeletons of:

1. **Env-isolated config test** — `chdirAway` + `t.Setenv` for *every* env var the function reads, including the ones you want empty. The host shell must not leak in.
2. **Filesystem test** — `t.TempDir()` for the working directory; never write to `.` or a hardcoded path.
3. **Package-var mutation test** — save the original into a local, mutate, register `t.Cleanup(func() { ... })` to restore. See `retryDelays` in vision_test.go:16.
4. **Table-driven text assertion** — `wantContains` / `wantNotContains` slices per case, `strings.Contains` in the assertion loop. See markdown_test.go:67.

## Rules of thumb

- Every env var the function reads gets a `t.Setenv`. Even when you want it empty, set it to `""` explicitly.
- All filesystem work goes inside `t.TempDir()`. Never touch the real filesystem outside the temp dir.
- Mutating a package-level var (e.g. `retryDelays = []time.Duration{0, 0}`) always pairs with a `t.Cleanup` that restores the original value.
- Output assertions on markdown / text use `strings.Contains` against expected-substring slices — not regex, not full equality.
- Table cases run under `t.Run(tc.name, ...)` so individual cases can be targeted with `-run`.
- Errors that should fail the test fast use `t.Fatal`; errors that should let other assertions in the same case run use `t.Error`.

## Verification

```
go test ./...
go test -race ./...
```

Get into the habit of `-race` locally now — it's free, and concurrency lands in NEXT-STEPS §2.4.
