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
  Two habits worth keeping: the readiness checks that touch NATS are tested
  against a **real operator-mode `nats-server`** built in the test (see
  `internal/health/nats_test.go`), because the thing being asserted IS the
  server's trust decision and a mock only asserts what we believe it to be; and
  the metrics exposition is parsed by **Prometheus's own parser and promlint**
  rather than string-matched, because nothing in CI scrapes it and a malformed
  body looks fine in a terminal.
- **`gofmt -l .` reports ~25 files on a Windows checkout, and they are all
  fine.** `core.autocrlf` rewrites `.go` files to CRLF in the worktree while
  `.gitattributes` (`*.go text eol=lf`) keeps the committed content LF, so gofmt
  sees line endings git will never store. The CI gate is plain `gofmt -l .`, and
  it passes because CI checks out LF. To check locally the way CI will, run it
  against what git STORES rather than the worktree:
  `git ls-files '*.go' | while read f; do git show ":$f" > /tmp/x.go; gofmt -l /tmp/x.go; done`.
  Do not "fix" the files `gofmt -l .` lists here, and do not conclude the gate is
  broken.
- **A test that reads a UI source file must be line-ending tolerant.**
  `.gitattributes` pins `*.sh`, `*.go`, `Dockerfile`, `*.yaml` and `*.yml` to
  LF and nothing else, so with `core.autocrlf=true` a `.vue` or `.ts` file is
  **CRLF in a Windows worktree and LF in git**. `TestActivityNounsAllHaveAConsoleRoute`
  matched `\{\n` against `ActivityView.vue` and therefore could never pass on a
  Windows checkout and always passed in CI -- the worst pairing available,
  because a genuinely broken `hooks` package hides behind a red run people have
  learned to ignore. Fixed with `\r?\n`, and verified against both line endings
  rather than just the one on this machine. Match `\r?\n`, or strip `\r` before
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
- **Capture "before" state immediately before the action it belongs to.** A check
  that an operation had an effect is worthless if anything between the two reads
  could have caused it. `rotation actually re-minted the credential` passed for
  months against a route that did nothing, because its baseline was captured a
  section earlier and an intervening `publish_permissions` PATCH re-minted the
  credential as a side effect. Side-effect assertions need a baseline read on the
  line above the call.
- **A re-minted NATS credential is byte-identical within the same second, so a
  "did it change" assertion is a race.** `regenerateUserJWT` reuses the stored
  seed, and a user JWT is deterministic apart from `iat`/`exp` — whole seconds
  (`nats-io/jwt` sets `IssuedAt = time.Now().UTC().Unix()`, hashes the claims into
  the id with a deliberately "repeatable hash", and signs with Ed25519, which is
  deterministic). Two mints in one wall-clock second produce the same
  `creds_file`. `rotation actually re-minted the credential` therefore passed for
  months and then failed on a commit that touched nothing near it, because the
  preceding `publish_permissions` PATCH happened to land in the same second. The
  suite now sleeps past the boundary AND asserts pb-nats cleared `regenerate`,
  which is the same evidence without a clock in it. Prefer a side-effect
  assertion that cannot be defeated by timestamp granularity; where the only
  observable IS a timestamped artifact, separate the two events explicitly.
