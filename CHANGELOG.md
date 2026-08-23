# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) — with the pre-1.0
caveat that a minor version may break something. Pin what you deploy.

History before `0.1.0` is not reconstructed here. Roughly three hundred commits
of pre-release development preceded it; `git log` is the record for that period,
and this file starts where the versioned releases do.

## [Unreleased]

### Added

- **Readiness endpoint: `GET /api/ready`.** PocketBase's `/api/health` reports
  that the HTTP server is listening, which is true of every failure worth
  catching here. This returns 503 while the operator is unseeded, the schema was
  never imported, or — the one that is otherwise invisible — the NATS server
  does not trust this platform's operator, so every account claim it publishes
  is rejected, no organization's account ever reaches the bus, and the console
  looks fine. Warnings (at-rest encryption off, no browser-facing WebSocket URL)
  still answer 200; a probe can only restart or de-register, and neither fixes a
  note. Unauthenticated — the callers are orchestrators and uptime checkers that
  hold no session — and every check carries a `detail` plus, when unhappy, a
  suggested `fix`, with the same text in the log. Checks run on a background
  interval and the endpoint serves the cached answer, so a probe never triggers
  a NATS dial.

- **Prometheus metrics: `GET /metrics`.** The readiness checks as a state set,
  this process's HTTP traffic by matched route pattern, row counts from its own
  database, SQLite size on disk, and — only with `serve --nats` — the embedded
  bus's own counters. Open by default; `metrics.token` closes it and is accepted
  as `Authorization: Bearer` or as HTTP Basic with any username, covering every
  scraper in common use. PocketBase's own auth is deliberately not used: its
  tokens expire and no scraper has a refresh flow.

  Nothing is per-organization. This process holds the NATS operator and the
  `$SYS` account and has no credential inside any tenant's account, so it cannot
  see what is in one. `stone_age_records{collection="leaf_nodes"}` counts leaf
  nodes **configured** — it is not availability, and an alert on it can never
  fire.

- **`leaf-sync` exports the health the platform cannot see.** `/ready` and
  `/metrics` behind `observability.addr` (off by default — opening a port on an
  edge appliance should be a decision): sync-cycle freshness, per-collection
  mirrored counts, per-collection errors, whether the uplink to the hub is
  attached, JetStream bytes, and devices actually connected at the site. The
  server-derived numbers come from the leaf's loopback monitoring port, so the
  edge reads its own server without ever holding a `$SYS` credential. An
  islanded site warns rather than fails and still answers 200: local NATS keeps
  working and devices keep running against the mirrored config, which is the
  entire reason a leaf node exists.

  This is not the `leaf_status` heartbeat. That is a tenant-facing feature the
  console renders, delivered over the very link that breaks, so it goes quiet
  during exactly the outage you want detail about. These endpoints are scraped
  locally and keep answering with the WAN down.

- **`HEALTHCHECK` in the container image**, wired to `/api/ready`. It marks the
  container unhealthy rather than restarting it, which is right: most of what
  the endpoint reports is not fixed by a restart.

### Fixed

- **`internal/natsd` claimed clustering the Control Plane was "deliberately not
  supported". It was never true.** `Start` hands the parsed config straight to
  `nats-server` — that is the package's stated design, "no embedded-only mode,
  no options derived in Go" — so a `cluster` block has always been honoured like
  any other directive, and `serve --nats` can be one node of a cluster whose
  other nodes are plain `nats-server` processes. The comment contradicted its
  own package doc three paragraphs above it. `TestEmbeddedServerClustersWithAPeer`
  now pins the behaviour with two real peered servers, and
  `stone_age_nats_cluster_routes` reports the peers. The real cost of the
  topology is unchanged and now stated where the claim used to be: the bus dies
  with the Control Plane process, so restarting the platform takes a cluster
  node with it.

### Changed

- `github.com/prometheus/client_golang` is now a direct dependency. It was
  already in the module graph via `slackhq/nebula`, so the binary does not grow.
  Preferred over hand-writing the text format for the same reason the generated
  leaf config is validated by `nats-server` itself: nothing in CI scrapes this,
  so a malformed exposition would look fine in a terminal and be silently
  unusable. The tests parse it with Prometheus's own parser and run promlint.

## [0.2.0] - 2026-08-22

Two fixes for installs that had not gone wrong yet, and real versions for the
four pb-* libraries this binary is built from.

### Fixed

- **A fresh install could not repair itself.**
  `nats_system_operator.signing_public_key`, `signing_private_key` and
  `signing_seed` were declared required but are never written. pb-nats moved to
  a list in `signing_keys` / `signing_keys_private` and stopped declaring the
  scalars; they survived here only because `schema.json` is a dump taken while it
  still did. pb-nats creates the operator record *before* this schema is
  imported, so on a fresh install the three columns arrive empty and required,
  and the next save of that record fails with `signing_private_key: cannot be
  blank`. Nothing re-saves the operator in normal operation — every path that
  does is a repair (resolving the system account, regenerating the operator JWT)
  — so the trap was armed for the moment something had already gone wrong. It was
  also unrecoverable in place: pb-nats initializes from `OnBootstrap`, which
  fires for *every* command, so an operator save it cannot complete takes down
  `migrate up` as well as `serve`, leaving the binary unable to reach the
  migration that would relax the flag.
  `migrations/schema_update_operator_legacy_signing_optional.go` makes the three
  optional. This protects installs going forward; a database already in that
  state stays stuck.
- **A Control Plane that never left bootstrap mode.** An operator collection
  created before `system_account_id` existed never acquired the field, and
  PocketBase discards writes to undeclared fields silently — so pb-nats's own
  repair path saved the id on every boot while persisting nothing. The symptom is
  a process that logs queued publish operations every 30s while making zero
  connection attempts. Nothing looks wrong until the NATS resolver directory is
  empty — a rebuilt server, or a restore onto a new host — at which point no
  account JWT is ever published and every client fails with `fetching jwt timed
  out`, because `ReconcileAccounts` is what repopulates the resolver on boot and
  is exactly what cannot run. Fixed by the pb-nats migration that adds the field.

### Added

- `--version` now names the pb-* libraries as well as PocketBase. That is where
  NATS credential minting, Nebula CA issuance, the tenancy collections and the
  audit trail actually live, so "which pb-nats is this" is the second fact any
  credential or authorization bug report needs. A Go library cannot be stamped
  with ldflags, so these are read from the build info — which is why the tags
  below matter:

  ```
  stone-age version v0.2.0
    pocketbase  v0.39.11
    pb-nats     v0.1.0
    pb-nebula   v0.1.0
    pb-tenancy  v0.1.0
    pb-audit    v0.1.0
  ```

### Changed

- **The pb-* libraries are pinned to `v0.1.0` instead of pseudo-versions.** All
  four are now tagged and released, so `go.mod` names releases rather than
  commits and this binary can be upgraded deliberately. No behaviour comes with
  the change: diffing each tag against the pseudo-version it replaced, the only
  non-tooling commits are two `gofmt` passes.
- Each of the four libraries also gained a release pipeline and, where it was
  missing, CI — so a future bump refers to something a reader can find.

## [0.1.0] - 2026-08-21

First tagged release. The platform has been in development and in use for
several months; this tag marks the point at which it is packaged for other
people to run, not the point at which it started working.

### Added

Summarising the state at first tag rather than the path to it:

- **Control plane for NATS and Nebula.** Single Go binary extending PocketBase,
  with the compiled Vue console embedded in it. SQLite for storage, via a pure-Go
  driver, so cross-compilation needs no C toolchain.
- **Multi-tenancy and roles.** Organizations, memberships, invitations, and five
  roles (`owner`, `admin`, `member`, `viewer`, `dashboard`) enforced entirely by
  PocketBase API rules in `schema.json`.
- **NATS operator-mode management.** Operator, account and user JWT provisioning;
  per-organization accounts; roles as permission templates; self-service
  credential rotation; account signing-key management; revocation that actually
  disconnects.
- **Nebula overlay management.** Certificate authorities, networks, lighthouses
  and host certificates, issued per organization.
- **Inventory as identity.** A Thing is simultaneously an inventory record, a
  login, a NATS identity and a mesh node. Deactivating one takes all four away in
  a single operation.
- **Edge nodes.** A `leaf_nodes` identity, the `leaf-sync` agent that mirrors an
  organization's configuration into a NATS leaf node's local JetStream KV, an
  optional in-process leaf node (`leaf-sync run --nats`), and liveness
  heartbeats surfaced in the console.
- **Digital twin.** Two KV buckets per organization with one writer each —
  `twin` for reported state flowing edge-to-hub, `twin_desired` for desired state
  flowing hub-to-edge — with drift shown as the values themselves rather than a
  status word.
- **Dashboards.** Grid-based dashboards with 17 widget types, three data-source
  kinds (subscription, consumer, KV), variable substitution, and a live NATS
  WebSocket connection from the browser.
- **Audit logging** across the tenancy, NATS and Nebula collections, with a
  searchable viewer.
- **Operator branding overlay**, so a deployment can be rebranded without
  rebuilding the frontend.

Added while preparing this tag, and therefore part of it:

- **`UNIQUE (organization, code)`** on `things`, `locations`, `thing_types`,
  `location_types` and `leaf_nodes`. `code` was documented as unique and used as
  one — as a NATS KV key at the edge, as a digital-twin key prefix, as the
  argument to `stone thing get` — while nothing enforced it. The migration sweeps
  for existing duplicates first and refuses with them listed rather than dying on
  a bare "UNIQUE constraint failed". The index is partial, so blank codes are
  still allowed, and it is scoped per organization, because two tenants both
  calling a site "HQ" is the tenancy model working.
- `--version` on the `stone-age` binary, reporting the stamped build version and
  the PocketBase module version it was built against. Both binaries now share one
  version variable (`internal/version.Version`), so one `-ldflags -X` stamps both.
- A CI workflow that builds the console, builds and tests the Go side, and runs
  the authorization suite on every pull request. It also asserts the deliberately
  pinned dependency majors (Tailwind 3, daisyUI 4, TypeScript 6), which had no
  automated guard at all — there is no frontend test runner, so `vue-tsc && vite
  build` stays green while the console renders wrong.
- A Dockerfile and a first-boot entrypoint: one container that seeds itself and
  runs the NATS server in the same process. No compose file, because `serve
  --nats` is why the flag exists.
- A goreleaser config and a release workflow: cross-compiled binaries for both
  commands on linux, darwin and windows (amd64 and arm64), plus a multi-arch
  image on GHCR. `modernc.org/sqlite` is pure Go, so none of this needs a C
  toolchain.
- MIT license, a security policy, and this changelog.

### Fixed

Found by reading the codebase against its own documentation before making the
repository public. Each of these was reproduced before being fixed.

- **A user removed from an organization kept reading that organization's
  inventory.** Every read rule on the inventory collections is
  `organization = @request.auth.current_organization` with no membership branch,
  and nothing cleared `current_organization` when the membership behind it was
  deleted. Their writes stopped (every write branch names a role) but their reads
  did not, and because `users` has no `authRule` they could log out and back in
  and land in the same organization again. `hooks/membership_lifecycle.go` now
  clears the context as part of the membership delete, and
  `scripts/test-authz.sh` reproduces the original exposure.
- **Search looked at the page you were on, not the collection.** Sixteen list
  views filtered the twenty records already on screen, so a match on page three
  answered "No results found". Worst in the audit log, where a false negative has
  consequences. All sixteen now filter server-side, debounced, resetting to page
  one, with the filter reaching the pager buttons too.
- **`userRole` defaulted to `member` when no membership resolved**, which handed
  member capabilities to someone with no organization context and made the
  console offer controls the API refused. It is `null` now.
- **`nats_username` uniqueness was checked globally**, so the second tenant who
  wanted `gateway-01` was refused. A NATS user name only has to be unique within
  its account.
- An organization-name slug collision ("Acme Inc" and "Acme, Inc." both shorten
  to `acme-inc`) surfaced as a generic "failed to create thing". It now says what
  happened and what to do about it.
- `leaf_nodes.code` is frozen after creation, matching how `organization` already
  was — it is the JetStream domain suffix and the KV key prefix the edge has
  already written under, so changing it centrally orphaned the site silently. The
  docs already claimed it was immutable.
- `leaf-sync` now logs when it falls back to keying a record by id, naming the
  duplicated handle. The fallback keeps the mirror correct, which is exactly why
  the underlying data problem was invisible.

### Changed

- `pb_public/index.html` is now tracked as a placeholder, so `go build`,
  `go vet` and `go test` work on a fresh clone without installing Node and
  building the frontend first. `npm run build` overwrites it.
- The README leads with the install rather than the architecture. Measured on a
  clean clone: about two minutes of machine time from `git clone` to a serving
  console with the bus up.
- `scripts/test-authz.sh` grew from 135 to 147 checks, covering the membership
  lifecycle, the code uniqueness constraint, and the frozen leaf-node code.

[Unreleased]: https://github.com/stone-age-io/platform/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/stone-age-io/platform/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/stone-age-io/platform/releases/tag/v0.1.0
