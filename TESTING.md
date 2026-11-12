# Testing — the long form

The commands are in the root `CLAUDE.md`. Vitest, the list-view sort check and
the manual frontend checks are in `ui/CLAUDE.md`. This file is the reasoning
behind the Go and script suites, and the traps in them.

## Go

- `go test -short ./...` — **the pass to run while working: ~19s instead of
  ~11min.** It skips exactly one thing: tests that stand up a real PocketBase.
  That is where all the time is — booting one costs the better part of ten
  seconds (four libraries bootstrap, every migration runs) and the suite does it
  about thirty times, which is the whole of `hooks`, `internal/demoseed` and
  `migrations`. The skip lives in `testutil.SkipIfShort`, called from `SetupApp`,
  so a test written next year lands in the right set without anyone remembering;
  the two packages whose `TestMain` builds one shared app guard it themselves,
  since `TestMain` runs before any test can skip (and must `flag.Parse()` first,
  or `testing.Short()` is always false).
  - **The real-`nats-server` tests are deliberately NOT skipped.** They cost
    about ten seconds between them and they assert the server's own trust
    decisions — the coverage you least want to drop from the pass you run most
    often.
  - `-short` is a local convenience and never a gate. CI runs
    `go test -count=1 ./...`, which skips nothing; verified at 0 skips. If a
    full run ever reports a skip, that is the bug — a test that cannot fail.
- `go test ./...` — everything, ~11min. What CI runs (`hooks` and `migrations`
  have the bulk of them).
  NATS-facing checks are tested against a real operator-mode `nats-server`
  (`internal/health/nats_test.go`), and the metrics exposition is parsed with
  Prometheus's own parser and promlint, not string-matched.
- **If `gofmt -l .` lists files on a Windows checkout, they are CRLF copies from
  before `.gitattributes` pinned `*.go` to LF.** The pin wins over
  `core.autocrlf` on every checkout since, so delete the listed files and
  `git checkout -- <files>` (or re-clone).
- **A test that reads a UI source file must be line-ending tolerant.**
  `.gitattributes` pins `*.sh`, `*.go`, `Dockerfile`, `*.yaml` and `*.yml` to
  LF and nothing else, so with `core.autocrlf=true` a `.vue` or `.ts` file is
  **CRLF in a Windows worktree and LF in git**. Match `\r?\n`, or strip `\r` before
  matching; do not add `*.vue text eol=lf` to `.gitattributes` to dodge it,
  which rewrites every contributor's worktree to fix one regex.

## `scripts/test-authz.sh`

- `./scripts/test-authz.sh` — **run after any API-rule change in `schema.json`.**
  Builds the binary, stands up a throwaway DB, and asserts every authorization
  behaviour in it against a live server — `EXPECTED_CHECKS` at the top of the
  script is the count, and is the only copy of it worth trusting. The rules are the only tenancy enforcement
  in the platform and nothing else type-checks them. Add a check when you add a
  rule, and bump `EXPECTED_CHECKS`. Note PocketBase answers 404 (not 403) when an
  update rule rejects, and 400 on a denied create — which is why every "cannot"
  is paired with a "can" on the same record.
- **Capture "before" state on the line above the action it belongs to.** Anything
  in between can cause the effect you are checking for.
- **A re-minted NATS credential is byte-identical within the same second**
  (`regenerateUserJWT` reuses the seed, and `iat` is whole seconds), so "did the
  creds change" is a race. Assert a side effect with no clock in it instead, such
  as pb-nats clearing `regenerate`.
