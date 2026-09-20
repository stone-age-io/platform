# CLAUDE.md - Stone Age IoT Platform

## Project Overview

Stone Age IoT Platform is a single-binary IoT and Event-Driven management platform combining:
- Go backend extending PocketBase
- Embedded Vue 3 frontend
- SQLite database
- NATS messaging infrastructure integration
- Nebula overlay network management
- Edge sites: a NATS leaf node run by the Agent (a separate repo), bootstrapped from `GET /api/me/leaf-config`

## Tech Stack

### Backend
- **Go 1.26.0** with PocketBase 0.39.11
- **Cobra** for CLI commands (bootstrap, NATS management)
- **Viper** for configuration management
- **Key libraries**: pb-audit, pb-nats, pb-nebula, nats-io/jwt, slackhq/nebula (tenancy was pb-tenancy until it was absorbed into `hooks/`)

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

  **Staying on v5 means carrying GHSA-jrc7-96c5-q579 unpatched** (critical; the
  `DOM.sanitize()` bypass, fixed in 6.4.1 and never backported — 5.24.0 is the
  last 5.x there will be). It is not reachable here, and the reason is narrow
  enough to be worth writing down: the only sink is MapLibre's own attribution
  control, and that control is never constructed, because
  `@maplibre/maplibre-gl-leaflet` hardcodes `attributionControl: false` when it
  builds the `maplibregl.Map` and `useLeafletMap.ts` passes the same. The credit
  on screen is **Leaflet's** control, fed by the `TILE_ATTRIBUTION` constant.
  Two changes would end that: turning the MapLibre control on, or making
  `STYLE_URLS` configurable — the Leaflet binding lifts a style's source
  `attribution` into Leaflet's control, which assigns to `innerHTML` with no
  sanitizing at all, so a hostile style document would get a cleaner path than
  the advisory describes and upgrading maplibre would not close it. Both are
  commented at the call site.
- **TypeScript 6, not 7.** TS 7 is the native port and no longer exposes the
  `./lib/tsc` subpath that `vue-tsc` resolves at startup, so `npm run build`
  dies before type checking. `vue-tsc` 3.3.10 is the newest there is; 6.0 is the
  ceiling until the Vue tooling catches up.

The only automated guard is the `Assert the deliberately pinned majors` step in
`.github/workflows/ci.yml`, which fails the build if any of the four package
majors moves. That catches the upgrade and nothing else: Vitest covers pure logic
only (see **Testing**), so nothing exercises a rendered map, a theme token or a
built bundle — `vue-tsc && vite build` stays green while the UI renders wrong,
whatever the versions say. That is exactly why these are written down rather than
left to be rediscovered.

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
│   │   │   │   └── config/ # 16 per-widget config panels + shared editors
│   │   │   ├── widgets/    # 16 widget type components
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
  hub URL → hub, leaf URL → that leaf's domain, which is its Thing code. The URL already selects
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
2. **Dashboard/Visualizer** - Grid-based dashboards, 16 widget types, variable substitution
3. **NATS Integration** - Account/User/Role provisioning, real-time WebSocket connection
4. **Nebula Networks** - Certificate Authority, network, and host management
5. **Resource Inventory** - Things and Locations with type definitions and metadata
6. **Digital Twin** - Live state via NATS KV buckets, revision history
7. **Audit Logging** - Comprehensive audit trail with searchable viewer
    (operator-only: `audit_logs` has no `organization` column, so there is
    nothing to scope a tenant read on). Every event records `changed_fields`;
    full before/after values only for the collections in
    `auditSnapshotCollections` (`main.go`). See the credential bullet under
    **Roles & Authorization** for why that list is short and why it is an
    allowlist.
7b. **Tenant activity feed** (`activity`) - who changed what in the console,
    org-scoped and readable by every role. NOT `audit_logs`, which stays
    operator-only: this carries no record values at all, only actor, action,
    resource and snapshotted labels.
    - **The invariant: an entry is visible to exactly those who could read the
      record it describes.** A flat collection mirroring other collections
      inherits none of their scoping — the lesson `audit_logs` taught. So the
      feed covers only the five tenant collections whose own reads are
      org-scoped with no role branch (`things`, `locations`, `thing_types`,
      `location_types`, `thing_type_operations`) and NOT `memberships`,
      `invites`, `nats_roles` or `nebula_networks`, whose reads stop at
      owner/admin. Adding to `activityCollections` is an authorization change,
      pinned by `hooks/activity_internal_test.go`.
    - **Bound to the REQUEST hooks, because they are the only layer that knows
      the actor.** `core.RecordEvent` carries a `Context`, but nothing in
      PocketBase ever puts auth into it — the only `context.WithValue` calls in
      the framework are its own tests, which is why pb-audit passes `nil` for
      request info on its success hooks.
    - **The id is the one field captured AFTER `e.Next()`**: a new record has no
      id until PocketBase assigns it during the save. Everything else is the
      pre-write snapshot, so an entry describes what the request asked for
      rather than what the request plus every server-side hook did.
    - `actor` is plain text, not a relation: a non-cascade relation is *blanked*
      on user delete, one write per row. Labels are snapshots for the same
      reason — a log that changes when a record is renamed is not a log.
    - Write rules are all `nil`, so the log cannot be forged or edited through
      the API. Deliberately no changed-field list, no retention cron and no
      transaction dance — `hooks/activity.go` says why.
    - **The console's "Open record" button is gated on the capability the
      TARGET route needs, not the one that opened the feed.** Every role in an
      org reads the feed, but the three type collections have no detail view —
      their only route is the edit form behind `manageDefinitions` — so an
      ungated link would bounce a `viewer` to `/`. `RESOURCE_ROUTES` in
      `ui/src/views/activity/ActivityView.vue` maps each snapshotted noun to a
      route plus its capability, and `TestActivityNounsAllHaveAConsoleRoute`
      reads that file so adding a sixth collection to `activityCollections`
      cannot silently ship entries with no way to reach the record. Nothing
      in the dialog is a live join: the label and the noun are both snapshots,
      which the dialog says out loud so an accurate feed does not read as a
      stale one.
    - **Both timestamps on every covered entity's detail surface.** A feed that
      reports an update is only useful beside a record that admits when it
      changed, so `RecordTimestamps.vue` (relative headline, exact underneath —
      the same shape the feed uses) is on the Thing and Location detail views
      and on the three type FORM views, which are those collections' only
      detail surface. No "never edited" marker: PocketBase's autodate fields
      each call `types.NowDateTime()` separately, so `created === updated` on
      an untouched record is a coin flip.

8. **Maps** - Leaflet maps over an OpenFreeMap vector basemap (WebGL), with floorplan overlays
9. **PWA** - Service worker, manifest, installable
10. **Keyboard Shortcuts** - Configurable keyboard shortcuts with modal reference
11. **Operator Org & Managed Orgs** - Bootstrap creates the operator's own org
    (`is_operator_org`) beside the `$SYS` org (`is_system_org`); its NATS account
    is the hub for shared operator services. Flagging a customer org `managed`
    provisions a `helpdesk.>` stream export from its account (configurable:
    `nats.managed_export_subject`) plus a hub-side import remapped to
    `helpdesk.{organizations.code}.>`, so event provenance is subject-based and
    cannot be forged by the publisher.
    - **Both records are read-only in the console.** Reconciliation rewrites them
      on every org save, so an Edit button would offer a change that is silently
      undone. Recognised by name in `ui/src/utils/managedExports.ts`, which
      `hooks/managed_org_exports_test.go` reads so the two cannot drift.
    - Why `reconcile` rather than create-if-missing, and why the import *record*
      is still keyed by the immutable `org.Id` while the *subject* token is the
      code: `hooks/managed_org_exports.go`.

12. **Edge sites** - An edge site is a **Thing**. The agent
    ([stone-age-io/agent](https://github.com/stone-age-io/agent)) authenticates
    against `things` and calls `GET /api/me/leaf-config` for everything a NATS
    leaf server needs, in ten named fields. So a gateway needs **no read grant on
    any `nats_*` or `nebula_*` collection**, and `nats_system_operator` stays
    superuser-only. Don't add a read branch to those collections to make an edge
    feature work — extend the route.
    - **Nothing in the schema marks which Things are gateways.** `thing_types`
      has no such field; "gateway" is a naming convention a tenant picks. Do not
      write code that assumes the marker exists — this file once claimed it did,
      and a console feature was built on the strength of it and then removed.
    - **The JetStream domain is the Thing's code**, computed and never stored. A
      stored column could disagree with the code, and the disagreement surfaced
      as a site that simply stopped appearing.
    - **`hub_domain` is cached by the agent, so changing it is not a live change.**
      The route serves it, but the agent stores it in its platform session file and
      re-fetches only when that is empty. A deployment that moves its hub's
      JetStream domain will not reach running agents until each re-runs
      `agent -leaf-config` — which is defensible, since such a move invalidates
      every leaf's generated `nats-leaf.conf` anyway, but it is not something a
      console action can push.
    - **Site liveness is asked of NATS, never stored** — no heartbeat, no
      `leaf_status` bucket. A heartbeat travels over the very link whose failure
      it reports. It is a dashboard widget recipe against
      `$SYS.REQ.ACCOUNT.PING.CONNZ`, deliberately not a screen.
    - **Tenant roles carry no `$SYS` publish deny, and adding one is a
      regression.** The account boundary already blocks the operator-wide
      endpoints, and in NATS a publish DENY beats a publish ALLOW — so a deny
      silently kills account monitoring and fails as a bare timeout.
      `internal/demoseed/contract.go`, pinned by
      `internal/health/leaf_visibility_test.go`.
    - Why the route gates on nothing, and the wire contract:
      `hooks/leaf_config_routes.go`. The generated `nats-leaf.conf` (two
      mandatory directives no string assertion can check) and the opt-in embedded
      `nats-server` live in the agent repo.

13. **Digital Twin / Live State** - **Two** KV buckets per org, split by owner:
    `twin` (reported — the device writes it, edge→hub) and `twin_desired`
    (desired — operators write it, hub→edge). Keys are `<kind>.<code>.<prop>`;
    direction is the bucket, so keys carry no sync bookkeeping. Defined in
    `ui/src/utils/twin.ts` and the agent's `internal/edge/twin.go` — keep the two
    retention configs in step, since whoever creates a bucket first defines it.
    The platform server **cannot** provision them: it holds the operator only and
    has no reach into an org's account.
    - **The twin is now a preset over a general mechanism, not the mechanism.**
      Since agent v0.2.0 the agent takes lists — `sync.mirrors` (hub→edge) and
      `sync.relays` (edge→hub) — and `sync.twin: true` expands to these two
      buckets. Everything below still holds for the twin; it now also holds for
      any bucket a site declares. (The old `twin.enabled` is rejected by name.)
    - **`TWIN_BUCKET_CONFIG` reaches further than its name.** The agent's copy
      (`bucketConfig()`, formerly `twinBucketConfig()`) is the shape it gives
      **every** bucket it creates locally, not just the twin's. Changing retention
      here changes the default for user-declared buckets too.
    - **Non-twin buckets have no creator but the console.** The agent creates only
      the *local* side of a declared bucket and requires the hub side to already
      exist; only the two preset names are exempt. This server cannot create it
      either (no credential in the org's account). So for any bucket beyond the
      twin, the console or `stone` CLI — something holding a user credential — is
      the only thing that can make it. Agent-side a missing hub bucket is reported,
      not created: `agent_edge_sync_up{bucket,direction}` goes to 0 and the
      agent's `sync` check warns.
    - **Do not merge the buckets.** One writer per bucket is the whole safety
      property; two ends writing one bucket does not pick a loser, it oscillates.
      On the agent this stopped being structural when the lists arrived — it is now
      a config check that **refuses to start** and names the bucket. Worth knowing
      here because the console creates buckets too: a bucket declared in both
      directions is a boot failure at the site, not a silent mess.
    - **Four jobs, four homes.** Reported state → `twin`. Setpoints and config →
      `twin_desired`. Commands ("reboot") → a message on `cmd.>`, never a durable
      KV value. Ranges, thresholds, alarms → a rule-router rule. The last two are
      the ones people try to cram into `twin_desired`.
    - **Pair a desired key with an echo, never a measurement**, and **do not add
      operators to `twinDrift`** — that is a rules engine inside a KV browser, and
      rule-router already is one. If equality feels wrong for a key, the key is
      paired with a measurement; fix the pairing.
    - **Say `differs`, never `pending`.** Nothing in this platform applies desired
      values to devices, so "waiting for the device" asserts a control loop that
      does not exist. Show the values themselves, not a word for the difference.
    - The twin view is `KvDashboard` with `desiredBucket` set, not a separate
      component. Subset semantics, the reverted key-encoding experiment, and the
      mirror-vs-relay asymmetry: `ui/src/utils/twin.ts`.

14. **Readiness & metrics** - `GET /api/ready` (unauthenticated, 503/200) and
    `GET /metrics` (Prometheus, open by default) on the Control Plane; `/ready` +
    `/metrics` on the agent. Checks in `internal/health`, exposition in
    `internal/metrics`, platform-specific parts in `hooks/observability.go` +
    `hooks/readiness.go` + `hooks/metrics.go`.
    - **A check must be answerable first-hand by the process running it.** This is
      the NATS account boundary restated: the Control Plane holds the operator and
      `$SYS` and has **no credential inside any organization's account**. Do not
      "improve" a metric by minting it one — that turns a credential issuer into a
      data-plane participant in every tenant's bus. **No per-org labels**, same
      reason.
    - **Four states, and only `fail` is unready.** `warn` is
      running-but-misconfigured; `skipped` means the check could not look, and it
      ranks **below** `ok`. An islanded edge warns rather than failing.
    - **Don't add a check that cannot fail, and don't publish a metric that cannot
      vary — but verify the claim first.** A comment asserting a limitation is not
      evidence of one; `stone_age_nats_cluster_routes` was nearly cut on one that
      was wrong.
    - **`stone_age_records{collection="things"}` counts devices CONFIGURED**, not
      online, and an alert on it can never fire. Say so in the HELP text of any
      metric mistakable for a health signal.
    - Why `nats_trust` earns the feature, why Nebula expiry is the one credential
      this process may check (and its **two** windows — see also
      `ui/src/utils/expiry.ts`), why the prober caches, and why `/metrics` is open
      by default: the files above, plus `hooks/cert_expiry.go`.

15. **Decommissioning a device** - `things.active`, owner/admin only. Enforced in
    **four** places at once because any one alone is a half-measure: the
    `authRule` blocks new logins, `hooks/active_flag.go` refreshes `tokenKey` so
    tokens already issued die immediately, and the same hook mirrors `active` onto
    the linked `nats_user` and `nebula_host`. A device's real capability is its
    credentials, not its PocketBase session.
    - **Deactivate, never delete.** Revoking a Nebula certificate requires the
      certificate to still be in the database to fingerprint it.
    - **It mirrors `active`, not `revoke`.** In pb-nats `revoke` is the
      "credentials leaked" button — it hands back a *working* replacement — and it
      is checked first, so setting both in one save silently takes the revoke path.
    - The Nebula half takes effect on redeploy, not instantly; Nebula has no CRL.
      Requires pb-nebula v0.2.0, and from v0.3.0 the blocklist is CA-scoped rather
      than network-scoped. Full reasoning: `hooks/active_flag.go`.

15b. **Rotating an organization's Nebula CA** - `POST /api/org/nebula-ca/rotate`,
    owner/admin, `{"step":"prepare"|"commit"|"finish"}`. A route rather than a
    rule branch for the reason `hooks/nats_account_routes.go` is: a rule cannot say
    "this one field and nothing else". **Three steps because the wait between them
    is the feature** — Nebula verification is mutual and config distribution is
    pull-based, so a single write carrying both new trust and new certificates
    splits the mesh. The tenant owns the lever because the wait belongs to whoever
    operates the devices.
    - Requires **pb-nebula v0.3.2**. Before it the rotation checks were bound to
      the record-API request hooks alone and this route calls `app.Save()`, so
      validation was skipped while the executor ran regardless.
    - `hooks/nebula_routes.go`, `hooks/nebula_rotation_test.go`, and
      `scripts/test-authz.sh` section 13b.

15c. **The `/32` certificate defect** - pb-nebula signed host certificates at
    `/32` until v0.3.0. Nebula puts a certificate's network straight onto the tun
    device and installs a link route for it, so the mask **is** the host's route to
    the overlay: a `/32` verifies, renders, handshakes — and moves no packet.
    Nothing errors.
    - **Nothing re-signs automatically, deliberately.** Re-signing moves a
      fingerprint, and a fingerprint is what `pki.blocklist` revokes, so a sweep
      would rewrite every peer config in the mesh on the strength of a dependency
      bump. `GET /api/org/nebula/cert-audit` names the affected hosts (active only);
      the fix is `renew`, one host at a time, then redeploy that host's config.
    - `hooks/nebula_routes.go`, `ui/src/utils/nebula.ts`.

16. **Organization code — the ecosystem's namespace root** (ADR 0002 in
    `platform-docs`). The rule is **ids for storage, codes for addressing**.
    `organizations.code` is the one *globally* unique identifier in the ecosystem;
    everything below it (`things`, `locations`, and the two type collections) is
    unique only within its org, enforced by `UNIQUE (organization, code) WHERE
    code != ''` partial indexes. Relation columns stay PocketBase ids.
    - **`code` is optional but immutable.** Mutability was the disqualifier, not
      optionality. `@request.body.code:changed = false` freezes it on
      `organizations` and the four inventory collections — but that is a rule and
      hook guard, so it binds the API and **not** a superuser editing in the
      PocketBase dashboard. Once codes are on stickers and in signed account JWTs,
      an edit is a site visit plus a re-signed export.
    - Derivation **refuses on collision rather than auto-suffixing**: an invented
      `acme-2` would be printed on labels and baked into signed JWTs before anyone
      noticed it was the wrong tenant. A leading digit is valid (`816tech`).
      `hooks/org_code.go`, and `orgCodeFor` in `bootstrap.go` for the two
      infrastructure orgs.
    - The review question this yields, and the reason it is worth keeping: not
      *"is this identifier unique"* but **"whose database does this identifier
      belong to."** `orgSlugFor` (`hooks/thing_routes.go`) was a second,
      independent instance of the same bug.

17. **QR labels** (`ui/src/components/common/QrLabelModal.vue`) - print an
    operator-branded label from any thing or location that has a code. It takes a
    **list**, and a detail view passes a list of one, so there is one code path
    rather than two layouts to keep in step. The Things and Locations lists print
    the **whole result set of the current filter**, not the current page — the
    search box IS the selection mechanism — and the count rides in the button so
    the scope is visible before the click.
    - **The payload is the bare code**: no host, no org, no kind token. A sticker
      in a public hallway is an attacker-writable surface, so a URL payload would
      let a forged label send a person to arbitrary content; a bare in-system
      identifier means the worst case is resolving a different record inside an
      already-authenticated session. It also makes maximum error correction free.
    - **The customer/org name is deliberately not printed** — a tenant name beside
      a device naming convention is free reconnaissance.
    - Records without a code are skipped and **named**: a silent drop is only
      discovered at the site.
    - Sizes are millimetres, and there is deliberately **no RFID inlay keep-out**.
      The teleport, the print-CSS mechanics and the per-size `@page` box: the
      component. The same labels are scanned by `ScannerWidget` and the helpdesk's
      `/staff/scan`; nothing fetches the decoded string as a destination, and there
      is deliberately **no resolver service**.

18. **Uploaded files are protected, all of them** - every file field on the
    platform sets `protected: true` (`users.avatar`, `organizations.logo`,
    `locations.floorplan`, `things.photo`, `locations.photo`). An unprotected
    PocketBase file URL is served to **anyone**, with no auth and no expiry --
    the only obstacle is the random suffix on the stored filename, so the URL is
    a non-revocable bearer credential that leaks through Referer headers,
    screenshots, proxy logs and support tickets for the life of the record.
    Protected, `apis/file.go` resolves the `token` query param to an auth record
    and runs `CanAccessRecord` against the collection's **viewRule** -- the same
    boundary the rest of the API enforces.
    - **The auth token is NOT a file token.** `/api/files` accepts only
      `core.TokenTypeFile`, minted by `POST /api/files/token`. Two avatar call
      sites passed `pb.authStore.token` for years and "worked" only because the
      field was unprotected and the param was ignored. Copy that onto a
      protected field and you get a 404 with nothing in the console.
    - **The migration derives its list by walking the live schema**
      (`migrations/schema_update_protected_files.go`), so a file field added
      next year is protected on upgrade by a migration written today. A
      hardcoded list splits adding an upload field into two halves that fail
      independently and silently: set the flag in `schema.json` and fresh
      installs are fine, forget the migration and every EXISTING deployment
      keeps serving it in the clear. `TestEveryFileFieldInTheSchemaIsProtected`
      is the matching guard.
    - **The token is cached and deliberately NOT reactive**
      (`ui/src/utils/fileToken.ts`). `fileToken.duration` is 180s; a reactive
      token would change every derived URL on rotation and re-fetch every image
      on the page every three minutes, forever, on a screen nobody is touching
      -- and `dashboard` is an appliance login for an unattended display. URLs
      resolve once per record (`useFileUrl`), so a rotation reaches only images
      that mount after it. `logout()` clears it: it outlives
      `authStore.clear()` by up to its full duration and is a bearer credential
      for every file the previous session could read.
    - Section 22 of `scripts/test-authz.sh` proves it at the HTTP layer, which
      is the only layer where the flag means anything -- a unit test can assert
      `protected: true` all day while the server hands over the bytes.

19. **Thing and location photos** - one image of the physical object
    (`things.photo`, `locations.photo`), for the moment somebody is standing in
    front of it wondering whether this is the right one. It lives on the thing
    and location DETAIL views and deliberately nowhere else; the install context
    it carries otherwise lives in one technician's head and leaves when they do.
    - **`things.photo` is EDIT-ONLY.** Creating a Thing goes through
      `POST /api/org/things`, a JSON provisioning route that writes the Thing
      and its identities in one server-side transaction; it cannot carry a
      multipart upload, and bolting a second client call onto it is the pattern
      that route exists to have removed. `locations.photo` works on create too,
      because locations use the plain record API with FormData.
    - **The photo is a separate request from the rest of a Thing edit**, and
      must stay one. That body's null-vs-ABSENT distinction is load-bearing --
      the member branch of `things.updateRule` requires
      `nats_user:changed = false`, and a field omitted from a JSON body counts
      as unchanged. FormData has no null and no way to omit: everything is a
      string and an empty one CLEARS the field, so rebuilding that body as
      FormData turns a member's ordinary inventory edit into a 404.
    - **No rule change was needed, and that is asserted rather than assumed.**
      The member branch freezes `organization`, `code`, `nats_user`,
      `nebula_host` and `active` and nothing else, so an ordinary inventory
      field is writable by `member` and not by `viewer` or `dashboard`. Section
      23 of `scripts/test-authz.sh` checks it against a live server, including
      that a multipart upload cannot smuggle a frozen field alongside the photo
      -- `-F` is a different parse path from every other check in that file.
    - **One photo, not a gallery -- and the migration is NOT the reason.** An
      earlier version of this note said 1 -> N was blocked by the column type;
      that is wrong and it pointed at the wrong risk. PocketBase's
      `normalizeSingleVsMultipleFieldChanges`
      (`core/collection_record_table_sync.go`) rebuilds the column AND wraps
      each existing value in `json_array(...)`, and both fields carry the
      deterministic id `file347571224`, so a plain re-import migration applies
      it. Going up is cheap. The reasons to stay at 1 are that **an array has no
      primary element** -- anything rendering "the" photo silently becomes
      `photo[0]`, i.e. whatever was uploaded first -- and that **coming back
      down is destructive**: multiple -> single keeps
      `json_extract(..., '$[#-1]')`, the LAST file, and PocketBase deliberately
      leaves the others orphaned in `pb_data/storage`, riding every backup
      forever. Cheap to add once a second photo has a reader; expensive to undo
      once every record has five. Revisit as a designated primary plus ordering,
      never as a bare `maxSelect` bump.
    - **It lives in the Basic Information card and clicks through to the full
      image** -- not in the page header, and not on list rows. Both of those
      shipped first and were wrong for the same reason: they spend prominent
      space on a thumbnail small enough to recognise a device but too small to
      answer anything, while the question a photo actually gets asked (read the
      serial off that label, see which way the panel faces, which door is it)
      needs the full frame. It sits in the card's right-hand gutter beside the
      fields (stacking above them on a phone, where there is no gutter), the
      caller gates it with `v-if` so a record without one simply lets the fields
      take the full width, and `zoomable` opens the stored image with no thumb
      parameter at all. The full-size URL resolves only on open, so a page with
      several photos does not fetch full copies of images nobody clicked.
    - **Not SVG**, though `locations.floorplan` allows it: this field is camera
      output, and there is no reason to re-widen the decoder surface
      `schema_update_floorplan_mime_types.go` narrowed.
    - **The browser downscales before upload** (`ui/src/utils/imageResize.ts`),
      so the 2 MiB cap is a backstop rather than the mechanism -- a phone
      produces 3-5 MB and would otherwise be rejected outright. Files live in
      `pb_data/storage` with no object store configured and ride in every
      backup, which matters most on the two collections with thousands of rows.
    - **Thumb sizes are a closed set.** PocketBase serves `100x100` for any file
      field whether declared or not; every other size must be in the field's own
      `thumbs` list or the request silently serves the full original.
      `TestPhotoThumbsAreDeclared` pins the 400x400 the detail views ask for.
    - Deliberately **not** on the QR label (print, bare code, minimal), not in
      `leaf-config` (agents do not need it), and not in
      `auditSnapshotCollections`.
    - **Also not in `ScannerWidget` or `ThingMapDrawer`** -- both shipped in
      `a7412ea` and were removed. If you have just scanned the label you are
      already looking at the device, so the photo answers a question nobody is
      asking; and a 40px thumbnail standing in for the drawer's 📦 tells you
      less than the name already above it. The scanner's cost was the larger
      one and is the part worth remembering: to render a photo at all it had to
      force `id`, `photo`, `collectionId` and `collectionName` into the
      operator's configured `pbFields` list, then strip `photo` back out of the
      key/value rows -- an accommodation in a widget's data path for a
      decoration. Re-adding either brings that back.

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
- **An empty string is a valid match, so a blank scope is a wildcard.** The
  third costume of the same bug, and the one no role audit could have caught:
  every inventory read rule was `organization = @request.auth.current_organization`,
  both sides are TEXT defaulting to `''`, and in PocketBase `'' = ''` is true. A
  record with a blank `organization` was therefore readable by any caller whose
  own context was blank — no rule mis-written, no role bypassed, two sentinels
  comparing equal. Both halves were ordinary product states: deleting an
  organization blanks `organization` on the 15 relations into it that are
  non-cascade AND non-required (PocketBase blanks rather than deletes, via
  `SaveNoValidate`), and `hooks/membership_lifecycle.go` blanks
  `current_organization` on the way out of an org. A second set of branches, for
  a `leaf_nodes` collection that has since been dropped, had the identical hole
  on `@request.auth.organization`. Fixed in `migrations/schema_update_tenancy_sentinel.go` by
  requiring a non-blank context **and** a correlated membership — the membership
  clause is the load-bearing half, because `memberships.organization` is
  required and cascade so it can never be `''`, which kills the sentinel by
  construction rather than by a comparison someone may later tidy away. Note
  every *write* rule already had that clause, which is exactly why only the
  reads were exposed. **The review question is not "does this rule name the
  right roles" but "what does this rule do when both sides are the zero
  value."**
- **`current_organization` is the read boundary, so guard every path that
  writes it.** `users.updateRule` froze it to organizations the caller holds a
  membership in; `users.createRule` did not mention it, so an invited registrant
  could name any organization id at signup and then read that tenant. The create
  branch is anonymous and cannot check membership even in principle, so it
  refuses the field outright and accept-invite fills it in afterwards. A field
  that scopes reads needs a guard on *create* as well as update.
- **Do not put an apostrophe in a rule comment.** PocketBase tokenizes quote
  characters in the rule string before `//` comments are stripped, so one stray
  apostrophe re-pairs every quote after it: a later `''` literal then opens a
  string that nothing closes, and the whole collection fails to import with
  `invalid quoted text`. This cost a debugging cycle — the rule looked correct
  and the failure named a fragment of the expression rather than the comment.
  Several existing comments contain apostrophes and are harmless only because
  no string literal follows them. Write "the organization" rather than
  "the org's".
- **Keep a zero-authority role in the test matrix.** The two role bugs above were caught
  by the same thing: a role that holds no capability at all, used as the probe in
  `scripts/test-authz.sh`. `dashboard` is that role. Don't "simplify" the suite by
  testing denials with `member` — a role with *some* authority cannot prove an
  allowlist, because it passes for the wrong reason. (This role was called `badge`
  until it was renamed in `migrations/schema_update_dashboard_role.go`; commits and
  PRs before that date say `badge` and mean this.) `viewer` cannot take over the
  job either: it holds read capability, so a denial it passes proves less. Two
  roles, two purposes — don't merge them to save an enum entry.
- **Reads are org-scoped, not role-scoped, and that is deliberate.** Every read
  rule on `things`, `locations`, `thing_types`, `location_types` and
  `thing_type_operations` scopes on the active organization plus
  a correlated membership in it (see the sentinel bullet above), with no ROLE
  branch, so *every* role in an org — `dashboard` included — can
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
- **Hooks enforce INVARIANTS; rules enforce PERMISSIONS; routes handle what a
  rule cannot express.** "The API rules are the only enforcement layer" is about
  permissions, and `hooks/relation_tenancy.go` is the one deliberate exception —
  worth knowing where the line is, because the temptation to move authorization
  into hooks recurs. It refuses a relation pointing into another organization's
  records, which no rule *can* express (PocketBase traverses a STORED field, but
  `@request.body.account_id` is a raw submitted string with no target to
  traverse into). pb-nats signs the user JWT with whatever `account_id` names,
  so without it an owner could mint themselves a credential inside another
  tenant's account. It is DERIVED from the schema rather than listing the
  nineteen relations, binds the MODEL hooks so it covers `app.Save()` routes as
  well as REST, and applies to superusers — a cross-tenant relation is corrupt
  data whoever wrote it.

  Three reasons not to generalise this into a permission system. Audit and
  enforcement need **opposite failure modes** — an audit hook must never block a
  write, an enforcement hook must always block one, which is the same Before vs
  `After*Success` distinction `hooks/membership_lifecycle.go` already turns on.
  Hooks **cannot be a read boundary**: `OnRecordsListRequest` hands you
  `Records` and `Result` after the rule-scoped query has run, so filtering there
  gives wrong `TotalItems` and broken pagination, and there is no pre-query hook
  to inject a scope. And a second authoritative layer means "can role X do Y"
  has two answers. Tenant-defined roles, if they ever land, belong in the rules:
  make `memberships.role` a relation to an org-scoped role collection with
  boolean capability columns and let the rules traverse it, the way
  `organization.owner = @request.auth.id` already does.

  **The unchanged-id shortcut inside that guard is off when the record's own
  `organization` moves.** Ids already present in `Original()` are skipped as
  "checked once" — but checked against the organization it *used to have*, so a
  record could be walked into another tenant keeping every relation it held.
  Unreachable through the record API (every org-scoped update rule freezes
  `organization`) and reachable through exactly the two paths the model-hook
  binding exists for. `TestRelationTenancy` pins it, and the reload in that
  subtest is load-bearing: a record built in memory has no `Original()`, so the
  shortcut never engages and the gap does not reproduce.
- **`?`-prefixed operators are row-correlated** on the same relation path, so
  `memberships_via_user.organization ?= X && memberships_via_user.role ?= "owner"`
  matches one membership row. Without `?`, the condition must hold for *all*
  related rows.
- **Credentials are protected by row scoping, not hidden fields — and that
  doctrine is about the RECORD API only.** `nats_users.creds_file` and
  `nebula_hosts.config_yaml` stay readable because the identity that owns them
  needs them (the browser's NATS connection and the admin download button). The
  read rules restrict *which rows* a caller sees. Do not add `hidden: true` to
  them — it breaks both and buys nothing.

  **It does NOT transfer to anything that holds COPIES of records**, and
  `audit_logs` is exactly that: one flat collection, no `organization` column,
  no row scoping, operator-read. pb-audit snapshots `Record.PublicExport()` —
  every field a collection does not mark hidden — so for as long as it
  snapshotted everything, the two correct decisions above combined into a
  plaintext archive of every credential the platform had ever minted. Verified,
  not inferred: creating one NATS identity wrote the full `.creds` file, NKEY
  seed included, into two audit rows. That archive also sat outside at-rest
  encryption (which covers the `seed`/`private_key` columns, the hidden ones,
  not the credential files derived from them) and never expired, retention
  being off by default.

  Closed by pb-audit v0.2.0: values are opt-in per collection and everything
  else records `changed_fields`, the NAMES of the fields that moved. The
  allowlist is `auditSnapshotCollections` in `main.go` and it is guarded by
  `audit_snapshots_test.go`, which reads `schema.json` and fails if a listed
  collection has an unhidden credential-bearing field. Adding `nats_users` to
  that list restores the archive in one line, with no symptom — which is why
  the guard exists rather than a comment.

  **The review question this yields:** not *"is this field hidden"* but
  **"does anything copy this record somewhere the row rules do not reach."**
- **At-rest encryption covers minting keys, not issued credentials, and that is
  deliberate.** `encryption_key` protects the operator seed, account seeds and
  signing keys, and the Nebula CA key. It does NOT protect `creds_file` or
  `config_yaml`, and cannot usefully: a `.creds` file *contains* the user seed
  (pb-nats `jwt.FormatUserConfig`), Nebula requires the host key inline, and
  `ui/src/stores/nats.ts` reads `creds_file` from the API to open the browser's
  own NATS connection — a browser can never hold the key. Encrypting the column
  would therefore force a decrypting route plus changes in the edge agent and
  five UI call sites, and `pb-nats`'s `EncryptField`/`DecryptField` live in
  `internal/`, so the platform cannot even call them without the library
  exporting a primitive. `migrations/schema_update_credential_scoping.go`
  reached the same conclusion for `hidden: true`.

  What that buys is worth knowing precisely: a stolen database with the key held
  elsewhere yields **no ability to mint new identities** and **every existing
  credential** — plus, in any database that predates the audit change above and
  has not had those rows cleared or aged out, **every historical one too**,
  including credentials rotated *because* they leaked. The NATS side survives
  that (the account JWT's revocation cutoff is by issue time and permanent);
  the Nebula side does not, since there is no CRL and an old `config_yaml`
  holds a live host key until the certificate expires or its fingerprint is
  blocklisted. The `UPDATE` that clears them is written out in
  `migrations/schema_update_audit_changed_fields.go`, deliberately as a comment
  rather than as migration code: that is the audit trail, and deleting it is a
  decision with a backup attached rather than something that should happen
  silently on deploy.

  Rotating the NATS side is central and cheap (`regenerate`); rotating the
  Nebula side needs re-issue plus a blocklist entry in every peer plus
  redelivery. Don't "fix" this by encrypting the column; state the boundary
  and let disk encryption, encrypted backups and single-tenant deployments carry
  the at-rest threat. See SECURITY.md.
- **A gateway reads nothing in `nats_*` or `nebula_*` beyond its own identity.**
  Its agent gets the account JWT, the operator JWT, the `$SYS` account JWT and
  its own creds from `GET /api/me/leaf-config` (`hooks/leaf_config_routes.go`),
  which reads those records with the privileges of the app itself and returns ten
  named fields. Don't add a read branch to those collections to make some edge
  feature work — extend the route instead. The point is that the edge's blast
  radius is a fixed list rather than a consequence of rules that change for
  unrelated reasons.
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
- **A hook that returns without `e.Next()` silently deletes every handler bound
  after it.** FIXED, and worth keeping written down because of how it presented.
  pb-tenancy's `organizations` AfterCreateSuccess handler was
  `return autoCreateOwnerMembership(...)` with no `e.Next()`, and it registered
  from `Setup`'s `OnBootstrap` callback rather than from `Setup` itself — which
  put it after every `hooks.Register*` call, so it was always last and nothing
  downstream ever noticed. Anything binding later got silence: no handler, no
  error, no log. `RegisterManagedOrgExports` on an already-bootstrapped test app
  provisioned nothing, and a `Priority: -9999` bind was the only thing that
  fired. That hook is now `hooks.RegisterOrgMembership` and it calls `e.Next()`,
  guarded by `TestOrgCreateDoesNotTerminateTheHookChain`, which binds on the
  bootstrapped app the harness hands out — precisely the case that used to be
  inert. `internal/testutil` still registers organization hooks before Bootstrap,
  now as a convention (be equivalent to main.go) rather than a requirement.
- **Tenancy is platform code, not a library.** `organizations`, `memberships` and
  `invites` were pb-tenancy until it was absorbed. Only half of it ever ran here:
  `schema.json` has owned those collections and every one of their API rules since
  the initial import, so the library's `createCollections`/`setAPIRules` half was
  dead, and its roles (`owner`/`admin`/`member`) had already been outgrown by
  `viewer` and `dashboard`. What came back in-tree is the half that did run —
  `hooks/org_membership.go` (owner membership) and `hooks/invites.go` (token,
  expiry, mail, resend, `POST /api/org/invites/accept`).
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
  not in the middleware that loads a bearer token. `things` sets
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
  exists only because `hooks/active_flag.go` gives it teeth — the flag, the token
  kill, and the NATS revoke are one operation. Do not
  add a status field to a device without deciding what enforces it.
- **A device's real capability is its credentials, not its PocketBase session.**
  Anything that takes a Thing out of service has to reach
  `nats_users` **and** `nebula_hosts`, or it has only closed some of the doors.
  This was a live gap until the Nebula half was added: the console door and the
  NATS door closed while the overlay network stayed open until the certificate
  expired. `hooks/active_flag.go` mirrors the flag to both, and the two cascades
  are independent — a device may hold either identity, both, or neither, so
  neither may short-circuit the other.
- **Schema changes need a new `migrations/schema_update_*.go`** — editing
  `schema.json` alone reaches fresh databases only. **A new non-null column with
  a live rule over it needs a backfill in the same migration**: PocketBase bools
  have no schema default, so existing rows land as `false`, and importing
  `authRule: "active = true"` without the `UPDATE` in
  `schema_update_device_active_flag.go` would lock every already-provisioned
  device out of the API on deploy.
- **A re-import cannot REMOVE a field, and no migration here could until 2026-09.**
  `initial_schema.go` calls `ImportCollectionsByMarshaledJSON` with
  `deleteMissing=false`, and in that mode `core.ImportCollections` walks the live
  collection and re-adds every field the import did not carry by id
  (`core/collection_import.go`). So deleting a field from `schema.json` and
  re-importing logs its ✅ and changes nothing. Removal needs an explicit
  `Fields.RemoveById` + `Save`, keyed on the ID rather than the name for the same
  reason the importer is. `migrations/field_removal_test.go` pins both halves
  against the real importer, and `schema_update_drop_message_schemas.go` is the
  worked example — note that it deliberately does NOT re-import, since an import
  placed after the removals would resurrect them in the same run. Step order
  matters too: PocketBase refuses to delete a collection that still has relation
  references pointing at it, so a relation INTO a doomed collection goes first.
- **A removal migration cannot be tested by `migrate up`.** A fresh database
  imports the already-thinned `schema.json`, so the migration reaches nothing to
  remove and passes without touching anything. `drop_contract_layer_test.go`
  restores the old shape first, through the real importer, from
  `migrations/testdata/`. Any future field removal wants the same shape or its
  green tick means nothing.
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

UI side: the capability map lives in `ui/src/stores/auth.ts` (`can.*`); the router
guards on `meta.requiresCapability` and the sidebar hides what a role can't reach.
Keep it in step with the table above.

**The router guard MUST await `authStore.initializeFromAuth()` before reading a
capability.** `app.use(router)` starts resolving the initial navigation the
moment the router is installed — before main.go's next line even calls the
hydration it deliberately awaits pre-mount. `user` is set synchronously, so
`isAuthenticated` passed and nobody was bounced to `/login` (which is the bug
main.ts's comment describes fixing), but `memberships` arrive over the network,
so every capability read false on that first pass and **every gated route
redirected to `/`**. The symptom is worth recognising: a deep link or a plain
browser refresh lands on the dashboard while the sidebar — reading the same
capabilities a tick later — cheerfully shows the link you were just refused.
Hydration is memoized in the store so the guard and main.ts share one in-flight
promise; that is deliberately not "call it earlier in main.ts", because an
ordering convention between two files is exactly what broke. The guard also
carries the intended path through as `?redirect=`, and `LoginView` accepts it
only when it is a relative in-app path — it arrives in a URL someone can send
you, so pushing an absolute one would make the login form an open redirect (the
`//` case is not redundant: `//evil.test/x` is protocol-relative).

## Testing

- `cd ui && npm test` — Vitest. Pure logic only, node environment, no component
  mounting except `ConfirmDialog` and `QrLabelModal`, where the DOM contract IS
  the subject. Covers the pure logic `vue-tsc && vite build` cannot protect:
  `twinDrift`, `useSubscriptionManager`, `useEscapeKey`, the `can` capability
  map, dashboard import/export, `createDefaultWidget`, the JSON Schema
  round trip in `schemaFields` + `inferSchema`, the file-token cache, and
  `targetDimensions`. A spec that needs a DOM opts in
  with `// @vitest-environment jsdom` on its first line.
  - **The file-token cache is tested through `fileUrl()`, not through the
    getter it wraps.** Every failure mode there is SILENT -- a missing cache is
    a burst of requests nobody sees, a missing reset hands one user's
    credential to the next, a missing expiry skew breaks an image only for the
    caller unlucky enough to ask in a token's last second. None of them throws.
    Asserting through the public surface means the tests survive the caching
    moving and still fail if the behaviour does.
  - **`imageResize` is split so the judgement is testable and only the plumbing
    is not.** vitest runs in node and jsdom does not implement `canvas.toBlob`,
    so the drawing half cannot be covered here at all; every decision therefore
    lives in `targetDimensions`, which is pure arithmetic.
  - **A form that edits a document needs a ROUND-TRIP assertion, not a render
    test.** `SchemaBuilder` reads a JSON Schema into an internal `Field` struct
    and writes it back out, so any keyword `schemaToFields` does not read is
    deleted by the next `fieldsToSchema`. `title` — the human label every
    seeded thing/location type uses, and the one both `JsonSchemaForm` and
    `MetadataEditor` render — was dropped this way for as long as the builder
    existed, silently, because `isFormCompatible` gates the "switch to JSON
    view" banner on `$ref`/`anyOf`/`oneOf`/`allOf` and knows nothing about
    which keywords survive the trip. Adding a keyword to the builder means
    adding it to `Field`, to both conversion functions, and to the identity
    test. Note the banner is not a safety net: it answers "can the form
    represent this shape", never "will the form preserve this content".
  - **The builder has no nesting cap, deliberately.** One used to live inside
    `isFormCompatible` and disagreed with `SchemaFieldEditor`'s `depth <
    maxDepth` by exactly one level, so a five-deep schema built in the form was
    refused by the form on reload. It was removed rather than corrected: every
    other branch of that predicate states a fact about what the builder can
    *represent*, while a depth limit is a rendering preference, and one function
    holding both kinds of judgement is what produced the off-by-one.
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
- `./scripts/test-authz.sh` — **run after any API-rule change in `schema.json`.**
  Builds the binary, stands up a throwaway DB, and asserts every authorization
  behaviour in it against a live server — `EXPECTED_CHECKS` at the top of the
  script is the count, and is the only copy of it worth trusting. The rules are the only tenancy enforcement
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
- Beyond Vitest's pure-logic specs (first bullet), the frontend is checked by
  hand: HMR (`npm run dev`), browser DevTools, and the PocketBase admin panel at
  `/_/`. Nothing renders a component in CI, so anything visual is unguarded.

## Important Files

- `main.go` - Backend entry, PocketBase setup, hooks, bootstrap command
- `hooks/leaf_config_routes.go` - `GET /api/me/leaf-config` (bound to `things`, no record id): everything an agent needs to stand up a NATS leaf server, including the `$SYS` account JWT the leaf's MEMORY resolver cannot fetch. The JetStream domain is computed from the Thing's code rather than stored
- `hooks/thing_routes.go` - `POST /api/org/things`: Thing + optional NATS/Nebula identity in one transaction; member-level for inventory, owner/admin for the identity half
- `hooks/nebula_routes.go` - `POST /api/org/nebula-ca/rotate` (owner/admin, three steps) and `GET /api/org/nebula/cert-audit`. Both are routes for the same reason `nats_account_routes.go` is: a PocketBase rule cannot say "this one field and nothing else", and the audit needs a Nebula certificate parsed, which the browser cannot do
- `hooks/activity.go` - the tenant activity feed. The collection list IS the safety argument (see feature 7b); bound to the request hooks because they are the only layer carrying the actor, and best-effort because an observation must never cost the user their write
- `hooks/relation_tenancy.go` - the one hook-based enforcement in the platform: no relation may point into another organization's records. Derived from the schema rather than listing the nineteen relations, bound to the MODEL hooks, and applied to superusers too. See the invariants-vs-permissions bullet under **Roles & Authorization**
- `hooks/org_provisioning.go` - every organization's NATS account and Nebula CA. Create-if-missing and bound to update as well as create, so re-saving the organization RETRIES a failed provision; failures are returned rather than logged, and returned AFTER `e.Next()` so a NATS outage cannot cost the owner their membership row
- `hooks/schema_fields.go` - the fields that exist only once `schema.json` has been imported, shared by `bootstrap` (fatal) and the `schema` readiness check (a Fail). One list, because PocketBase discards a write to a missing field in silence and two copies drift
- `hooks/client_config_routes.go` - `GET /api/client-config`: deployment facts the SPA cannot be compiled with (browser-facing NATS WebSocket URLs). Authed (`users`) — there is no pre-login need, so no reason to publish the bus address
- `internal/health/` - readiness engine: check registry, background prober, and the unauthenticated NATS reachability (`DialInfo`) + operator-trust (`CheckCreds`) probes
- `internal/metrics/` - Prometheus exposition, plus the optional Bearer/Basic scrape token. The agent repo carries a copy of this and of `internal/health`: duplicated rather than extracted into a shared module, because two small copies that drift are cheaper to live with than a third repo to version, and the two processes check different things
- `hooks/observability.go` + `hooks/readiness.go` + `hooks/metrics.go` - the Control Plane's `/api/ready` + `/metrics` routes, its checks, and its collectors
- `hooks/cert_expiry.go` - the one scan behind both the `nebula_cert_expiry` check and the `stone_age_certificate*` metrics, so the two cannot disagree. **Two windows, not one**: hosts at 30 days (they renew themselves), the CA at 90 (it cannot be renewed at all, only rotated, and rotation needs months). `TestCAGetsALongerWarningWindowThanAHost` stops them collapsing back together
- `ui/src/utils/fileToken.ts` - the file-token cache and the only place that mints one, plus `fileUrl()` for imperative callers. Every uploaded file is protected, so a URL without a token is a guaranteed 404. Deliberately non-reactive: see feature 18
- `ui/src/composables/useFileUrl.ts` - the reactive twin, for a component rendering straight from a prop. Resolves once per record, never per token rotation
- `ui/src/components/common/ImageUploadField.vue` - the one image picker, replacing three hand-rolled copies that had already drifted (one leaked object URLs, one had no removal path at all). Staged like every other field: choosing and removing both take effect on Save
- `ui/src/components/common/UserAvatar.vue` / `RecordPhoto.vue` - the read-only halves. UserAvatar is for people (initial-circle fallback, and a viewRule that is NOT org-scoped, so it belongs only on owner/admin or operator screens); RecordPhoto is for things and locations
- `ui/src/utils/nebula.ts` - rotation state derived from the CA's certificates (never a stored status), the rotation call, and the `/32` audit fetch. The audit swallows its own failure and returns an empty set: it drives an advisory badge, and a list view that refused to render because an advisory endpoint was down would be the worse outcome
- `ui/src/utils/managedExports.ts` - names the platform-provisioned export/import pair so the console can present them read-only; mirrors `managedExportName` in `hooks/managed_org_exports.go`, and `hooks/managed_org_exports_test.go` reads this file to keep the two honest
- `internal/health/leaf_visibility_test.go` - what a tenant can learn about its own leaf nodes, asserted against a real hub with a real leaf attached. Nothing in the platform ships a view on it — site connectivity is a dashboard widget recipe — but the recipe only works if these three nats-server behaviours hold: CONNZ names leaves by `server_name`, the **account** (not any permission we write) blocks the operator-wide `SERVER.PING.*` endpoints, and a publish DENY beats a publish ALLOW
- `ui/src/stores/auth.ts` - Authentication and organization context
- `ui/src/stores/nats.ts` - NATS WebSocket connection manager
- `ui/src/stores/dashboard.ts` - Dashboard state and persistence
- `ui/src/router/index.ts` - Route definitions with auth guards
- `ui/src/types/pocketbase.ts` - All PocketBase record type interfaces
- `ui/src/types/dashboard.ts` - Widget and dashboard type definitions
- `ui/src/utils/pb.ts` - PocketBase client singleton
- `config.yaml` - Application configuration
- `schema.json` - PocketBase collection definitions
