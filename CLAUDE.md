# CLAUDE.md

Project-local rules for working in market-lister.

## Commit format

Use `type(scope): subject` — lowercase, imperative, no period.

Types from actual usage: `feat`, `fix`, `docs`, `test`, `config`, `chore`, `ci`, `deps`, `prompt`, `log`.

Scope only when there's a specific subsystem (e.g. `fix(validate):`, `log(processFolder):`). For repo-wide changes, drop the scope and use just `type: subject`.

## Append-only design docs

`PLAN.md` and `NEXT-STEPS.md` are design records, not living docs. Never edit or delete existing content.

To supersede a decision, append a note below the original entry — the original wording stays in place, the new note explains what changed and why. See PLAN.md decision #6 for the pattern: the row remains, prefixed with "Superseded —", and the original rationale is preserved verbatim.
