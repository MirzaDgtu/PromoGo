# CI baseline follow-up: Q-P0-126 / Q-P0-127 fixed, 8/8 green (2026-09-09)

Follow-up to [`ci-clean-checkout-baseline-2026-09-09.md`](ci-clean-checkout-baseline-2026-09-09.md),
which ran the full `.github/workflows/ci.yml` job set from a clean checkout
at `3e7c174` and found 6/8 jobs green with two real, pre-existing failures
(`race`, `secret-scan`), deliberately left unfixed and filed as `Q-P0-126`
and `Q-P0-127`. This document covers: fixing both, then re-running the same
full clean-checkout suite to confirm a green baseline before the security/
regression review starts.

## What changed

1. **`Q-P0-126` (`race`)** — `internal/httpserver/middleware_test.go`'s
   `fakeStoreAPIKeyRepo` stored `*domain.StoreAPIKey` in a plain
   unsynchronized `map`, and `RequireStoreAPIKey`'s async
   `TouchLastUsed` call (via `BackgroundTracker.Go`) mutated the same
   struct pointer another goroutine was reading. Fixed by storing
   `domain.StoreAPIKey` **by value** under a `sync.Mutex`, handing callers
   fresh copies on every read — no goroutine ever mutates a struct another
   goroutine holds. Commit `3cf98ba`.
2. **`Q-P0-127` (`secret-scan`)** — `docs/load-testing.md` showed a real
   `loadtest-seed` output (a throwaway dev-stand key) as its example
   `api_key`. Replaced with an explicit placeholder in the current tree.
   Since gitleaks scans full git history by default and the old literal
   string still exists in commit `82399bb6`'s tree (rewriting history was
   out of scope), added a narrow allowlist entry matching that **exact
   literal string** — not the whole file or commit — so nothing else in
   that commit or file is silently allowlisted. Commit `ab6909a`.
   - Note on the process: first tried an `[[allowlists]]` array in
     `.gitleaks.toml` to add this as a second, independently-scoped entry
     alongside the pre-existing SKILL.md one. gitleaks v8.21.2's top-level
     config turned out to only honor a single `[allowlist]` table — the
     array form silently dropped the regex-based SKILL.md entry instead of
     adding a second one. Caught by testing the throwaway config against
     the real scan output before committing anything, not assumed to work
     from the TOML syntax alone. Settled on adding a second entry to the
     `regexes` array inside the existing single table (conditions across
     categories are OR'd, confirmed empirically), which needed no path or
     commit scoping since the value matched is unique to that one dead key.

Both fixes were verified against the *live working tree* (not yet a fresh
clone) before being committed:
- `Q-P0-126`: `go test -race ./internal/httpserver/...` inside a
  `golang:1.25` + gcc container (same setup as the CI `race` job) — `ok`,
  1.76s.
- `Q-P0-127`: the exact `secret-scan` CI command
  (`zricethezav/gitleaks@sha256:0e99e...1a7f46 detect --source /repo
  --config /repo/.gitleaks.toml --redact -v`) — `no leaks found`.

## Full re-run from a clean checkout

Same methodology as the first baseline: a fresh `git clone` into a new
scratch directory, `core.autocrlf=false` / `core.eol=lf` forced and
re-materialized via `git checkout-index -a -f`, pinned to a single commit,
kept separate from the working tree.

- Commit: `ab6909a82b4307e4664ddd213da855bb65f6038e` ("Fix Q-P0-127: replace
  example API key with a placeholder, allowlist the historical literal")
- Branch: `main`
- Working copy at time of the run: clean
- Same host-artifact adaptations as the first baseline applied again for
  the same reasons (CRLF on this host's global git config; no native gcc,
  so `race` ran inside `golang:1.25`+gcc with `GOTOOLCHAIN=auto`) — neither
  is a repository finding, both documented in the first baseline.

### Results by job

| Job | Steps | Result | Duration (sum of steps) |
|---|---|---|---|
| `verify` | go mod verify, gofmt, build, vet, test -shuffle, httpserver coverage, redocly lint, route/OpenAPI parity | **PASS** (8/8) | ~24s |
| `race` | test -race ./... (in `golang:1.25` + gcc) | **PASS** — no data races (was 2, now fixed) | 1m54s |
| `static-analysis` | staticcheck, govulncheck | **PASS** (2/2; same 3 unreached-dependency vulnerabilities noted, non-blocking, as before) | ~22s |
| `migrations` | goose validate, up from zero, down-to 0, re-apply | **PASS** (4/4) | — |
| `migration-deploy-model` | build, `promogo migrate`, two concurrent replicas, stale-schema refusal, restore | **PASS** (5/5) — no harness mistake this time (all env vars exported in one shell block) | — |
| `e2e-pilot` | `TestPilotEndToEnd` (`-tags e2e`) | **PASS** — all 9 phases | ~3s |
| `container` | docker build, Trivy scan, Syft SBOM | **PASS** (3/3) — Trivy: 0 HIGH/CRITICAL; SBOM: 224 KB SPDX JSON | ~4s build + Trivy DB download + ~7s SBOM |
| `secret-scan` | gitleaks detect | **PASS** — no leaks found (was 1, now fixed) | ~12s |

**Overall verdict: 8 of 8 jobs green.**

### Step-by-step notes (only what differs from the first baseline)

- **`verify`**: identical results to the first baseline —
  `internal/httpserver` coverage 74.3% (gate ≥65.0%, unchanged by the
  mutex fix since it only touched a test fixture), Redocly's same 2
  pre-existing `operation-4xx-response` warnings on `/healthz`/`/readyz`
  (exit 0, warnings don't fail the lint).
- **`race`**: `go test -race -count=1 ./...` — every package, including
  `internal/httpserver`, passes clean. No `WARNING: DATA RACE` anywhere.
- **`static-analysis`**: `govulncheck` still reports the same 3 known,
  currently-unreached `golang.org/x/crypto` vulnerabilities
  (`GO-2026-6355`, `GO-2026-6354`, `GO-2026-5932`) — informational, exits
  0, unrelated to either fix in this follow-up.
- **`migration-deploy-model`**: the stale-schema refusal step that
  produced a false alarm in the first baseline (a forgotten
  `PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true` re-export across separate
  shell invocations) was run this time with every required environment
  variable exported inside the *same* shell block as the check itself.
  Clean result on the first attempt: `promogo` refused the stale schema
  immediately (`verify migrations: database schema is behind: pending
  migrations exist`, exit 1), as designed.
- **`container`**: Trivy and Syft results are numerically identical to the
  first baseline (0 HIGH/CRITICAL; 224 KB SBOM) — expected, since neither
  fix touched the image's dependency surface.
- **`secret-scan`**: `36 commits scanned` (one more than the first
  baseline's 35, from the two new fix commits) — `no leaks found`.

## Flaky / retried / skipped

Same as the first baseline: no test was retried or skipped by the
workflow itself, and no flakiness was observed as "passed on retry" —
every job produced a deterministic result on its first attempt in this
run.

## Logs and artifacts

Raw step logs for this follow-up run are saved locally (not committed)
under:
```
%LOCALAPPDATA%\Temp\claude\c--Projects-Golang-github-com-MirzaDgtu-PromoGo\<session>\scratchpad\ci-logs-followup\
```
One log file per step (`01-verify-gomodverify.log` through
`24-secretscan-gitleaks.log`), plus the generated SBOM and the clean
checkout itself in the same scratch directory — reproducible from commit
`ab6909a` above, not committed.

## Verdict

8 of 8 CI jobs pass cleanly from a clean checkout at `ab6909a`. Both
findings from the first baseline (`Q-P0-126`, `Q-P0-127`) are fixed,
verified against the live tree before commit, and re-confirmed fixed in
an independent, from-scratch clean-checkout run. No new problem was found
in this follow-up. The project now has a **confirmed clean CI baseline**
to build the Phase 1–3 security/regression review and the pilot
rollout/rollback checklist on.
