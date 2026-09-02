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
- **Go 1.26.0** with PocketBase 0.39.11
- **Cobra** for CLI commands (bootstrap, NATS management)
- **Viper** for configuration management
- **Key libraries**: pb-audit, pb-tenancy, pb-nats, pb-nebula, nats-io/jwt, slackhq/nebula

### Frontend
- **Vue 3** (Composition API + `<script setup>`)
- **Vite 8** build tool (bundles with Rolldown, not Rollup)
- **Pinia 4** state management
- **Vue Router 5** routing
- **Tailwind CSS 3.4 + DaisyUI 4.12** styling (light/dark themes) — **pinned, see below**
- **TypeScript 6** (7 is out but unusable here, see below)
- **NATS WebSocket** (@nats-io/nats-core, jetstream, kv)
- **Leaflet 1.9** for maps — markers, clustering, floor-plan overlays — over a
  **MapLibre GL 5** vector basemap via `@maplibre/maplibre-gl-leaflet` (WebGL)
- **ECharts 6.1 + vue-echarts** for charts
- **grid-layout-plus** for dashboard grid layout
- **@vueuse/core** for reactive utilities
- **date-fns** for date formatting
- **jsonpath-plus** for JSON path extraction in widget data
- **marked** for Markdown rendering
- **PocketBase JS SDK** for API client

### Three dependencies are deliberately held back

Everything else in `ui/package.json` tracks latest. These three do not, and a
routine "bump everything" pass must not drag them along:

- **Tailwind 3.x + daisyUI 4.x.** Upgrading is a design-system migration, not a
  version bump: 376 references across 27 files use the daisyUI v4 form
  `oklch(var(--b1))`, and v5 both renames those vars and changes them from bare
  OKLCH components to complete color values, so every one becomes
  `oklch(oklch(...))` — invalid, and silently falls back. The alpha form
  `oklch(var(--bc) / 0.7)` needs `color-mix()` or relative color syntax, both of
  which `assets/dashboard-compat.css` already rejected for failing silently on
  older engines. The theme block is also copied **verbatim** into the
  access-control console, which is on tailwind ^3.4 / daisyui ^4.4, so migrating
  one repo alone breaks that contract. The full reasoning lives at the top of
  `ui/tailwind.config.js`; do it as scheduled work across both repos, with a
  human clicking through dashboards, widgets and both themes.
- **maplibre-gl 5, not 6.** v5 inlines its tile-parsing worker into
  `dist/maplibre-gl.js`. v6 splits it out and resolves it as a file sitting
  next to itself — `new URL('./maplibre-gl-worker.mjs', import.meta.url)` —
  which, once Vite has bundled the library into a hashed application chunk,
  points at `/assets/maplibre-gl-worker.mjs`, a file the build never emits.
  **Nothing throws and nothing reaches the console.** The style still loads over
  HTTP and its background layer still paints, so the map renders as a flat sheet
  of the theme colour; water, landuse, roads and labels are all parsed in the
  worker that never started, so they vanish together and it reads as a design
  choice rather than a failure. `setWorkerUrl()` with a `?url` import does fix
  it, and was tried, but it is a workaround for a problem v5 does not have —
  and v5 is also the version OpenFreeMap's own quick start pins, for both the
  plain and the Leaflet-binding setup. Revisit when
  `@maplibre/maplibre-gl-leaflet` documents v6 rather than merely permitting it
  in `peerDependencies`.
- **TypeScript 6, not 7.** TS 7 is the native port and no longer exposes the
  `./lib/tsc` subpath that `vue-tsc` resolves at startup, so `npm run build`
  dies before type checking. `vue-tsc` 3.3.10 is the newest there is; 6.0 is the
  ceiling until the Vue tooling catches up.

Neither has an automated guard — there is no frontend test runner, so
`vue-tsc && vite build` stays green while the UI renders wrong. That is exactly
why these are written down rather than left to be rediscovered.

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
│   │   │   ├── common/     # Reusable UI (ConfirmDialog, MetadataEditor, etc.)
│   │   │   ├── layout/     # AppHeader, AppSidebar, MainLayout
│   │   │   ├── ui/         # Base UI primitives (BaseCard, ResponsiveList)
│   │   │   ├── dashboard/  # Dashboard grid, widget containers, variables
│   │   │   │   └── config/ # 23 widget configuration form components
│   │   │   ├── widgets/    # 17 widget type components
│   │   │   │   └── map/    # Map marker sub-components (detail, kv, publish, switch, text)
│   │   │   ├── map/        # FloorPlanMap component
│   │   │   ├── nats/       # NATS-specific components (KvDashboard)
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
go build -o stone-age .

# Build the edge agent (separate lean binary — runs on edge boxes, not the server)
go build -o leaf-sync ./cmd/leaf-sync
# Release build with the version stamped in (surfaced by `leaf-sync --version` and in heartbeats):
go build -ldflags "-X platform/internal/version.Version=$(git describe --tags --always --dirty)" -o leaf-sync ./cmd/leaf-sync

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

The console resolves URLs in three tiers: device override (localStorage) →
deployment default (this key) → compiled-in `ws://localhost:9222`. Rules:

- **The override replaces the defaults; the two are never merged.** nats-core
  shuffles the server list by default (`noRandomize: false`), so a merged list
  is a pool picked at random, not a priority order — and the reason a device
  overrides is to reach its local leaf node instead of the hub, which are
  different JetStream domains holding different data under the same bucket
  names. Merging would make *which dataset you are looking at* a coin flip per
  reconnect.
- **Multiple entries mean one cluster.** Peers, not failover order. Do not list
  a hub URL and a leaf URL together.
- **No JetStream domain setting, deliberately.** The UI passes no domain, so
  plain `$JS.API` resolves to the JetStream of whichever server was dialed —
  hub URL → hub, leaf URL → that leaf's `edge-<code>`. The URL already selects
  the domain. A separate domain knob would be a second control that can
  disagree with the first, failing as an empty bucket list with no diagnosis.
  Cross-domain browsing ("read site S01's KV from the hub") is a per-view
  choice next to the bucket name, not a connection setting — and it needs
  publish rights on `$JS.<domain>.API.>`, which vary by `nats_roles`.
- **HTTPS pages cannot open `ws://`.** Browsers block it outright, so the
  settings form rejects it rather than saving a URL that can never connect.

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
2. **Dashboard/Visualizer** - Grid-based dashboards, 17 widget types, variable substitution
3. **NATS Integration** - Account/User/Role provisioning, real-time WebSocket connection
4. **Nebula Networks** - Certificate Authority, network, and host management
5. **Resource Inventory** - Things and Locations with type definitions and metadata
6. **Digital Twin** - Live state via NATS KV buckets, revision history
7. **Audit Logging** - Comprehensive audit trail with searchable viewer
8. **Maps** - Leaflet maps over an OpenFreeMap vector basemap (WebGL), with floorplan overlays
9. **PWA** - Service worker, manifest, installable
10. **Keyboard Shortcuts** - Configurable keyboard shortcuts with modal reference
11. **Operator Org & Managed Orgs** - Bootstrap creates the platform operator's own org (`is_operator_org`) alongside the `$SYS` org (`is_system_org`); its NATS account is the hub for shared operator services (helpdesk etc.). Flagging a customer org `managed` provisions a stream export of `helpdesk.>` (configurable: `nats.managed_export_subject`) from its account plus a hub-side import remapped to `helpdesk.{organizations.code}.>` — the org prefix is baked into the signed account JWT, so event provenance is subject-based and unforgeable (`hooks/managed_org_exports.go`). That token was `org.Id` until ADR 0002 (see **Organization code** below); `hubImportName(org.Id)` still keys the import *record* by the immutable id, which is correct and should stay. `ensureManagedExports` is no longer create-if-missing: it `reconcile`s the desired fields on an existing import, because a create-only hook would have left a renamed org's signed import routing at the old token while the consumer's `helpdesk.*.tickets.>` wildcard masked the failure — traffic matches, and never arrives.
12. **Edge / Leaf Nodes** - `leaf_nodes` auth collection (a "special thing" with one nats_user, server-provisioned). The `leaf-sync` agent runs on the edge, authenticates as the leaf node, and mirrors its org's config collections into a NATS leaf node's local JetStream KV. A leaf-node identity holds **no read grant on any `nats_*` or `nebula_*` collection**: `leaf-sync config` gets everything it needs from `GET /api/leaf/bootstrap`, which returns eight named fields (`domain`, `code`, `creds`, `account_jwt`, `account_pub`, `operator_jwt`, `sys_account_jwt`, `sys_account_pub`). `nats_system_operator` stays superuser-only; `GET /api/leaf/operator-jwt` remains as a superseded alias so upgrade order doesn't matter.
    - **A generated `nats-leaf.conf` must satisfy operator-mode validation, which no string assertion can check.** Two directives are mandatory and were both missing for months, so `leaf-sync config` produced a file `nats-server` refused to load — the failure was invisible because the only tests were `strings.Contains` over the output. (1) Every leaf remote needs an `account` key naming the local account; (2) `resolver_preload` needs the **`$SYS` account JWT** as well as the org's, because the operator JWT names a system account and `resolver: MEMORY` has nowhere to fetch it — without it the server dies with `error resolving system account: account missing` before JetStream starts. Preloading `$SYS`'s *account* JWT is public trust material and grants nothing; connecting as `$SYS` needs a `$SYS` **user** credential, which is never served. `TestBuildLeafConfIsAcceptedByNATSServer` now runs the real generator's output through `nats-server`'s own `ProcessConfigFile` + `NewServer` (no ports, no network) — keep it, and don't replace it with more `Contains` checks.
    - **`leaf-sync run --nats` runs the leaf node in-process** (`internal/leafsync/embedded.go`, reusing `internal/natsd`), off by default. Two consequences worth keeping: the server starts **before** PocketBase is touched, and a failed login then **retries** instead of exiting — exiting would take the bus down, and a supervisor cycling the pair through a WAN outage means devices reconnecting and JetStream recovering its store on a loop. Without `--nats` the old fail-fast behaviour stands, because the bus is another process. Cost: the binary goes ~12 MB → ~26 MB, since `nats-server` links in either way, plus ~3 MB for the Prometheus client behind `leaf-sync`'s `/metrics` (measured 25.8 → 28.9 MB). The Control Plane pays nothing for that second one — `slackhq/nebula` already linked `client_golang` in there. `leaf-sync` writes a best-effort liveness heartbeat into the hub's `leaf_status` KV (when `nats.hub_domain` is set); the UI reads it to show online/offline status on the leaf node list + detail views. Credentials are resettable by org Admins/Owners (collection `manageRule`) — `things` now carries the same `manageRule`, so a device's PocketBase password is recoverable too.
13. **Digital Twin / Live State** - **Two** KV buckets per org, split by owner:
    `twin` (reported — the device writes it, flows edge→hub) and `twin_desired`
    (desired — operators write it, flows hub→edge). Keys are
    `<kind>.<code>.<prop>` (`thing.S01.temp`); direction is the bucket, so keys
    carry no sync bookkeeping. Defined once in `ui/src/utils/twin.ts` and
    `internal/leafsync/twin.go` — keep the two retention configs in step, since
    both the console and `leaf-sync` create these buckets and whoever gets there
    first defines them. **Not** one bucket per location; the old `ui/README.md`
    claim to that effect described a design that was never built. The platform
    server **cannot** provision them: it holds the NATS operator (SYSTEM account
    only) and has no reach into an org's account. Creation is the console's
    Initialize button or `leaf-sync`.
    - **One writer per bucket is the whole safety property.** A single bucket
      written from both ends does not pick a loser on a conflict, it *oscillates*:
      two concurrent values swap across the link, then swap back, forever — ~170k
      writes to one key in 300ms, measured. Encoding the owner in the key
      (`thing.S01.state.temp`) was tried and reverted: same safety, but it taxes
      every key in firmware/rule-router/widgets and a mistyped segment silently
      never syncs. Don't merge the buckets.
    - **The twin view is `KvDashboard` with its `desiredBucket` prop set**, not a
      separate component. A bespoke card was built and deleted: it reimplemented
      tree/flat, filtering, history, the responsive detail drawer and the JSON
      parse hint, all worse. Adding twin behaviour to the browser costs one prop;
      without it the browser is unchanged for `NATS → KV Buckets`.
    - **Reported state is read-only in twin mode.** The edge overwrites it, so an
      edit button is a lie. The Desired tab is the writable half.
    - **A desired value is a partial assertion** (`twinDrift` in
      `ui/src/utils/twin.ts`): only the keys present in the desired value are
      checked, so extra fields in the reported object are ignored. Full-equality
      comparison rots — the day a device reports one new field, every assertion
      set months ago flips to "differs". Subset semantics are objects-only;
      arrays and scalars compare exactly. Values may be primitives or objects.
    - **Pair a desired key with an echo, never with a measurement.** Desired
      belongs on a property the device *echoes back* to acknowledge an
      instruction (`thing.S01.setpoint`, `.mode`) — those converge exactly, so
      equality is the right question and "differs" means "the device has not
      accepted my instruction". Desired `temp = 20` against reported
      `temp = 20.3` compares an instruction to a continuous reading; it differs
      forever, and no tolerance fixes it in general because the right tolerance
      varies by property, device and season. This is the rule that dissolves
      "what if desired is a range" — a range is a threshold over *reported*
      state, which is a rule-router rule, not a desired value.
    - **Do not add operators to `twinDrift`.** `{$gt: 30}`, then `{$between:
      [18,22]}`, then tolerances, and it is a rules engine inside a KV browser —
      and rule-router already is one, properly. If equality feels wrong for a
      key, the key is paired with a measurement instead of an echo; fix the
      pairing.
    - **Four jobs, four homes.** Reported state → `twin` KV. Setpoints and
      config → `twin_desired` KV (durable: a device booting after three days
      offline reads the current value from its local mirror). Commands
      ("reboot") → a NATS message on `cmd.>`, *not* a KV value, because a
      durable "reboot now" sitting in a bucket forever is a bug. Ranges,
      thresholds, alarms, hysteresis → a rule-router rule over `twin`. Rows
      three and four are the ones people try to cram into `twin_desired`.
    - **Say `differs`, never `pending`.** Nothing in this platform applies desired
      values to devices — no hook, no agent, no subscription. A "waiting for the
      device" message asserts a control loop that does not exist. Both readings of
      desired (a command to converge on, or an expected value to alarm on) are
      legitimate and the platform cannot tell which the customer means, so state
      the difference and predict nothing.
    - **Show the difference, not a word for it.** The drift indicator renders the
      values themselves — `"auto" → "manual"` in the row, a reported/desired
      column pair in the detail pane. "Differs on: mode" is the same width and
      sends the reader off to look both values up. `twin_desired` is a
      *delivery mechanism*, not a control: the platform's job ends when the
      value is readable in the edge's local KV. What consumes it is the
      integrator's firmware or rule-router, the same boundary as minting NATS
      creds and not caring what publishes with them.
    - **Edge sync is `internal/leafsync/twin.go`**, off by default
      (`twin.enabled`). `twin_desired` is a native JetStream **mirror** (one
      origin, N mirrors — no code in the data path, and it serves last-known
      values offline because the edge never writes it). `twin` needs the relay:
      aggregating N sites natively would need N sources all named `KV_twin`,
      which requires the server's internal `iname` that nats.go doesn't expose,
      so the alternative is `twin_<code>` at every edge and rule-router reading a
      different bucket name per site. Don't "finish the job" by making reported
      state a source without solving that.
14. **Readiness & metrics** - `GET /api/ready` (unauthenticated, 503/200) and
    `GET /metrics` (Prometheus, open by default) on the Control Plane;
    `/ready` + `/metrics` on `leaf-sync` behind `observability.addr`. Checks live
    in `internal/health`, exposition in `internal/metrics`, platform-specific
    parts in `hooks/observability.go` (one `RegisterObservability` call, one
    options struct) + `hooks/readiness.go` + `hooks/metrics.go`.
    - **A check must be answerable first-hand by the process running it.** This
      is the whole design constraint, and it is the NATS account boundary
      restated. The Control Plane holds the operator and `$SYS` and has **no user
      credential inside any organization's account**, so it cannot read `twin`,
      `twin_desired`, or the `leaf_status` heartbeats — the console can, because
      a browser connects as the logged-in user, and `leaf-sync` can, because it
      runs inside the account. Do not "improve" a metric by minting the platform
      a credential in a tenant's account: that turns a credential issuer into a
      data-plane participant in every tenant's bus. **No per-org labels**, for
      the same reason — with per-org data reduced to row counts, a tenant label
      would be a customer name on an inventory count.
    - **`stone_age_records{collection="leaf_nodes"}` counts leaf nodes
      CONFIGURED.** It is not availability and an alert on it can never fire.
      Per-site liveness is `leaf_sync_*` on the edge box. Say this in the HELP
      text of any metric that could be mistaken for a health signal.
    - **Four states, and only `fail` is unready.** `warn` is
      running-but-misconfigured (encryption off, no `websocket_urls`); failing on
      those would refuse to serve the stock dev deployment. `skipped` is "this
      check did not apply / could not look", and it ranks BELOW `ok` —
      `Registry.Run` seeds the worst-state from the results rather than from
      `StateOK`, or an all-skipped report reads as a clean bill of health.
      Likewise the leaf collector **omits** server-derived series when the
      monitoring port is down rather than emitting zeros.
    - **An islanded edge warns, it does not fail.** Local NATS still works and
      devices keep running against the mirrored config; that autonomy is why a
      leaf node exists, so 503 would invert the design.
    - **Don't publish a metric that cannot vary — but check the claim first.**
      `stone_age_nats_cluster_routes` was nearly cut on the grounds that
      `internal/natsd` "deliberately does not support clustering the Control
      Plane". That comment was wrong and nothing in the code ever enforced it:
      `Start` hands the parsed config straight to `nats-server`, so a `cluster`
      block is honoured like any other directive.
      `TestEmbeddedServerClustersWithAPeer` now pins it with two real peered
      servers. A comment asserting a limitation is not evidence of one.
    - **Don't add a check that cannot fail.** `serve` calls `RunAllMigrations`
      before it listens (PocketBase `apis/serve.go`), so a "migrations pending"
      check is always green — which is why `schema_version` looks for the
      opposite: applied migrations this binary does not know, i.e. a pb_data
      written by a NEWER build. `serve` cannot fix that one. The `schema` check
      survives the same test only because `initial_schema.go` returns nil when
      the embedded `SchemaJSON` is empty, marking itself applied.
    - **`nats_trust` is the check that earns the feature.** It connects with the
      `$SYS` creds from the database; a server whose `nats.conf` carries a stale
      operator JWT rejects it, which is otherwise invisible — every account claim
      fails, no org's account reaches the bus, and the console looks fine.
      Reachability (`DialInfo`, an unauthenticated INFO read) is deliberately a
      separate check, because "nothing listening" and "listening but does not
      trust us" have different fixes. It reads `creds_file`, not the seed:
      that column is not one at-rest encryption covers, so no decryption code is
      duplicated from pb-nats.
    - **The prober caches; the endpoint never runs checks.** A probe would
      otherwise trigger a NATS dial per request, and a slow check would read as
      unready and kill a healthy container. `OnRefresh` (every probe) feeds the
      metrics; `OnChange` (flips only) feeds the log — collapsing them makes the
      gauges stale or the log unreadable.
    - **`/metrics` is open by default and does not use PocketBase auth.** PB
      tokens are JWTs that expire and no scraper has a refresh flow, so it would
      take a custom sidecar to read a standard format. `metrics.token` is
      accepted as Bearer *or* Basic-with-any-username, which between them covers
      every scraper in use. `/api/ready` is unauthenticated and serves ONE body
      to everyone. A tiered version (names/states anonymously, detail and fixes
      for a superuser) was built and cut: `/metrics` is open by default and
      already publishes every check's state, so the withholding bought half a
      boundary in exchange for a second response shape and six authz checks.
      Closing these endpoints is a proxy's job.
    - **Use `client_golang`, don't hand-roll the text format.** It costs no
      binary size (slackhq/nebula already links it) and there is no scrape test
      in CI, so a malformed exposition would look fine and be unscrapeable —
      the same argument as `TestBuildLeafConfIsAcceptedByNATSServer`. The tests
      parse the output with Prometheus's own parser and run promlint over it.
15. **Decommissioning a device** - `things.active` / `leaf_nodes.active`, owner/admin only. The flag is enforced in three places at once, because any one of them alone is a half-measure: the `authRule` (`active = true`) blocks new logins, `hooks/active_flag.go` refreshes `tokenKey` so tokens already issued die immediately, and the same hook sets `revoke` on the linked `nats_user` so the signed NATS credential stops working. Reactivating sets `regenerate`, issuing a fresh credential — the old `.creds` stays dead, because the account JWT's revocation cutoff is permanent. Distinct from a leaf node's heartbeat status, which reports whether the edge box *is* connected, not whether it *may* connect.

16. **Organization code — the ecosystem's namespace root** (ADR 0002 in
    `platform-docs`). The rule is **ids for storage, codes for addressing**.
    `organizations.code` is the one *globally* unique identifier in the
    ecosystem; everything below it (`things`, `locations`, `thing_types`,
    `location_types`, `leaf_nodes`) is unique only within its org, which the
    `UNIQUE (organization, code) WHERE code != ''` partial indexes already
    enforce. Relation columns stay PocketBase ids — codes address, ids store.

    `hooks/org_code.go` derives a code from the name on create when one wasn't
    given (`Slugify`, max 31, `^[a-z0-9][a-z0-9-]{1,30}$`) and **refuses on
    collision rather than auto-suffixing** — an invented `acme-2` would be
    printed on labels and baked into signed JWTs before anyone noticed it was
    the wrong tenant. Errors go through `apis.NewBadRequestError`, because a
    plain `fmt.Errorf` from a hook reaches the client as
    `{"message":"Failed to create record."}` and the operator never learns which
    name collided. A leading digit is fine — `816tech` is a valid code; the
    pattern demanded a leading letter until an operator org named exactly that
    could not be migrated, and nothing downstream (NATS subject tokens,
    `edge-`-prefixed JetStream domains, KV bucket names, RFC 1123 hostname
    labels) justified the restriction.

    **Bootstrap derives the two infrastructure orgs' codes the same way**, from
    `--org-name` / `--operator-org`, falling back to `system` / `operator` only
    when the name yields nothing valid (`orgCodeFor` in `bootstrap.go`). It used
    to pin those two strings unconditionally, so bootstrap and
    `migrations/schema_update_org_code.go` disagreed about the same org: a fresh
    install of `--operator-org "816tech"` produced `operator` while the backfill
    migration produced `816tech`. One deployment, two codes, decided by install
    history — for a value that is immutable and printed on labels. The cost of
    the fix is that `system` and `operator` are no longer reserved by
    construction, which is acceptable because nothing resolves an org by them:
    they are written and never read.

    **`code` is optional but immutable.** Mutability was the disqualifier, not
    optionality: `@request.body.code:changed = false` freezes it on
    `organizations` and the four inventory collections. The sharp edge is that
    this is a hook and rule guard, so it binds the API and **not** a superuser
    editing in the PocketBase dashboard — and once codes are on stickers and in
    signed account JWTs, an edit is a site visit plus a re-signed export.

    `orgSlugFor` (`hooks/thing_routes.go`) returns this code rather than
    slugifying the mutable `name`. That was a second, independent instance of
    the same bug, and it yields the review question worth keeping: not *"is this
    identifier unique"* but **"whose database does this identifier belong to."**

17. **QR labels** (`ui/src/components/common/QrLabelModal.vue`) - print an
    operator-branded label from any thing or location that has a code. The
    payload is the **bare code** — no host, no org, no kind token. A sticker in
    a public hallway is an attacker-writable surface, so a URL payload would let
    a forged label send a person to arbitrary content; a bare in-system
    identifier means the worst case is resolving a different record inside an
    already-authenticated session. It also makes maximum error correction free:
    `DOOR-1` at EC level H is a 21×21 symbol where the URL form needs 41×41.

    Sizes are **millimetres**, not pixels — 2″ × 1″ and 4″ × 2″, the two that
    exist in both plain and UHF RFID stock — with the per-size `@page` box
    written imperatively, since `@page` cannot be interpolated from a template
    or a scoped style block. Both reserve a centred **RFID inlay keep-out** that
    the artwork straddles (QR one side, text the other), so one layout prints on
    either medium; the *RFID stock* toggle only reveals the reserved band for
    checking against a datasheet. Quiet zone is the spec's 4 modules. The
    human-readable code is not decoration — it is the path for the label that
    won't scan. The customer/org name is deliberately **not** printed: a tenant
    name beside a device naming convention is free reconnaissance.

    The same labels are scanned by `ScannerWidget` here and by the helpdesk's
    `/staff/scan`. Nothing fetches the decoded string as a destination, and
    there is deliberately **no resolver service**.

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
| Leaf nodes, JetStream streams, KV buckets | ✓ | ✓ | | | |
| Attach a NATS/Nebula identity to a Thing | ✓ | ✓ | | | |
| Delete a thing or location | ✓ | ✓ | | | |
| Deactivate a thing or leaf node; reset a thing's password | ✓ | ✓ | | | |
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
tenant role has an update path to it.

Rules to follow when touching authorization:

- **Use an allowlist, never a deny-list.** `role ?!= "member"` was satisfied by
  `dashboard` — the *least privileged* role — so its holders passed every admin
  check. Write `(role ?= "owner" || role ?= "admin")`. Copy the canonical snippet
  verbatim from a neighbouring rule; do not hand-write a variant.
- **Restricting fields is not restricting roles.** The same bug came back in a
  second costume: `things` create/update admitted a member through a branch that
  froze `nats_user`/`nebula_host` but named no role, and `locations` create/update
  had no role check at all — so `dashboard` satisfied both and could write
  inventory. A branch that constrains *what* may be written still has to say *who*
  may write it. Every write branch names its roles.
- **Keep a zero-authority role in the test matrix.** Both bugs above were caught
  by the same thing: a role that holds no capability at all, used as the probe in
  `scripts/test-authz.sh`. `dashboard` is that role. Don't "simplify" the suite by
  testing denials with `member` — a role with *some* authority cannot prove an
  allowlist, because it passes for the wrong reason. (This role was called `badge`
  until it was renamed in `migrations/schema_update_dashboard_role.go`; commits and
  PRs before that date say `badge` and mean this.) `viewer` cannot take over the
  job either: it holds read capability, so a denial it passes proves less. Two
  roles, two purposes — don't merge them to save an enum entry.
- **Reads are org-scoped, not role-scoped, and that is deliberate.** Every read
  rule on `things`, `locations`, `thing_types`, `location_types`,
  `message_schemas` and `leaf_nodes` is `organization = current_organization`
  with no role branch, so *every* role in an org — `dashboard` included — can
  `curl` the whole inventory. `viewer` therefore reads exactly what `member`
  reads; the difference between them is writes plus which screens
  `ui/src/router/index.ts` navigates to. Do not describe the console's
  navigation as a read boundary, and do not "tighten" a view for a role by
  editing `can.*` — that is theatre. A read that must actually be a boundary is
  a branch in `schema.json`, with the cost that comes with it: eight collections
  to keep in step, and a new failure mode where a relation expansion silently
  returns nothing.
- **A read-only role costs one enum entry, and that is the point.** `viewer` was
  added (`migrations/schema_update_viewer_role.go`) with **zero rule text
  changes**: a role value naming itself in no write branch is denied everywhere
  by construction. That is the dividend of the allowlist rule above, and it
  doubles as a test of it — if you ever find yourself editing a rule to keep a
  new read-only role *out*, that rule is a deny-list and it is the bug. The
  work of such a role is all in the UI: split the capability
  (`viewInventory` vs `manageInventory` in `ui/src/stores/auth.ts`), point
  list/detail routes at the read one and forms at the write one, and gate every
  create/edit/delete control in the views.
- **Gate a write control on the capability that matches its rule, not the one
  that matches its screen.** Adding `viewer` surfaced two buttons that had been
  wrong since before it existed: the Delete buttons on the Things and Locations
  lists, and on the Location detail view, were ungated, so a `member` saw a
  Delete the server then refused (`deleteRule` is owner/admin). Any control
  whose rule differs from its screen's entry capability needs its own `v-if`.
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
  rotation is `POST /api/me/nats-creds/rotate` (`hooks/credential_routes.go`) and
  account key management is `POST /api/org/nats-account/keys`
  (`hooks/nats_account_routes.go`), rather than update-rule branches: the
  alternative is `:isset = false` on every other field, which silently opens up
  when a field is added. Both routes take no record id — the target is derived from
  the caller's own identity or active organization — and a `switch` maps each
  action to exactly one field, rejecting anything unrecognised.
- **A route that writes with `app.Save()` bypasses every API rule.** So each check
  the rules would have made has to be restated in the route. `POST /api/org/things`
  (`hooks/thing_routes.go`) is the worked example: organization comes from the
  caller's own record and never the body, and a `link`ed `nats_user`/`nebula_host`
  is verified to belong to that organization — without that second check the route
  would be a cross-tenant credential-theft path, because a Thing may read the
  credential of its own linked identity. Rules protect the CRUD endpoints, not
  yours.
- **A rule cannot express two authority levels for one operation.** Creating a
  Thing is a member action; attaching an identity to it is not. `things.createRule`
  approximates this by freezing `nats_user`/`nebula_host` in the member branch, but
  a *provisioning* endpoint that mints those records cannot be expressed as a
  create rule at all — hence `POST /api/org/things`, which role-checks per section.
  It also replaced three unguarded client calls whose partial failure orphaned a
  signed NATS credential, and which never sent `active`, so every Thing the console
  created was locked out by `things.authRule`. When provisioning spans collections,
  it belongs in one transaction on the server. PocketBase defers
  `*AfterCreateSuccess` hooks to commit (`core/db.go`, `txInfo.OnComplete`), so a
  rollback means pb-nats never signed or published either.
- **UI capability gates must match what the reader can READ, not just write.**
  Members hold inventory rights but cannot read `nats_users` (beyond their own row),
  `nebula_hosts`, `nebula_networks` or `nats_roles`. A view that expands those
  relations gets nothing back, and a `v-else` reading "No NATS user linked" then
  states something false about a Thing that *is* linked. The relation **id** on the
  `things` record is readable by any org member, so "linked but not visible to you"
  and "not linked" stay distinguishable without any rule change — three states, not
  two. Same rule as the twin markers: show what you actually know.
- **pb-nats trigger fields only fire from a route if pb-nats watches them on the
  MODEL hook.** `regenerate`, `revoke`, `rotate_keys`, `add_signing_key` and
  `remove_signing_key` are all handled in `pb-nats internal/sync/manager.go`. Those
  handlers used to be bound to `OnRecordUpdateRequest`, which fires **only for REST
  requests** — so a route doing `app.Save()` persisted the flag and nothing acted on
  it, leaving it set to fire later on an unrelated update. They are now on
  `OnRecordUpdate`. If a trigger-setting route ever appears to do nothing, check
  which hook the library binds before debugging the route.
- **`nats_users.publish_permissions` is copied verbatim into the signed JWT**
  (pb-nats `internal/jwt/generator.go`). Write access to that collection is
  equivalent to granting NATS permissions, so it is owner/admin only.
- **An `authRule` is checked at the auth endpoint only, never on an existing
  token.** PocketBase evaluates it in `apis.RecordAuthResponse`
  (`apis/record_helpers.go`), reached from `/auth-with-password` and friends —
  not in the middleware that loads a bearer token. `things` and `leaf_nodes` set
  `authToken.duration` to 7 days, so `active = true` on its own would leave a
  deactivated device with a working session for a week. `hooks/active_flag.go`
  calls `RefreshTokenKey()` on the true→false flip, which invalidates every
  outstanding token at once. Any future "disable this identity" feature needs
  the same pairing; the rule alone is a latch, not a switch.
- **A flag is not a control unless something acts on it.** `nats_users.active` is
  read into pb-nats's model (`internal/types/converters.go`) and consulted by
  *nothing* in JWT generation or sync — only `revoke`, which adds the public key
  to the account's revocation list and re-signs the account JWT
  (`internal/sync/manager.go`, `revokeUser`), actually disconnects anyone. The UI
  used to expose `active` as an editable checkbox next to a red/green badge, so
  an admin could "deactivate" a device that kept publishing. That checkbox is
  gone; Revoke/Re-enable on the detail view are the real controls. `things.active`
  and `leaf_nodes.active` exist only because `hooks/active_flag.go` gives them
  teeth — the flag, the token kill, and the NATS revoke are one operation. Do not
  add a status field to a device without deciding what enforces it.
- **A device's real capability is its NATS credential, not its PocketBase
  session.** Anything that takes a Thing or leaf node out of service has to reach
  `nats_users`, or it has only closed the console door.
- **Schema changes need a new `migrations/schema_update_*.go`** — editing
  `schema.json` alone reaches fresh databases only. **A new non-null column with
  a live rule over it needs a backfill in the same migration**: PocketBase bools
  have no schema default, so existing rows land as `false`, and importing
  `authRule: "active = true"` without the `UPDATE` in
  `schema_update_device_active_flag.go` would lock every already-provisioned
  device out of the API on deploy.
- **A `schema.json` re-import silently keeps the LIVE field when a name matches
  and the id does not.** `core.ImportCollections` re-adds each existing field
  the import did not carry by id, and `FieldsList.add` then replaces the
  imported definition with the existing one — so the migration logs ✅ and
  changes nothing. Since every `schema_update_*.go` here is "re-import
  schema.json", this turns a whole class of migration into a no-op. It has bitten
  twice: `e72557f` (pb-nats recreated `account_id` as text) and the 25
  hand-written ids on `nats_account_exports`/`nats_account_imports`, whose
  definitions had never been applied anywhere. **Use PocketBase's deterministic
  id — `<type>` + crc32 of the field name — for anything a library also creates,
  and diff a real install before trusting that a re-import migration did what it
  says.** Only field definitions freeze this way; collection rules are applied
  normally, which is why those two collections' org-scoped rules are live while
  their field definitions were inert. Changing an id is safe: PocketBase reuses
  the existing field object rather than recreating the column.
- **Run `./scripts/test-authz.sh` after any rule change** and add a check. Pair
  every "cannot" with a "can" on the same record, or a blanket deny passes.
- **Capture "before" state immediately before the action it belongs to.** A check
  that an operation had an effect is worthless if anything between the two reads
  could have caused it. `rotation actually re-minted the credential` passed for
  months against a route that did nothing, because its baseline was captured a
  section earlier and an intervening `publish_permissions` PATCH re-minted the
  credential as a side effect. Side-effect assertions need a baseline read on the
  line above the call.

UI side: the capability map lives in `ui/src/stores/auth.ts` (`can.*`); the router
guards on `meta.requiresCapability` and the sidebar hides what a role can't reach.
Keep it in step with the table above.

## Testing

- `go test ./...` — Go unit tests (`internal/leafsync` has the bulk of them).
  Two habits worth keeping: the readiness checks that touch NATS are tested
  against a **real operator-mode `nats-server`** built in the test (see
  `internal/health/nats_test.go`), because the thing being asserted IS the
  server's trust decision and a mock only asserts what we believe it to be; and
  the metrics exposition is parsed by **Prometheus's own parser and promlint**
  rather than string-matched, because nothing in CI scrapes it and a malformed
  body looks fine in a terminal.
- `./scripts/test-authz.sh` — **run after any API-rule change in `schema.json`.**
  Builds the binary, stands up a throwaway DB, and asserts 150 authorization
  behaviours against a live server. The rules are the only tenancy enforcement
  in the platform and nothing else type-checks them. Add a check when you add a
  rule, and bump `EXPECTED_CHECKS`. Note PocketBase answers 404 (not 403) when an
  update rule rejects, and 400 on a denied create — which is why every "cannot"
  is paired with a "can" on the same record.
- `./scripts/check-sort-fields.sh` — asks a real server about every `sortable:`
  term on a list-view column, which is the only thing that checks them. Same
  shape as the authz script: build, throwaway database, probe, tear down. The
  probes are deliberately unauthenticated — PocketBase validates the sort
  expression after building the list rule but before applying one, so a guest
  gets the same 400 for a bad field that a member would, and the check needs no
  fixtures or login to fail for the right reason. Run it when you add a
  `sortable:` column. It covers the sort half of the next bullet only; `filter`
  and `expand` terms are still unchecked.
- **An unknown `sort` or `filter` field is a 400 before any rule is evaluated.**
  `apis.recordsList` hands the query to `searchProvider.ParseAndExec` and returns
  `BadRequestError("", err)`, which reaches the browser as only "Something went
  wrong while processing your request." — no field name, and it fails for
  superusers too, so it never looks like authorization. Members and Invitations
  sorted by a `created` that `memberships` and `invites` did not have, and both
  screens were dead for every caller. Nothing type-checks a query string against
  `schema.json`, so **check sort and filter terms against the collection when you
  touch a list view** — the fix that caused this verified its generated filter
  fields and not its sort terms. `expand` also 400s but by a different route,
  after the query, in `apis.expandFetch`: a relation whose TARGET collection has
  a nil `viewRule` is an error, not an empty expansion, so making a collection
  superuser-only breaks every list view that expands a relation into it.
- Frontend has no test runner; development relies on HMR (`npm run dev`),
  browser DevTools, and the PocketBase admin panel at `/_/`

## Important Files

- `main.go` - Backend entry, PocketBase setup, hooks, bootstrap command
- `hooks/leaf_node_provisioning.go` - Mints a leaf node's NATS user on create
- `hooks/leaf_node_routes.go` - `GET /api/leaf/bootstrap` (leaf-node-authed; everything `leaf-sync config` needs, including the `$SYS` account JWT the leaf's MEMORY resolver cannot fetch), plus the superseded `GET /api/leaf/operator-jwt`
- `internal/leafsync/embedded.go` - `leaf-sync run --nats`: the edge's leaf node inside the agent process, via `internal/natsd`
- `hooks/thing_routes.go` - `POST /api/org/things`: Thing + optional NATS/Nebula identity in one transaction; member-level for inventory, owner/admin for the identity half
- `hooks/client_config_routes.go` - `GET /api/client-config`: deployment facts the SPA cannot be compiled with (browser-facing NATS WebSocket URLs). Authed (`users`) — there is no pre-login need, so no reason to publish the bus address
- `internal/health/` - readiness engine shared by both binaries: check registry, background prober, and the unauthenticated NATS reachability (`DialInfo`) + operator-trust (`CheckCreds`) probes
- `internal/metrics/` - Prometheus exposition shared by both binaries, plus the optional Bearer/Basic scrape token
- `hooks/observability.go` + `hooks/readiness.go` + `hooks/metrics.go` - the Control Plane's `/api/ready` + `/metrics` routes, its checks, and its collectors
- `internal/leafsync/observe.go` - the edge's own `/ready` + `/metrics`, reading the leaf's loopback monitoring port; the only place per-site health is actually visible
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
