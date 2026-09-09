# CI baseline: full suite from a clean checkout (2026-09-09)

First run of the entire `.github/workflows/ci.yml` job set against a truly
clean checkout — a fresh `git clone` into a scratch directory (not the
working tree used for day-to-day edits), pinned to a single commit, with
`core.autocrlf=false` so on-disk bytes match what GitHub's `ubuntu-latest`
runners actually check out. Every job's own steps were run in their
documented order using the same commands, tool versions, and (where
relevant) real Postgres/Redis/Docker infrastructure the workflow specifies.
Recorded as-is, per explicit instruction: **no new problem found during this
run was fixed in this commit** — genuine findings are tracked as backlog
items instead (see "Findings" below).

## Commit and working-copy state

- Commit: `3e7c17413bab8bba7475e5cc61546eb711172b57` ("Close Q-P0-095:
  record the pilot E2E test in the backlog and decision log")
- Branch: `main`
- Working copy at time of the run: clean (`git status --short`, excluding
  the untracked `graphify-out/` knowledge-graph artifacts, empty)
- Clean checkout: fresh `git clone` of the local repository into a scratch
  directory, `git checkout` to the commit above, `core.autocrlf=false` /
  `core.eol=lf` forced and re-materialized via `git checkout-index -a -f`
  (see "Environment note" below for why this mattered)

## Tool versions

| Tool | Version |
|---|---|
| Go (host) | go1.25.14 windows/amd64 |
| Go (in `golang:1.25` container, `race` job) | auto-upgraded via `GOTOOLCHAIN=auto` to go1.25.14 linux/amd64 (base image ships go1.25.12, below go.mod's `go 1.25.14` floor) |
| Docker | 28.4.0 (client and server) |
| Docker Compose | v2.39.2-desktop.1 |
| Node.js | v24.11.1 (npx 10.8.2) |
| git | 2.52.0.windows.1 |
| goose | v3.27.3 (pinned in workflow) |
| staticcheck | 2026.1 (pinned in workflow) |
| govulncheck | v1.1.4 (pinned in workflow) |
| @redocly/cli | 1.25.5 (pinned in workflow) |
| Trivy | image digest `sha256:ab70a02200597efa04748f210f793936eb647cbcdb0ea69cc30b226d6f5a22c7` |
| Syft | image digest `sha256:b8c170b8e51bfc4779ec3ef4399942c57290f5ce76a9c3af564c9d00d4946a6b` |
| gitleaks | image digest `sha256:0e99e8821643ea5b235718642b93bb32486af9c8162c8b8731f7cbdc951a7f46` |
| Postgres (service container) | 16-alpine |
| Redis (service container) | 7-alpine |
| gcc (for `CGO_ENABLED=1`, `race` job) | 14.2.0 (Debian, installed inside `golang:1.25` container — this Windows host has no native C toolchain) |

## Environment note: this host is not `ubuntu-latest`

Two adaptations were needed to get a result that actually reflects what CI
produces on GitHub's runners, both recorded here rather than silently
worked around:

1. **Line endings.** This machine's global `core.autocrlf=true` rewrites
   every checked-out file to CRLF. A first pass of `gofmt -l .` against
   that checkout flagged all ~130 Go files as "not gofmt'd" — not a real
   formatting problem, just CRLF vs. the LF gofmt normalizes to. Re-cloned
   with `core.autocrlf=false` / `core.eol=lf` before re-running; the
   corrected checkout matches what `actions/checkout` produces on
   `ubuntu-latest` and `gofmt -l .` returns clean.
2. **No native cgo toolchain.** The `race` job needs `CGO_ENABLED=1` and a
   C compiler (Go's race detector requires cgo). This Windows host has no
   `gcc` on `PATH`. Ran that job's steps inside a `golang:1.25` Docker
   container with `apt-get install -y gcc` (mirroring the workflow's own
   `sudo apt-get update && sudo apt-get install -y gcc` step), volume-mounted
   to the same clean checkout — everything else ran natively.

Both are host artifacts of running this off GitHub Actions, not findings
about the repository.

## Results by job

| Job | Steps | Result | Duration (sum of steps) |
|---|---|---|---|
| `verify` | go mod verify, gofmt, build, vet, test -shuffle, httpserver coverage, redocly lint, route/OpenAPI parity | **PASS** (7/7) | ~66s |
| `race` | test -race ./... (in `golang:1.25` + gcc) | **FAIL** — 2 reproducible data races (test fixture, not production code — see Findings) | 2m35s |
| `static-analysis` | staticcheck, govulncheck | **PASS** (2/2; govulncheck reports 3 vulnerabilities in an unreached dependency — see Findings, does not fail the gate) | ~25s |
| `migrations` | goose validate, up from zero, down-to 0, re-apply | **PASS** (4/4) | ~6s (post-download) |
| `migration-deploy-model` | build, `promogo migrate`, two concurrent replicas, stale-schema refusal, restore | **PASS** (5/5; see note below on a self-corrected harness mistake) | ~9s (excl. one aborted+retried step) |
| `e2e-pilot` | `TestPilotEndToEnd` (`-tags e2e`) | **PASS** — all 9 phases (webhook → duplicate delivery → OTP login → mobile visibility → QR → redeem → history → refund → refund duplicate delivery → mobile visibility) | ~3s |
| `container` | docker build, Trivy scan, Syft SBOM | **PASS** (3/3) — Trivy: 0 HIGH/CRITICAL; SBOM: 224 KB SPDX JSON generated | ~1m57s (incl. Trivy DB download) |
| `secret-scan` | gitleaks detect | **FAIL** — 1 finding, pre-existing, in `docs/load-testing.md:46` (see Findings) | ~17s |

**Overall verdict: 6 of 8 jobs green. 2 red (`race`, `secret-scan`), both
genuine, both pre-existing on `main` at this commit, neither touched or
fixed as part of this run.**

## Step-by-step detail

### `verify` — PASS
- `go mod verify`: `all modules verified` (25.2s, dominated by first-time
  module download/verification, not gate logic)
- `gofmt -l .`: clean (0 files) — after the CRLF correction above
- `go build ./...`: clean, 6.9s
- `go vet ./...`: clean, 2.5s
- `go test -shuffle=on -count=1 ./...`: all packages pass, no flakes, 6.1s
- `internal/httpserver` coverage: 74.3% (gate: ≥65.0%) — pass
- Redocly OpenAPI lint: valid, 2 pre-existing warnings
  (`operation-4xx-response` on `/healthz` and `/readyz` — neither endpoint
  documents a 4XX response; both are gate-only-on-200 health endpoints by
  design, warnings not errors, not fixed here)
- Route/OpenAPI parity test: pass, 0.1s

### `race` — FAIL (genuine)
`go test -race -count=1 ./...` inside `golang:1.25`+gcc, 2m35s. Every
package passes except `internal/httpserver`, which fails with two
`WARNING: DATA RACE` reports:

1. `TestHandleCreateStoreAPIKey_PlaintextAuthenticatesRealRequest`: a
   write in `(*fakeStoreAPIKeyRepo).Create` (`middleware_test.go:102`)
   races a read in `(*fakeStoreAPIKeyRepo).TouchLastUsed`
   (`middleware_test.go:118`), reached via a `BackgroundTracker`-launched
   goroutine (`background.go:25-27`) still running from a prior subtest.
2. `TestRateLimit_AccrualPrincipalIsolatedByStoreAPIKey`: two concurrent
   `(*fakeStoreAPIKeyRepo).TouchLastUsed` writes (`middleware_test.go:120`)
   from two `BackgroundTracker` goroutines racing each other directly.

Both are in the **test double** (`fakeStoreAPIKeyRepo` in
`internal/httpserver/middleware_test.go` — a plain Go `map`, no mutex),
triggered by production code's intentionally-async "touch last used"
(`middleware.go:101-102`, fire-and-forget via `BackgroundTracker.Go`). The
real `StoreAPIKeyRepository` (Postgres) does not have this race —
concurrent `UPDATE`s are serialized by the database. Reproduced once in
this run; not re-run multiple times to separately establish a flake rate
(the failure mechanism — an unsynchronized map hit by a leaked background
goroutine — is deterministic in nature, not a timing coincidence, so a
repeat-run flake check was not judged necessary before filing it. That
judgment is itself worth someone independently checking rather than taking
on faith.). Tracked as `Q-P0-126` in `Project Questions.md` section 15 —
not fixed here.

### `static-analysis` — PASS (with a noted, non-blocking observation)
- `staticcheck@2026.1`: clean, no output, 9.5s
- `govulncheck@v1.1.4`: `0 vulnerabilities` reachable by our code; reports
  3 known vulnerabilities in `golang.org/x/crypto@v0.55.0` (`GO-2026-6355`,
  `GO-2026-6354` — DoS in `x/crypto/ssh`; `GO-2026-5932` — unmaintained
  `x/crypto/openpgp`) that exist in a required-but-unreached dependency.
  `govulncheck` exits 0 in this case (matches the workflow's actual gate
  condition — "affected by 0 vulnerabilities" is what governs pass/fail,
  not "0 vulnerabilities anywhere in the dependency graph"), so this job
  is genuinely green, not a masked failure. Recorded here since it's a
  real, if currently inert, fact about the dependency tree — not filed as
  a P0/P1 since nothing currently calls the vulnerable code paths.

### `migrations` — PASS
`goose validate` → `up` (0 → 28) → `down-to 0` → `up` (0 → 28) again, all
clean, against a throwaway `postgres:16-alpine` container. ~6s total
excluding first-run tool download.

### `migration-deploy-model` — PASS (after a self-corrected harness mistake)
- Build + `promogo migrate` (the standalone deploy step): clean.
- Two replicas started concurrently against an already-migrated schema
  (`PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true`): both reached `/readyz`
  200 within ~1s, no re-migration race.
- Stale-schema refusal check: **first attempt produced a false alarm**,
  not a product bug — the shell command that ran this step forgot to
  re-export `PROMOGO_APP_SKIP_STARTUP_MIGRATIONS=true` (environment
  variables set in one Bash tool call do not persist into the next one in
  this environment), so the binary silently took the "auto-migrate"
  branch instead of the "verify and refuse" branch, applied the missing
  migration on its own, and started serving — which looked exactly like
  a "doesn't refuse a stale schema" regression until the mistake was
  found by reading `internal/app/app.go:85-101`, confirming the two-branch
  logic was correct, and re-running with the environment variable
  correctly set. On the corrected re-run: `promogo` failed startup
  immediately (`verify migrations: database schema is behind: pending
  migrations exist (run \`promogo migrate\` first)`, exit 1) as designed.
  Recorded in full rather than silently discarded, since the first
  (wrong) result was briefly treated as a real finding before the mistake
  was caught — worth being able to see that trail.

### `e2e-pilot` — PASS
`TestPilotEndToEnd -tags e2e`, real Postgres/Redis, 0.64s test time (~3s
including process startup). All 9 phases logged with HTTP 200 at every
step; no duplicate-accrual, no OTP/QR/redeem/refund inconsistency.

### `container` — PASS
- `docker build`: clean, 5.1s (mostly cache hits on unchanged layers).
- Trivy (`--severity HIGH,CRITICAL`, repo's `.trivyignore` applied):
  `Total: 0 (HIGH: 0, CRITICAL: 0)` on `promogo:ci (alpine 3.22.3)`.
- Syft SBOM (SPDX JSON): generated, 224 KB, 7.4s.

### `secret-scan` — FAIL (genuine, pre-existing)
`gitleaks detect --config .gitleaks.toml --redact -v` against the full
clone history (33 commits scanned, 17.4s): 1 finding.

- Rule: `generic-api-key`, entropy 5.07
- File: `docs/load-testing.md:46`
- Commit: `82399bb6bc0b309c11aef2e2148beb307171ee58` (Phase 4,
  load-testing documentation)
- Content: a sample `api_key` value shown as the illustrative output of
  `go run ./cmd/promogo loadtest-seed` in a doc code block — a real
  dev-stand key generated by a throwaway local seeding command, not a
  production credential, but gitleaks has no way to tell that apart from
  a real leaked key without an explicit allowlist rule.

Tracked as `Q-P0-127` in `Project Questions.md` section 15 — not fixed
here (fixing it means a product decision: redact the example to a
placeholder, or add a justified `.gitleaks.toml` allowlist entry; making
that call silently would undercut the point of a clean, unmodified
baseline run).

## Flaky / retried / skipped

- **No test was retried or skipped** in any job (the workflow does not use
  test retries, and no `t.Skip` fired anywhere in this run).
- **No flakiness observed** in the sense of "passed on retry" — every job
  produced a deterministic result on its first attempt. The one item that
  looked like a false failure (`migration-deploy-model`'s stale-schema
  step) was a harness/environment-variable mistake on the runner side of
  *this recording exercise*, not the workflow or the product code
  behaving inconsistently — see the note above.
- The `race` failure was observed once; whether it reproduces 100% of the
  time across independent runs was not separately verified (see the
  `race` section above).

## Logs and artifacts

All raw step logs are saved locally (not committed — they're a byproduct
of this recording run, not repo content) under:
```
%LOCALAPPDATA%\Temp\claude\c--Projects-Golang-github-com-MirzaDgtu-PromoGo\<session>\scratchpad\ci-logs\
```
One log file per step (`01-verify-gomodverify.log` through
`21-secretscan-gitleaks.log`), each starting with the exact command run and
ending with `real`/`exit` timing. The generated SBOM
(`promogo-sbom.spdx.json`) and the clean checkout itself
(`ci-clean-checkout/`) live alongside them in the same scratch directory —
both are reproducible from the commit above and were not committed either.

## Verdict

6 of 8 CI jobs pass cleanly from a clean checkout at `3e7c174`. 2 fail for
real, pre-existing reasons unrelated to Phase 5's work:

- `race` — a data race confined to a test fixture
  (`internal/httpserver/middleware_test.go`'s `fakeStoreAPIKeyRepo`), not
  production code. Tracked as `Q-P0-126`.
- `secret-scan` — a gitleaks false positive on a documented example API
  key from Phase 4's load-testing guide. Tracked as `Q-P0-127`.

Neither was introduced in this run, and neither was fixed as part of it,
per instruction. Both now have an owner, risk, and milestone in
`knowledge/Project Questions.md` section 15, alongside the rest of the
project's open-question backlog.
