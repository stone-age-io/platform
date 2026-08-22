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

## [0.1.0]

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

[Unreleased]: https://github.com/stone-age-io/platform/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/stone-age-io/platform/releases/tag/v0.1.0
