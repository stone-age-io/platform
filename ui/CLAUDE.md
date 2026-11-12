# ui/CLAUDE.md - the console (Vue 3 SPA)

Claude Code loads this file when it touches anything under `ui/`. The role table
and the one-line authorization rules are in the root `CLAUDE.md`, and
`AUTHORIZATION.md` has the long form. Its rules about capability gates apply to
every view here.

## Stack

- **Vue 3** (Composition API + `<script setup>`)
- **Vite 8** build tool (bundles with Rolldown, not Rollup)
- **Pinia 4** state management
- **Vue Router 5** routing
- **Tailwind CSS 3.4 + DaisyUI 4.12** styling (light/dark themes) — **pinned, see below**
- **TypeScript 6** (7 is out but unusable here, see below)
- **NATS WebSocket** (@nats-io/nats-core, jetstream, kv)
- **Leaflet 1.9** for maps — markers, clustering, floor-plan overlays — over a
  **MapLibre GL 6** vector basemap via `@maplibre/maplibre-gl-leaflet` (WebGL).
  v6 loads its tile worker as a separate file, so the `setWorkerUrl()` call at
  the top of `ui/src/composables/useLeafletMap.ts` is load-bearing. Without it
  the map is a flat sheet of theme colour and the page shows no error (only a
  console line). Import the worker with `?worker&url`, not `?url`: the worker
  file imports `maplibre-gl-shared.mjs`, which `?url` does not bundle. CI checks
  the worker is emitted, not that the map draws.
- **ECharts 6.1 + vue-echarts** for charts
- **grid-layout-plus** for dashboard grid layout
- **@vueuse/core** for reactive utilities
- **date-fns** for date formatting
- **jsonpath-plus** for JSON path extraction in widget data
- **marked** for Markdown rendering
- **@lucide/vue** for icons: import each one as a component (`import { Pencil } from '@lucide/vue'`), so only the icons used ship. Not emoji: those draw differently on every OS and ignore the theme. Plain text glyphs (✕ ✓ ↑ →) are fine. `lucide-vue-next` is the deprecated old name
- **PocketBase JS SDK** for API client

### Pinned on purpose

Everything else in `ui/package.json` tracks latest. A routine "bump everything"
pass must not move these:

- **Tailwind 3 + daisyUI 4.** Moving to Tailwind 4 / daisyUI 5 is a
  design-system migration: hundreds of `oklch(var(--x))` references stop being
  valid and fall back in silence, and the theme block is copied verbatim into
  the access-control console. Do it across both repos as scheduled work, with a
  person clicking through both themes. The reasons: the top of
  `ui/tailwind.config.js`.
- **TypeScript 6.** TS 7 no longer exposes the `./lib/tsc` subpath `vue-tsc`
  resolves, so `npm run build` dies before type checking.
- **`postcss-selector-parser` exactly 7.1.6**, through `overrides` (a security
  fix under Tailwind 3). 7.2.0 double-escapes selectors under Tailwind 3, so
  every `disabled:` and `focus:` utility stops matching while every check stays
  green. To move it, build before and after and compare the CSS files byte for
  byte. Remove it with the Tailwind 4 migration.

The `Assert the deliberately pinned majors` step in `.github/workflows/ci.yml`
fails the build if Tailwind, daisyUI, TypeScript or `maplibre-gl` changes major.
Nothing in CI renders the UI, so it catches the upgrade and nothing else; the
nearest thing to an eye on it is `/dev/widgets`.

## Directory structure

`ui/src`: `components/` (`dashboard/` and `widgets/` are the Visualizer),
`composables/`, `stores/`, `router/`, `types/`, `utils/`, `views/`. `npm run
build` writes to `../pb_public`, which the Go binary embeds.

## Code conventions

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

`ui/CONTRIBUTING.md` is the short version for a newcomer: how to run it, the
widget gallery, and how to add a widget. This file is the reference behind it.

## Widgets live in one object

`components/dashboard/widgets.ts` holds `WIDGETS`, a
`Record<WidgetType, WidgetDefinition>`: the renderer, the config panel, the Add
Widget palette entry (label, icon, description, category), how it sits on a
phone (title, expand button, width, height), and whether the shared data-source
panel applies. Adding a widget is that entry plus the component.

- **It is a map, not a plugin system**, and must stay one. No `import.meta.glob`,
  no manifest, no self-registration, no schema-driven form generation. Eighteen
  widget types that change twice a year do not need a framework, and the object
  is less code than the switch it replaced.
- **Declaration order is the palette's order**, grouped by `category` in
  `WIDGET_CATEGORIES` order.
- **`WidgetConfig` is a discriminated union** keyed on `type`
  (`types/dashboard.ts`), so a widget types its prop as `WidgetConfigOf<'gauge'>`
  and reads `config.gaugeConfig` without a `!`. The field names are frozen:
  every dashboard ever saved — localStorage, a KV bucket, an exported JSON file
  — carries those keys, so renaming them to a single `config` is a data
  migration rather than a refactor. The union is a compile-time claim about
  data that arrives at runtime from user-editable JSON, so the `?.` reads that
  remain in the widgets are load-bearing for an imported dashboard.

## The widget gallery

`/dev/widgets` renders every widget against fixtures
(`views/dev/widgetFixtures.ts`) with no NATS connection, no login and no
dashboard. Most of this console is the dashboard and its widgets, and
CI renders none of them, so this is the closest thing to an eye on them.

The route is registered inside `if (import.meta.env.DEV)` in `router/index.ts`.
Vite replaces that with the literal `false` in a production build, so the view
and its fixtures are **dropped from the bundle**, not merely unreachable. Keep
it that way: the gallery mounts widgets with no auth guard in front of them.

## NATS server URLs in the console

`nats.websocket_urls` is the deployment default (see Configuration in the root
`CLAUDE.md`).

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

## Router guard and capabilities

**The router guard MUST await `authStore.initializeFromAuth()` before reading a
capability.** Memberships arrive over the network after the router has started
the first navigation, so without the await every capability reads false and a
refresh on a gated page lands on `/` while the sidebar shows the link.
Hydration is memoized, so the guard and `main.ts` share one promise; do not
replace this with an ordering convention between the two files.

The guard passes the intended path as `?redirect=`, and `LoginView` accepts it
only as a relative in-app path, `//` included in the refusal, so the login form
is not an open redirect.

## Switching organization remounts the page

`MainLayout` keys its `<router-view>` on the active organization, so a switch
throws the page away and mounts a fresh one: page one, empty search, its own
`onMounted` load. **Do not give a page, or anything inside one, its own
org-change handling** — no listener, no `watch` on `currentOrgId`. Things
that outlive the page and care about the org (the NATS
reconnect in `App.vue`) watch `currentOrgId` themselves. The switch re-runs the
router guard against the new role, so a page the role cannot reach goes to `/`.
A record page (any route with a param) goes to the list its path sits under
instead of remounting, since its record belongs to the org being left; and
logout remounts nothing. `MainLayout.vue` says why.

## Reading one KV key

Watch a single key with `useKvKey(bucket, key)` (`composables/useKvKey.ts`),
never with a hand-rolled `Kvm` open/get/watch; the file's header says why it
does `get` before `watch`. A key prefix is a different
job: `useNatsKv` (the twin and the KV browser) or `useNatsKvWatcher` (tables and
maps).

## The activity feed

- **"Open record" is gated on the capability the TARGET route needs**, not the
  one that opened the feed. The three type collections have no detail view, only
  an edit form behind `manageDefinitions`, so an ungated link bounces a
  `viewer` to `/`. `RESOURCE_ROUTES` in `views/activity/ActivityView.vue`
  maps each noun to a route and capability; `TestActivityNounsAllHaveAConsoleRoute`
  (Go) reads it so a new feed collection cannot ship without one.
- `RecordTimestamps.vue` sits on every covered record's detail surface (the
  Thing and Location detail views, the three type forms). There is no "never
  edited" marker: PocketBase sets `created` and `updated` separately, so they
  can differ on an untouched record.

## Features

The numbers match the feature index in the root `CLAUDE.md`. What each feature
looks like to a user is in platform-docs `console.md`; these are the rules for
changing it.

5. **Resource inventory**
    - **`metadata_schema` is a form hint, not a contract.** Nothing on the server
      validates `metadata` against it, and the form never blocks a save on it. Do
      not add server-side validation without deciding what happens to records
      saved before a schema tightened.
    - The editor splits one document between typed fields and key/value rows
      (`utils/metadataDoc.ts`); a key that falls between the two is deleted on
      the next save, which `metadataDoc.spec.ts` pins.

13. **Digital twin** - two KV buckets per org: `twin` (the device writes it,
    edge to hub) and `twin_desired` (operators write it, hub to edge). Keys are
    `<kind>.<code>.<prop>`.
    - **`TWIN_BUCKET_CONFIG` (`utils/twin.ts`) must match the agent's
      `bucketConfig()`**: whoever creates a bucket first defines it, and the
      agent uses that shape for every bucket it creates locally.
    - **The server cannot create these buckets** (no credential in the org's
      account). The console or `stone` creates them; for any bucket beyond the
      twin's two, the console is the only creator the hub side has.
    - **Never merge the buckets.** One writer per bucket is the safety property;
      the agent refuses to start if a bucket is declared in both directions.
    - **Four jobs, four homes**: reported state in `twin`, setpoints in
      `twin_desired`, commands as `cmd.>` messages, thresholds as rules.
    - **Pair a desired key with an echo, never a measurement, and do not add
      operators to `twinDrift`.** Say `differs`, never `pending`.
    - The twin view is `KvDashboard` with `desiredBucket` set. Subset semantics
      and the mirror/relay asymmetry: `utils/twin.ts`.

17. **QR labels** (`components/common/QrLabelModal.vue`)
    - **One code path**: it takes a list, and a detail view passes a list of one.
    - **The payload is the bare code**, never a URL, so a forged sticker can at
      worst open a different record in a session the person already has.
    - **Only the organization's own data**: its code, then the record's code and
      name. No operator brand, no type.
    - The code's size is fitted per label (`utils/labelFit.ts`). Records without
      a code are skipped and named.

18. **Uploaded files are protected, all of them** - every file field sets
    `protected: true`, because an unprotected PocketBase file URL is served to
    anyone for the life of the record.
    - **The auth token is not a file token.** `/api/files` accepts only the token
      from `POST /api/files/token`; passing the auth token gives a 404 with
      nothing in the console. Use `utils/fileToken.ts` and `useFileUrl`.
    - **The token is cached and deliberately not reactive** (180s), so an idle
      dashboard does not refetch every image every three minutes. `logout()`
      clears it.
    - **`migrations/schema_update_protected_files.go` walks the live schema**, so
      a new file field is protected on upgrade too.
      `TestEveryFileFieldInTheSchemaIsProtected` is the guard, and section 22 of
      `scripts/test-authz.sh` checks it over HTTP.

19. **Thing and location photos** (`things.photo`, `locations.photo`), on the
    detail views and nowhere else.
    - **`things.photo` is edit-only**: Things are created through
      `POST /api/org/things`, a JSON route that cannot carry a file.
    - **The photo is its own request, separate from the rest of a Thing edit.**
      The member branch of `things.updateRule` needs frozen fields to be absent,
      and FormData cannot omit a field, so a combined multipart edit 404s for a
      member. `PhotoQuickAdd` sends `photo` alone. Section 23 of
      `scripts/test-authz.sh` checks this, multipart included.
    - **One photo, not a gallery.** Never raise `maxSelect`: an array has no
      primary element, and coming back down to one orphans every other file in
      `pb_data/storage`. Revisit as a designated primary plus ordering.
    - **Not SVG**; the browser downscales before upload (`utils/imageResize.ts`);
      thumb sizes are a closed set, and `TestPhotoThumbsAreDeclared` pins the
      400x400 the detail views use.
    - Not in the QR label, `leaf-config`, `auditSnapshotCollections`,
      `ScannerWidget` or `ThingMapDrawer`.

## Testing

- `cd ui && npm test` — Vitest: pure logic in a node environment, plus the few
  mounted components listed at the top of `vitest.config.ts` (keep that list
  short). A spec that needs a DOM starts with `// @vitest-environment jsdom`.
  Nothing renders a component in CI, so anything visual is checked by hand.
- **A form that edits a document needs a round-trip test, not a render test.**
  `SchemaBuilder` reads a JSON Schema into `Field` and writes it back, so a
  keyword `schemaToFields` does not read is deleted on the next save. A new
  builder keyword goes in `Field`, both conversion functions and the identity
  test.
- **Test the file-token cache through `fileUrl()`**, not the getter it wraps:
  every failure mode there is silent. `imageResize` keeps every decision in the
  pure `targetDimensions`, because jsdom has no `canvas.toBlob`.
- **An unknown `sort` or `filter` field is an opaque 400**, before any rule runs
  and for superusers too, so it never looks like authorization. Check sort and
  filter terms against the collection when you touch a list view;
  `./scripts/check-sort-fields.sh` checks every `sortable:` column against a
  real server. `expand` into a collection whose `viewRule` is nil is an error,
  not an empty expansion.

## Files

- `utils/fileToken.ts` - the file-token cache, the only place that mints one,
  and `fileUrl()`. `composables/useFileUrl.ts` is the reactive form, resolved
  once per record.
- `components/common/ImageUploadField.vue` - the one image picker, staged like
  every other field until Save.
- `components/common/UserAvatar.vue` / `RecordPhoto.vue` - the read-only halves.
  `users` has a viewRule that is not org-scoped, so `UserAvatar` belongs only on
  owner/admin or operator screens.
- `utils/nebula.ts` - CA rotation state, derived from the certificates, and the
  rotation call.
- `utils/managedExports.ts` - recognises the platform-provisioned helpdesk pair
  so the console shows it read-only; `hooks/managed_org_exports_test.go` reads
  it.
