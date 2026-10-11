# CLAUDE.md - Stone Age IoT Platform

## Project Overview

Stone Age IoT Platform is a single-binary IoT and Event-Driven management platform combining:
- Go backend extending PocketBase
- Embedded Vue 3 frontend
- SQLite database
- NATS messaging infrastructure integration
- Nebula overlay network management
- Edge sites: a NATS leaf node run by the Agent (a separate repo), bootstrapped from `GET /api/me/leaf-config`

## Where the detail lives

This file holds what every session needs. The long-form notes sit next to the
code they describe. Claude Code loads a directory's `CLAUDE.md` the first time it
touches a file in that directory.

| File | Read it before you touch |
|---|---|
| `AUTHORIZATION.md` | an API rule in `schema.json`, a route or hook that writes with `app.Save()`, a credential, a `can.*` gate |
| `TESTING.md` | Go tests, `scripts/test-authz.sh`, or gofmt on Windows |
| `hooks/CLAUDE.md` | PocketBase hook mechanics, and features 7, 7b, 11, 12, 14, 15–15d, 16–16c |
| `migrations/CLAUDE.md` | any `schema.json` change or migration |
| `ui/CLAUDE.md` | the frontend: pinned majors, NATS URL tiers, the router guard, Vitest, and features 5, 13, 17–19 |

`AUTHORIZATION.md` and `TESTING.md` span several directories, so nothing loads
them automatically. Open them when a row above says to.

## Build & Run Commands

### Development
```bash
# Terminal 1: Backend
go run main.go serve

# Terminal 2: Frontend with HMR
cd ui
npm install
npm run dev
```
- Backend: http://localhost:8090
- Frontend dev: http://localhost:5173 (proxies to backend)
- PocketBase Admin: http://localhost:8090/_/

### Bootstrap (Initial Setup)
Four commands, in this order — the order is load-bearing:
```bash
./stone-age superuser upsert admin@example.com 'password'   # PB superuser + NATS $SYS seed
./stone-age migrate up                                      # import schema.json
./stone-age bootstrap --email admin@example.com --org "System" --operator-org "816tech"
./stone-age nats export --output ./nats-config/             # only for serve --nats
```
The fourth is needed only by `serve --nats`, which reads the exported operator
JWT and resolver config from disk — but it cannot run any earlier, because there
is no operator in the database until `bootstrap` has run. `docker-entrypoint.sh`
does all four in this order.
`bootstrap` writes `is_operator` / `is_system_org` / `is_operator_org`, which only
exist after the schema is imported. PocketBase silently drops writes to fields
that don't exist, so running `bootstrap` first yields a platform with no operator;
it now refuses to run before the migrations. This is also the only way to grant
operator status apart from the admin panel — the API cannot.

### Production Build
```bash
# Build frontend
cd ui && npm run build

# Build Go binary
go build -o stone-age .

# Run
./stone-age serve
```

The edge agent is a separate repo — [stone-age-io/agent](https://github.com/stone-age-io/agent) — and has its own deployment flow.

### NPM Scripts
- `npm run dev` - Vite dev server with HMR
- `npm run build` - TypeScript check + production build to `../pb_public`
- `npm run preview` - Preview production build

### Tests
- `go test -short ./...` - the pass to run while working (~19s). It skips only
  the tests that boot a real PocketBase. CI runs everything with
  `go test -count=1 ./...` (~11 min).
- `cd ui && npm test` - Vitest, pure logic only. Nothing renders a component in
  CI, so you check anything visual by hand.
- `./scripts/test-authz.sh` - after **any** API-rule change. Add a check and bump
  `EXPECTED_CHECKS`.
- `./scripts/check-sort-fields.sh` - after you add a `sortable:` column to a list
  view.
- On a Windows checkout, `gofmt -l .` lists about 25 files that are fine (CRLF in
  the worktree, LF in git). Do not "fix" them. `TESTING.md` has the check that
  matches CI.

## Tech Stack

### Backend
- **Go 1.26.0** with PocketBase 0.39.11
- **Cobra** for CLI commands (bootstrap, NATS management)
- **Viper** for configuration management
- **Key libraries**: pb-audit, pb-nats, pb-nebula, nats-io/jwt, slackhq/nebula (tenancy was pb-tenancy until it was absorbed into `hooks/`)

### Frontend
Vue 3, Vite 8, Pinia 4, Vue Router 5, TypeScript 6, Tailwind 3.4 + daisyUI 4.12,
NATS over WebSocket, Leaflet over MapLibre GL 6, ECharts. Three majors are pinned
on purpose (Tailwind and daisyUI, which move together, and TypeScript), and
`maplibre-gl` 6 is asserted. A routine "bump everything" pass must not move them,
and CI fails if it does. The full list, and the reasons: `ui/CLAUDE.md`.

### Database
- SQLite (managed by PocketBase, stored in `pb_data/`)

## Directory Structure

```
platform/
├── main.go                 # Backend entry point
├── go.mod, go.sum          # Go dependencies
├── config.yaml             # Application configuration
├── schema.json             # PocketBase collection schema
├── hooks/                  # Server-side hooks: org provisioning, leaf-config route, thing routes
├── migrations/             # Schema migrations (re-import schema.json)
├── ui/                     # Frontend Vue 3 application (tree in ui/CLAUDE.md)
├── AUTHORIZATION.md        # Authorization and credential rules, long form
├── TESTING.md              # Testing notes, long form
├── pb_public/              # Compiled frontend (generated)
└── pb_data/                # Runtime database (generated)
```

## Configuration

### config.yaml
Located at project root. Key sections:
- `tenancy` - Multi-tenant settings (collections, invite expiry)
- `nats` - NATS server URL, operator name, default limits
- `nebula` - Nebula CA/network/host settings
- `audit` - Audit logging configuration
- `readiness` - `GET /api/ready` check interval and timeout
- `metrics` - `GET /metrics` on/off and the optional scrape token
- `branding` - Operator branding overlay directory

**`nats.server_url` and `nats.websocket_urls` are not the same address.** The
first is the TCP address *this process* dials to publish account claims; the
second is the WebSocket listener a *browser* dials, on a different port and
often a different host. Never derive one from the other — a Control Plane
publishing to `nats://nats:4222` inside a container says nothing about what a
browser can reach. `websocket_urls` is served to the SPA at runtime by
`GET /api/client-config` (`hooks/client_config_routes.go`), deliberately not
baked in at build time: the UI is embedded in the binary, so a build-time
constant would mean a frontend rebuild per operator — the same problem
`branding.dir` exists to avoid.

The console resolves these URLs in three tiers, and a device override never
merges with the defaults. The reasons, and the other URL rules: `ui/CLAUDE.md`.

### Environment Variables
Prefix: `STONE_AGE_`
```bash
STONE_AGE_NATS_SERVER_URL="nats://localhost:4222"
STONE_AGE_AUDIT_LOG_TO_CONSOLE=true
```

### Config Priority (highest to lowest)
1. Environment variables (`STONE_AGE_*`) - override everything
2. CLI flag: `--config /path/to/config.yaml`
3. Current directory: `./config.yaml`
4. System: `/etc/stone-age/config.yaml`
5. Hardcoded defaults

## Code Conventions

The frontend (Vue/TypeScript) conventions are in `ui/CLAUDE.md`.

### Backend (Go)

**Hooks Pattern**:
```go
app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error { /* init */ })
app.OnRecordAfterCreateSuccess("collection").BindFunc(func(e *core.RecordEvent) error { /* provision */ })
```

**Logging**: Structured with emoji prefixes

## Roles & Authorization

**PocketBase API rules in `schema.json` are the only enforcement layer.** pb-nats
and pb-nebula contain no tenancy logic — they never reference `organization`. The
UI's capability map is navigation convenience, not a boundary.

Five roles on `memberships.role` (`invites.role` offers all but `owner`).
`viewer` is a tenant's read-only staff: the inventory screens, no write control
anywhere. `dashboard` is an appliance login for an unattended screen: the
Visualizer at `/` and its own `/settings`, nothing else. Neither restriction is a
NATS restriction — a console role's real capability on the bus is whatever its
linked `nats_users` role permits, which is set independently.

| | owner | admin | member | viewer | dashboard |
|---|:-:|:-:|:-:|:-:|:-:|
| Members, invitations | ✓ | ✓ | | | |
| NATS + Nebula infrastructure | ✓ | ✓ | | | |
| Thing/location types, operations, schemas | ✓ | ✓ | | | |
| JetStream streams, KV buckets | ✓ | ✓ | | | |
| Attach a NATS/Nebula identity to a Thing | ✓ | ✓ | | | |
| Delete a thing or location | ✓ | ✓ | | | |
| Deactivate a thing; reset a thing's password | ✓ | ✓ | | | |
| Things + locations: create and edit | ✓ | ✓ | ✓ | | |
| Things + locations: **browse in the console** | ✓ | ✓ | ✓ | ✓ | |
| Own NATS credential + rotation | ✓ | ✓ | ✓ | ✓ | ✓ |
| Dashboards | ✓ | ✓ | ✓ | ✓ | ✓ (its only screen) |

The browse row is the only one in this table that the API rules do **not**
enforce — see the read-scope note below. Every other row is a rule.

`owner` and `admin` are deliberately identical in the rules — the only difference
is that an owner cannot leave their own organization (`ui/src/stores/auth.ts`).
Editing the **organization record itself is a platform-operator action, not an
owner one**: it carries the tenancy flags (`managed`, `is_operator_org`,
`is_system_org`) and drives NATS account and Nebula CA provisioning, so no
tenant role has an update path to it — and, since
`schema_update_tenancy_sentinel.go`, no delete path either: deleting an
organization blanks rather than cascades, orphaning its entire inventory.

Rules to follow when touching authorization. Each is one line here. The
reasoning, and the incident that taught it, are in `AUTHORIZATION.md`.

- **Use an allowlist, never a deny-list.** Write `(role ?= "owner" || role ?= "admin")`,
  and copy the canonical snippet from a neighbouring rule.
- **Every write branch names its roles.** Restricting *fields* is not restricting
  *roles*.
- **A blank scope is a wildcard.** In PocketBase `'' = ''` is true. Ask what a
  rule does when both sides are the zero value.
- **Guard every path that writes `current_organization`**, on create as well as
  update. It is the read boundary.
- **No apostrophes in rule comments.** PocketBase tokenizes quotes before it
  strips comments, and the collection then fails to import.
- **Keep `dashboard`, the zero-authority role, as the denial probe** in
  `scripts/test-authz.sh`. `member` and `viewer` pass for the wrong reason.
- **Reads are org-scoped, not role-scoped, and that is deliberate.** `can.*` and
  the router are navigation, not a boundary.
- **Gate a write control on the capability its rule needs**, not the one its
  screen needs.
- **Hooks enforce invariants, rules enforce permissions, and routes do what a
  rule cannot.** `hooks/relation_tenancy.go` is the worked example of an
  invariant hook.
- **A route that writes with `app.Save()` bypasses every API rule.** Restate each
  check the rules would have made.
- **Credentials are protected by row scoping, not hidden fields.** Ask whether
  anything copies the record somewhere the row rules do not reach. That is why
  `auditSnapshotCollections` is an allowlist.
- **A gateway reads nothing in `nats_*` or `nebula_*`.** Extend
  `GET /api/me/leaf-config` instead of adding a read branch.
- **An empty NATS allow list grants nothing** (pb-nats v0.3.0 and later). Write
  full access as an explicit `>`.
- **An `authRule` is checked only at the auth endpoint, never on an existing
  token.** Disabling an identity also needs `RefreshTokenKey()`.
- **A flag is not a control unless something acts on it.** And `revoke` is not
  suspend.
- **Run `./scripts/test-authz.sh` after any rule change.** Pair every "cannot"
  with a "can" on the same record.

PocketBase traps that bite here (`hooks/CLAUDE.md`):
- A hook that returns without `e.Next()` silently drops every handler bound after it.
- `Record.Original()` is the state at the last database READ, not before this write.
- pb-nats trigger fields fire from `app.Save()` only because pb-nats binds them
  to the MODEL hook.

Schema traps (`migrations/CLAUDE.md`):
- A `schema.json` change needs a new `migrations/schema_update_*.go`. A new
  column under a live rule also needs a backfill.
- A re-import cannot remove a field.
- A re-import keeps the LIVE field when the name matches but the id does not.

UI side: the capability map lives in `ui/src/stores/auth.ts` (`can.*`); the router
guards on `meta.requiresCapability` and the sidebar hides what a role can't reach.
Keep it in step with the table above.

## Key Features

Features with a long note get one line here. The note lives where the code does.

1. **Authentication & Multi-Tenancy** - PocketBase auth, OAuth2, organization switching, RBAC
2. **Dashboard/Visualizer** - Grid-based dashboards, 16 widget types, variable substitution
3. **NATS Integration** - Account/User/Role provisioning, real-time WebSocket connection
4. **Nebula Networks** - Certificate Authority, network, and host management
5. **Resource Inventory** - Things and Locations with type definitions and metadata. `metadata_schema` is a form hint, not a contract (`ui/CLAUDE.md`)
6. **Digital Twin** - Live state via NATS KV buckets, revision history
7. **Audit Logging** - operator-only. Field values are recorded only for the
   collections in `auditSnapshotCollections` (`hooks/CLAUDE.md`)
7b. **Tenant activity feed** - org-scoped and carries no record values. Adding a
   collection to it is an authorization change (`hooks/CLAUDE.md`)
8. **Maps** - Leaflet maps over an OpenFreeMap vector basemap (WebGL), with floorplan overlays
9. **PWA** - Service worker, manifest, installable
10. **Keyboard Shortcuts** - Configurable keyboard shortcuts with modal reference
11. **Operator Org & Managed Orgs** - a `helpdesk.>` export for each managed org.
    Both records are read-only in the console (`hooks/CLAUDE.md`)
12. **Edge sites** - an edge site is a Thing, and the agent gets everything from
    `GET /api/me/leaf-config`. Liveness is asked of NATS, never stored (`hooks/CLAUDE.md`)
13. **Digital Twin / Live State** - the device writes `twin` and operators write
    `twin_desired`. Never merge them. Say `differs`, never `pending` (`ui/CLAUDE.md`)
14. **Readiness & metrics** - `GET /api/ready` and `/metrics`. A check must be
    answerable first-hand, and there are no per-org labels (`hooks/CLAUDE.md`)
15. **Decommissioning a device** - `things.active`. Deactivate, never delete (`hooks/CLAUDE.md`)
15b. **Rotating an organization's Nebula CA** - three steps, because the wait
    between them is the feature (`hooks/CLAUDE.md`)
15c. **The `/32` certificate defect** - nothing re-signs automatically. `renew`
    one host at a time (`hooks/CLAUDE.md`)
15d. **Suspending an organization** - `organizations.active` withdraws the NATS
    account and does nothing else (`hooks/CLAUDE.md`)
16. **Organization code** - ids for storage, codes for addressing. `code` is
    optional but immutable (`hooks/CLAUDE.md`)
16b. **Thing and Location codes** - generated on the server only, prefixed, with
    case-folded uniqueness (`hooks/CLAUDE.md`)
16c. **`locations.path`** - the hook is the only writer, and a move rewrites the
    subtree (`hooks/CLAUDE.md`)
17. **QR labels** - the payload is the bare code, and the label carries only the
    organization's own data (`ui/CLAUDE.md`)
18. **Uploaded files are protected, all of them** - every file field is
    `protected: true`, and the auth token is not a file token (`ui/CLAUDE.md`)
19. **Thing and location photos** - one photo, not a gallery, and `things.photo`
    is edit-only (`ui/CLAUDE.md`)

## Important Files

One line each. The `CLAUDE.md` in each file's directory says more.

- `main.go` - Backend entry, PocketBase setup, hooks, bootstrap command
- `hooks/leaf_config_routes.go` - `GET /api/me/leaf-config`: everything an edge agent needs
- `hooks/thing_routes.go` - `POST /api/org/things`: a Thing and its optional identities in one transaction
- `hooks/relation_tenancy.go` - the one enforcement hook: no relation may point into another organization
- `hooks/org_provisioning.go` - every organization's NATS account and Nebula CA
- `hooks/active_flag.go` / `hooks/org_active_flag.go` - device deactivation and org suspension. Read both before changing either
- `hooks/codes.go` / `hooks/location_path.go` - generated codes (ADR 0003) and `locations.path` (ADR 0004)
- `hooks/nebula_routes.go` - Nebula CA rotation and the `/32` certificate audit
- `hooks/activity.go` - the tenant activity feed
- `hooks/schema_fields.go` - the fields that exist only after `schema.json` is imported
- `hooks/client_config_routes.go` - `GET /api/client-config` (the NATS URLs a browser dials)
- `internal/health/`, `internal/metrics/`, `hooks/{observability,readiness,metrics,cert_expiry}.go` - readiness and metrics
- `ui/src/utils/fileToken.ts` / `ui/src/composables/useFileUrl.ts` - file tokens. Every uploaded file is protected
- `ui/src/stores/auth.ts` - Authentication and organization context
- `ui/src/stores/nats.ts` - NATS WebSocket connection manager
- `ui/src/stores/dashboard.ts` - Dashboard state and persistence
- `ui/src/router/index.ts` - Route definitions with auth guards
- `ui/src/types/pocketbase.ts` - All PocketBase record type interfaces
- `ui/src/types/dashboard.ts` - Widget and dashboard type definitions
- `ui/src/utils/pb.ts` - PocketBase client singleton
- `config.yaml` - Application configuration
- `schema.json` - PocketBase collection definitions
