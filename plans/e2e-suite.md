# Plan: an end-to-end suite against real NATS and Nebula

Status: **Phases 0, 1 and 3 done** (`e2e/`, five tests, ~100s, a CI step).
Phase 2 (the leaf node) is optional and untouched. **Changed in
implementation:** Phase 3 is a step in the existing job, not a separate job,
because `ci.yml` is one sequential job on purpose. Checked against the pre-fix libraries (pb-nats v0.4.0, pb-nebula
v0.5.0): the suspension test fails right after the member's rotation, and the
Nebula test fails with "private key is not a pair with public key in nebula
cert". On current versions all five pass.

**Changed in implementation:** "every library it needs is already in `go.mod`"
was wrong. Importing `github.com/slackhq/nebula` for `nebula.Main` moved it
from indirect to direct and added 23 indirect requirements (its stats, DNS and
tun backends). The binary does not link them and no existing version moved,
but `go.mod` is noisier.

Found in Phase 0:
- **The first ~30s after `serve --nats` starts, no account reaches NATS.**
  pb-nats starts before the embedded server accepts it, enters "bootstrap
  mode", and holds every publish until its 30s queue ticker. An org created
  just after a restart cannot connect its devices for up to 30s. The harness
  absorbs it (`connect` waits 45s); a fix belongs in pb-nats.
- **A binary under the OS temp dir runs in PocketBase dev mode**, which logs
  every SQL statement, seeds and private keys included. PocketBase reads a temp
  path as `go run`. The harness passes `--dev=false`.

## The problem

v0.19.1 fixed two bugs that every unit test passed: no Nebula host the platform
ever issued could start (the private key did not pair with the certificate),
and any member could undo an org's suspension by rotating their own NATS
credentials. Both lived in pb-nebula and pb-nats, and nothing asked the real
Nebula or the real NATS server whether the platform's output worked.

## The shape

- **The server under test is the built binary**: `init`, then `serve --nats`,
  exactly as a customer runs it. `testutil.NewApp` registers no routes and never
  serves, so it cannot stand in.
- **The probes are in-process**: `nats.go` clients connect with the
  `creds_file` the platform issued, and Nebula configs go through
  `nebula.Main(cfg, configTest=true, …)`, which loads the PKI and runs the
  `VerifyPrivateKey` check that refused to start in v0.19.1.
- **Same module, build tag `e2e`.** `go test ./...` never sees it; CI runs
  `go test -tags e2e -count=1 ./e2e/`. Every library it needs is already in
  `go.mod`, so the versions match pb-nebula's and pb-nats's by construction.
- **One platform per package**, started in `TestMain`. Each test makes its own
  organization. Tests run one after another until slowness says otherwise.
- **Wait by polling, never by sleeping.** pb-nats publishes after a 3s
  debounce, so assertions go through `eventually()`.

Deliberately not done: a userspace netstack and real handshakes over the
overlay, parallel platforms, a restart helper, a debounce config knob. Each is
cleverness for a bug that has not happened.

## The rule going forward

When a bug gets past the unit tests, its fix adds an e2e test. Do not write
tests here for bugs that have not happened.

## Phases

0. **Harness and smoke.** Build once; `init`; re-run `nats export` with random
   ports (`init` always exports 4222/9222/7422); `serve --nats`; wait for
   `/api/ready`. Smoke: an org's NATS user connects with its issued creds and a
   message round-trips.
1. **Four tests for what actually broke.**
   | Test | Would have caught |
   |---|---|
   | A Thing's creds publish on an allowed subject and are refused on a denied one | a limit of 0 disconnecting on connect, an empty allow list |
   | Suspend an org: the connection drops and stays down after a member rotates creds and an owner deactivates a device; un-suspend and it connects | the v0.19.1 suspension bypass |
   | Deactivating a Thing drops its live connection | deactivation not failing closed |
   | A lighthouse's and a host's `config_yaml` pass `nebula.Main(configTest=true)` | the v0.19.1 unpaired keys |
2. **Optional.** A leaf server started from `GET /api/me/leaf-config` carries a
   device's message to the hub (`internal/health/leaf_visibility_test.go` has the
   pattern).
3. **CI and docs.** One CI job, a short section in `TESTING.md`, one line in the
   root `CLAUDE.md` test list.

Out of scope: the UI (a Playwright smoke run against `demo-seed` is its own
plan), the Agent's own behaviour, rule-router.
