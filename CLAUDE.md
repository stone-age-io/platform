# CLAUDE.md - Stone Age IoT Platform

## Project Overview

Stone Age IoT Platform is a single-binary IoT and Event-Driven management platform combining:
- Go backend extending PocketBase
- Embedded Vue 3 frontend
- SQLite database
- NATS messaging infrastructure integration
- Nebula overlay network management
- Edge nodes: a NATS leaf node + local KV mirror of an org's config, kept in sync by the `leaf-sync` agent

## Tech Stack

### Backend
- **Go 1.25.0** with PocketBase 0.38.0
- **Cobra** for CLI commands (bootstrap, NATS management)
- **Viper** for configuration management
- **Key libraries**: pb-audit, pb-tenancy, pb-nats, pb-nebula, nats-io/jwt, slackhq/nebula

### Frontend
- **Vue 3** (Composition API + `<script setup>`)
- **Vite 6.0** build tool
- **Pinia 2.1** state management
- **Vue Router 4.2** routing
- **Tailwind CSS 3.4 + DaisyUI 4.4** styling (light/dark themes)
- **TypeScript 5.3**
- **NATS WebSocket** (@nats-io/nats-core, jetstream, kv)
- **Leaflet 1.9** for maps
- **ECharts 5.5 + vue-echarts** for charts
- **grid-layout-plus** for dashboard grid layout
- **@vueuse/core** for reactive utilities
- **date-fns** for date formatting
- **jsonpath-plus** for JSON path extraction in widget data
- **marked** for Markdown rendering
- **PocketBase JS SDK** for API client

### Database
- SQLite (managed by PocketBase, stored in `pb_data/`)

## Directory Structure

```
platform/
├── main.go                 # Backend entry point
├── go.mod, go.sum          # Go dependencies
├── config.yaml             # Application configuration
├── schema.json             # PocketBase collection schema
├── hooks/                  # Server-side hooks: org + leaf-node provisioning, operator-jwt route
├── migrations/             # Schema migrations (re-import schema.json)
├── cmd/leaf-sync/          # Edge agent binary (separate from the server; PocketBase → local NATS KV)
├── internal/leafsync/      # leaf-sync engine (pbclient, sync, kv, bootstrap)
├── ui/                     # Frontend Vue 3 application
│   ├── public/             # PWA assets (manifest, service worker, icons)
│   ├── src/
│   │   ├── main.ts         # Vue app initialization
│   │   ├── App.vue         # Root component
│   │   ├── components/     # UI components
│   │   │   ├── common/     # Reusable UI (ConfirmDialog, ErrorBoundary, etc.)
│   │   │   ├── layout/     # AppHeader, AppSidebar, MainLayout
│   │   │   ├── ui/         # Base UI primitives (BaseCard, ResponsiveList)
│   │   │   ├── dashboard/  # Dashboard grid, widget containers, variables
│   │   │   │   └── config/ # 18 widget configuration form components
│   │   │   ├── widgets/    # 14 widget type components
│   │   │   │   └── map/    # Map marker sub-components (detail, kv, publish, switch, text)
│   │   │   ├── map/        # FloorPlanMap component
│   │   │   ├── nats/       # NATS-specific components (KvDashboard, LiveMessageStream)
│   │   │   └── locations/  # Location map visualization
│   │   ├── composables/    # Vue composables (useNatsKv, useLeafletMap, etc.)
│   │   ├── stores/         # Pinia stores (auth, dashboard, nats, ui, widgetData)
│   │   ├── router/         # Vue Router with auth guards
│   │   ├── types/          # TypeScript interfaces
│   │   ├── utils/          # Utility functions
│   │   └── views/          # Route page components
│   ├── vite.config.ts      # Vite configuration
│   ├── tailwind.config.js  # Tailwind configuration
│   └── package.json        # Frontend dependencies
├── pb_public/              # Compiled frontend (generated)
└── pb_data/                # Runtime database (generated)
```

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
Three commands, in this order — the order is load-bearing:
```bash
./stone-age superuser upsert admin@example.com 'password'   # PB superuser + NATS $SYS seed
./stone-age migrate up                                      # import schema.json
./stone-age bootstrap --email admin@example.com --org "System" --operator-org "816tech"
```
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
go build -o stone-age main.go

# Build the edge agent (separate lean binary — runs on edge boxes, not the server)
go build -o leaf-sync ./cmd/leaf-sync
# Release build with the version stamped in (surfaced by `leaf-sync --version` and in heartbeats):
go build -ldflags "-X platform/internal/leafsync.Version=$(git describe --tags --always --dirty)" -o leaf-sync ./cmd/leaf-sync

# Run
./stone-age serve
```

See [cmd/leaf-sync/README.md](cmd/leaf-sync/README.md) for the edge deployment flow.

### NPM Scripts
- `npm run dev` - Vite dev server with HMR
- `npm run build` - TypeScript check + production build to `../pb_public`
- `npm run preview` - Preview production build

## Configuration

### config.yaml
Located at project root. Key sections:
- `tenancy` - Multi-tenant settings (collections, invite expiry)
- `nats` - NATS server URL, operator name, default limits
- `nebula` - Nebula CA/network/host settings
- `audit` - Audit logging configuration

### Environment Variables
Prefix: `STONE_AGE_`
```bash
STONE_AGE_NATS_SERVER_URL="nats://localhost:4422"
STONE_AGE_TENANCY_LOG_TO_CONSOLE=true
```

### Config Priority (highest to lowest)
1. Environment variables (`STONE_AGE_*`) - override everything
2. CLI flag: `--config /path/to/config.yaml`
3. Current directory: `./config.yaml`
4. System: `/etc/stone-age/config.yaml`
5. Hardcoded defaults

## Code Conventions

### Frontend (Vue/TypeScript)

**Components**: Use Composition API with `<script setup lang="ts">`
```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
const items = ref([])
</script>
```

**Pinia Stores**: Composition style with `defineStore`
```typescript
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = computed(() => !!user.value)
  return { user, isAuthenticated }
})
```

**Composables**: `use` prefix, handle lifecycle cleanup
```typescript
export function useNatsKv() {
  onMounted(() => { /* setup */ })
  onUnmounted(() => { /* cleanup */ })
}
```

**Naming**:
- Components: PascalCase (`ConfirmDialog.vue`)
- Composables: camelCase with `use` prefix (`useNatsKv.ts`)
- Stores: camelCase (`auth.ts`)
- Views: End with `View` (`LoginView.vue`)

**Type Safety**: All types in `src/types/`, strict TypeScript enabled

### Backend (Go)

**Hooks Pattern**:
```go
app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error { /* init */ })
app.OnRecordAfterCreateSuccess("collection").BindFunc(func(e *core.RecordEvent) error { /* provision */ })
```

**Logging**: Structured with emoji prefixes

## Key Features

1. **Authentication & Multi-Tenancy** - PocketBase auth, OAuth2, organization switching, RBAC
2. **Dashboard/Visualizer** - Grid-based dashboards, 14 widget types, variable substitution
3. **NATS Integration** - Account/User/Role provisioning, real-time WebSocket connection
4. **Nebula Networks** - Certificate Authority, network, and host management
5. **Resource Inventory** - Things and Locations with type definitions and metadata
6. **Digital Twin** - Live state via NATS KV buckets, revision history
7. **Audit Logging** - Comprehensive audit trail with searchable viewer
8. **Maps** - Leaflet-based maps with floorplan overlays
9. **PWA** - Service worker, manifest, installable
10. **Keyboard Shortcuts** - Configurable keyboard shortcuts with modal reference
11. **Operator Org & Managed Orgs** - Bootstrap creates the platform operator's own org (`is_operator_org`) alongside the `$SYS` org (`is_system_org`); its NATS account is the hub for shared operator services (helpdesk etc.). Flagging a customer org `managed` provisions a stream export of `helpdesk.>` (configurable: `nats.managed_export_subject`) from its account plus a hub-side import remapped to `helpdesk.{orgId}.>` — the org prefix is baked into the signed account JWT, so event provenance is subject-based and unforgeable (`hooks/managed_org_exports.go`).
12. **Edge / Leaf Nodes** - `leaf_nodes` auth collection (a "special thing" with one nats_user, server-provisioned). The `leaf-sync` agent runs on the edge, authenticates as the leaf node, and mirrors its org's config collections into a NATS leaf node's local JetStream KV. A leaf-node identity holds **no read grant on any `nats_*` or `nebula_*` collection**: `leaf-sync config` gets everything it needs from `GET /api/leaf/bootstrap`, which returns six named fields (`domain`, `code`, `creds`, `account_jwt`, `account_pub`, `operator_jwt`). `nats_system_operator` stays superuser-only; `GET /api/leaf/operator-jwt` remains as a superseded alias so upgrade order doesn't matter. `leaf-sync` writes a best-effort liveness heartbeat into the hub's `leaf_status` KV (when `nats.hub_domain` is set); the UI reads it to show online/offline status on the leaf node list + detail views. Credentials are resettable by org Admins/Owners (collection `manageRule`).

## Roles & Authorization

**PocketBase API rules in `schema.json` are the only enforcement layer.** pb-nats
and pb-nebula contain no tenancy logic — they never reference `organization`. The
UI's capability map is navigation convenience, not a boundary.

Four roles on `memberships.role` (`invites.role` offers all but `owner`):

| | owner | admin | member | badge |
|---|:-:|:-:|:-:|:-:|
| Members, invitations | ✓ | ✓ | | |
| NATS + Nebula infrastructure | ✓ | ✓ | | |
| Thing/location types, operations, schemas | ✓ | ✓ | | |
| Leaf nodes, JetStream streams, KV buckets | ✓ | ✓ | | |
| Attach a NATS/Nebula identity to a Thing | ✓ | ✓ | | |
| Delete a thing or location | ✓ | ✓ | | |
| Things + locations: create and edit | ✓ | ✓ | ✓ | |
| Own NATS credential + rotation | ✓ | ✓ | ✓ | ✓ |
| Dashboards | ✓ | ✓ | ✓ | badge routes only |

`owner` and `admin` are deliberately identical in the rules — the only difference
is that an owner cannot leave their own organization (`ui/src/stores/auth.ts`).
Editing the **organization record itself is a platform-operator action, not an
owner one**: it carries the tenancy flags (`managed`, `is_operator_org`,
`is_system_org`) and drives NATS account and Nebula CA provisioning, so no
tenant role has an update path to it.

Rules to follow when touching authorization:

- **Use an allowlist, never a deny-list.** `role ?!= "member"` was satisfied by
  `badge` — the *most* restricted role — so badge holders passed every admin
  check. Write `(role ?= "owner" || role ?= "admin")`. Copy the canonical snippet
  verbatim from a neighbouring rule; do not hand-write a variant.
- **Restricting fields is not restricting roles.** The same bug came back in a
  second costume: `things` create/update admitted a member through a branch that
  froze `nats_user`/`nebula_host` but named no role, and `locations` create/update
  had no role check at all — so `badge` satisfied both and could write inventory.
  A branch that constrains *what* may be written still has to say *who* may write
  it. Every write branch names its roles.
- **`?`-prefixed operators are row-correlated** on the same relation path, so
  `memberships_via_user.organization ?= X && memberships_via_user.role ?= "owner"`
  matches one membership row. Without `?`, the condition must hold for *all*
  related rows.
- **Credentials are protected by row scoping, not hidden fields.**
  `nats_users.creds_file` and `nebula_hosts.config_yaml` stay readable because the
  identity that owns them needs them (the browser's NATS connection and the admin
  download button). The read rules restrict *which rows* a caller sees. Do not add
  `hidden: true` to them — it breaks both and buys nothing.
- **A leaf node reads nothing in `nats_*` or `nebula_*`.** `leaf-sync config` gets
  its creds, the account JWT, and the operator JWT from `GET /api/leaf/bootstrap`
  (`hooks/leaf_node_routes.go`), which reads those records with the app's own
  privileges and returns six named fields. Don't re-add a leaf-node read branch to
  those collections to make some edge feature work — extend the route instead. The
  point is that the edge's blast radius is a fixed list rather than a consequence
  of rules that change for unrelated reasons.
- **A rule cannot express a single-field allowlist.** That is why self-service
  rotation is `POST /api/me/nats-creds/rotate` (`hooks/credential_routes.go`)
  rather than an update-rule branch: the alternative is `:isset = false` on every
  other field, which silently opens up when a field is added.
- **`nats_users.publish_permissions` is copied verbatim into the signed JWT**
  (pb-nats `internal/jwt/generator.go`). Write access to that collection is
  equivalent to granting NATS permissions, so it is owner/admin only.
- **Schema changes need a new `migrations/schema_update_*.go`** — editing
  `schema.json` alone reaches fresh databases only.
- **Run `./scripts/test-authz.sh` after any rule change** and add a check. Pair
  every "cannot" with a "can" on the same record, or a blanket deny passes.

UI side: the capability map lives in `ui/src/stores/auth.ts` (`can.*`); the router
guards on `meta.requiresCapability` and the sidebar hides what a role can't reach.
Keep it in step with the table above.

## Testing

- `go test ./...` — Go unit tests (`internal/leafsync` has the bulk of them)
- `./scripts/test-authz.sh` — **run after any API-rule change in `schema.json`.**
  Builds the binary, stands up a throwaway DB, and asserts 68 authorization
  behaviours against a live server. The rules are the only tenancy enforcement
  in the platform and nothing else type-checks them. Add a check when you add a
  rule, and bump `EXPECTED_CHECKS`. Note PocketBase answers 404 (not 403) when an
  update rule rejects, and 400 on a denied create — which is why every "cannot"
  is paired with a "can" on the same record.
- Frontend has no test runner; development relies on HMR (`npm run dev`),
  browser DevTools, and the PocketBase admin panel at `/_/`

## Important Files

- `main.go` - Backend entry, PocketBase setup, hooks, bootstrap command
- `hooks/leaf_node_provisioning.go` - Mints a leaf node's NATS user on create
- `hooks/leaf_node_routes.go` - `GET /api/leaf/bootstrap` (leaf-node-authed; everything `leaf-sync config` needs), plus the superseded `GET /api/leaf/operator-jwt`
- `cmd/leaf-sync/` + `internal/leafsync/` - Edge agent (config bootstrap + KV sync); see `cmd/leaf-sync/README.md`
- `ui/src/stores/auth.ts` - Authentication and organization context
- `ui/src/stores/nats.ts` - NATS WebSocket connection manager
- `ui/src/stores/dashboard.ts` - Dashboard state and persistence
- `ui/src/router/index.ts` - Route definitions with auth guards
- `ui/src/types/pocketbase.ts` - All PocketBase record type interfaces
- `ui/src/types/dashboard.ts` - Widget and dashboard type definitions
- `ui/src/utils/pb.ts` - PocketBase client singleton
- `config.yaml` - Application configuration
- `schema.json` - PocketBase collection definitions
