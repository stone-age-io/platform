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
- **PocketBase JS SDK** for API client

### Two dependencies are deliberately held back

Everything else in `ui/package.json` tracks latest. These two do not, and a
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
- **TypeScript 6, not 7.** TS 7 is the native port and no longer exposes the
  `./lib/tsc` subpath that `vue-tsc` resolves at startup, so `npm run build`
  dies before type checking. `vue-tsc` 3.3.11 still resolves it; its only TS 7
  accommodation is aliasing `typescript` to `@typescript/typescript6`, which
  runs the TS 6 compiler under a TS 7 name. 6.0 is the ceiling until the Vue
  tooling type-checks with TS 7 itself.

Holding Tailwind 3 back has a security cost: it keeps vulnerable build-time
dependencies in the tree. `overrides` in `ui/package.json` forces
`postcss-selector-parser` to `^7.1.6` (GHSA-rj75-hqrm-r3gf) under Tailwind 3,
which asks for `^6`. 7.0.0's one breaking change is "insertions during iteration
are safe". When the override went in, all 27 built CSS files came out
byte-identical with and without it, and that comparison is the only evidence it
is safe. Repeat it after any Tailwind config change. Remove the override with
the Tailwind 4 migration. `braces` (via `chokidar`/`micromatch`) also shows in
`npm audit`, but no fixed release exists. All of these parse only our own CSS
and globs, at build time.

The only automated guard is the `Assert the deliberately pinned majors` step in
`.github/workflows/ci.yml`, which fails the build if any of these three package
majors moves. It also asserts `maplibre-gl` 6, which is current rather than held
back, because its failure mode is the least visible (see the Leaflet entry above). That catches the upgrade and nothing else: Vitest is pure logic plus
five mounted components (see **Testing**), so nothing exercises a rendered map,
a theme token or a built bundle — `vue-tsc && vite build` stays green while the
UI renders wrong, whatever the versions say. The nearest thing to an eye on it
is `/dev/widgets` (see **The widget gallery**), which renders every widget
against fixtures in a dev build and is looked at by a person, not by CI. That is
exactly why these are written down rather than left to be rediscovered.

## Directory structure

```
ui/
├── public/             # PWA assets (manifest, service worker, icons)
├── src/
│   ├── main.ts         # Vue app initialization
│   ├── App.vue         # Root component
│   ├── components/     # UI components
│   │   ├── common/     # Reusable UI (ConfirmDialog, MetadataEditor, etc.)
│   │   ├── layout/     # AppHeader, AppSidebar, MainLayout
│   │   ├── ui/         # Base UI primitives (BaseCard, ResponsiveList)
│   │   ├── dashboard/  # Dashboard grid, widget containers, variables
│   │   │   └── config/ # 16 per-widget config panels + shared editors
│   │   ├── widgets/    # 16 widget type components
│   │   │   └── map/    # Map marker sub-components (detail, kv, publish, switch, text)
│   │   ├── map/        # FloorPlanMap component
│   │   ├── nats/       # NATS-specific components (KvDashboard)
│   │   └── locations/  # Location map visualization
│   ├── composables/    # Vue composables (useNatsKv, useLeafletMap, etc.)
│   ├── stores/         # Pinia stores (auth, dashboard, nats, ui, widgetData)
│   ├── router/         # Vue Router with auth guards
│   ├── types/          # TypeScript interfaces
│   ├── utils/          # Utility functions
│   └── views/          # Route page components
├── vite.config.ts      # Vite configuration
├── tailwind.config.js  # Tailwind configuration
└── package.json        # Frontend dependencies
```

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
Widget palette entry (label, icon, description, category), the two locked-mobile
behaviours, and whether the shared data-source panel applies. Adding a widget is
that entry plus the component.

It replaced six places in three directories, **two of which failed in silence** —
`MOBILE_TITLED_TYPES` and `MOBILE_EXPANDABLE_TYPES`, hardcoded `Set`s of type
strings in `WidgetContainer.vue`. Leaving a type out of either was an error
nowhere; the widget just behaved wrong on a phone, where nobody developing it
was looking. A seventh was the palette in `AddWidgetModal.vue`, sixteen
hand-written `<button>` blocks, where the omission is total: a widget missing
from it cannot be added at all.

- **It is a map, not a plugin system**, and must stay one. No `import.meta.glob`,
  no manifest, no self-registration, no schema-driven form generation. Sixteen
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
dashboard. 24,000 of this console's lines are the dashboard and its widgets and
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

## Switching organization remounts the page

`MainLayout` keys its `<router-view>` on the active organization, so a switch
throws the page away and mounts a fresh one: page one, empty search, its own
`onMounted` load. **Do not give a page, or anything inside one, its own
org-change handling** — no listener, no `watch` on `currentOrgId`. A `window`
event once did this job through 18 hand-copied handlers, and they had drifted:
only one reset the page number, so page 2 of a long list showed an empty page in
a shorter org. Things that outlive the page and care about the org (the NATS
reconnect in `App.vue`) watch `currentOrgId` themselves. The switch re-runs the
router guard against the new role, so a page the role cannot reach goes to `/`.
A record page (any route with a param) goes to the list its path sits under
instead of remounting, since its record belongs to the org being left; and
logout remounts nothing. `MainLayout.vue` says why.

## Reading one KV key

Watch a single key with `useKvKey(bucket, key)` (`composables/useKvKey.ts`),
never with a hand-rolled `Kvm` open/get/watch. Four widgets carried their own
copy and had drifted apart on what a deleted key means, on reading every value
twice, and on whether a restart could race the one it replaced. The header of
that file says why it does `get` before `watch`. A key prefix is a different
job: `useNatsKv` (the twin and the KV browser) or `useNatsKvWatcher` (tables and
maps). It was checked against a real `serve --nats` server rather than a mock,
since everything it decides depends on what the server sends.

## Features

The numbers match the feature index in the root `CLAUDE.md`.

5. **Resource Inventory** - Things and Locations with type definitions and metadata
    - **`metadata_schema` is a form hint, not a contract.** Nothing on the
      server validates `metadata` against it, a record may carry keys the type
      does not list, and the form never blocks a save on it (no `required`,
      `min` or `max` attribute reaches the input). A rule that one client
      enforces and the API, the CLI and `POST /api/org/things` do not is half a
      rule, and half a rule invites someone to finish it in a hook. Do not add
      server-side validation without deciding what happens to records saved
      before a schema tightened. The editor splits one document between typed
      fields and key/value rows (`ui/src/utils/metadataDoc.ts`); any key that
      falls between the two halves is deleted on the next save, which is what
      `metadataDoc.spec.ts` pins.

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

17. **QR labels** (`ui/src/components/common/QrLabelModal.vue`) - print a
    label from any thing or location that has a code. It takes a
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
    - **Everything printed is the organization's own data** — its code (its name
      only if it has none), then the record's code and name. **No operator
      brand**: a label belongs to one organization, and the deployment-wide
      brand says nothing true about who owns or services that device. The org
      *code* rather than the name because a thing code is unique only within its
      org, so the org code is what makes `AHU-1` unambiguous to a tech who
      services several customers — and it is immutable, on a sticker that stays
      put for years. This reverses the first version, which printed the brand
      and left the tenant off as reconnaissance. **No type**: the name already
      says what the thing is, and the small stock has no line to spare.
    - The code's point size is **fitted per label** to its text column
      (`ui/src/utils/labelFit.ts`), so a short code prints large rather than every
      code printing at the size the longest one needs.
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
      take the full width, and clicking the plate opens the stored image with no
      thumb parameter at all. The full-size URL resolves only on open, so a page
      with several photos does not fetch full copies of images nobody clicked.
      The edit forms put it in the same place, beside Name and Description.
    - **The empty slot is for writers only.** With `can.manageInventory` and no
      photo, the gutter holds `PhotoQuickAdd`: a dashed plate (not a skeleton,
      which reads as loading) that uploads one photo and saves at once, like
      MetadataCard's quick edit. Anyone else still gets nothing there. Its body
      is `photo` and nothing else, because multipart cannot omit a field and
      the member branch of `things.updateRule` freezes several -- section 23 of
      `scripts/test-authz.sh` is the assertion it relies on. It shares
      `ImageUploadField` with the forms and nothing more: the forms stage until
      Save, the detail view does not, and one component switching on a prop
      would be the mode flag `RecordPhoto` shed. Add only; replacing or removing
      a photo stays on the form.
      `RecordPhoto` takes three props and no more: the thumb size, the plate
      size and the click-to-zoom were props until both call sites turned out to
      pass identical values for all three, which is a constant wearing a prop's
      clothes. Contrast `UserAvatar`, whose eight call sites use five different
      sizes and three different fallback initials -- that is what a prop that
      has earned its place looks like.
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

## Testing

- `cd ui && npm test` — Vitest. Pure logic, node environment, with five
  mounting exceptions listed at the top of `vitest.config.ts`: `ConfirmDialog`
  and `QrLabelModal`, where the DOM contract IS the subject; `ThingFormView` and
  `NebulaHostFormView`, where the UI meets provisioning and the submitted body
  is not a shape the compiler checks; and `AddWidgetModal`, because a widget
  missing from the palette cannot be added at all. Keep that list short. Covers
  the pure logic `vue-tsc && vite build` cannot protect:
  `twinDrift`, `useSubscriptionManager` (including the per-widget system-subject
  filter), `useEscapeKey`, the `can` capability
  map, dashboard import/export, `createDefaultWidget`, the JSON Schema
  round trip in `schemaFields` + `inferSchema`, the metadata document split
  between schema fields and free rows in `metadataDoc`, the file-token cache,
  `targetDimensions`, the label code sizing in `fitCodePt`, and every decision the chart widget makes about its data
  (`utils/chartSeries` — series extraction, timeline segments — plus the
  `duration` and message `timestamp` parsers). A spec that needs a DOM opts in
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

## Files

- `ui/src/utils/fileToken.ts` - the file-token cache and the only place that mints one, plus `fileUrl()` for imperative callers. Every uploaded file is protected, so a URL without a token is a guaranteed 404. Deliberately non-reactive: see feature 18
- `ui/src/composables/useFileUrl.ts` - the reactive twin, for a component rendering straight from a prop. Resolves once per record, never per token rotation
- `ui/src/components/common/ImageUploadField.vue` - the one image picker, replacing three hand-rolled copies that had already drifted (one leaked object URLs, one had no removal path at all). Staged like every other field: choosing and removing both take effect on Save. Its `emptyLabel` prop exists because the `#fallback` slot let that same drift straight back in: four of five callers were overriding the slot with this component's own default span and one noun changed, and one had already lost the `px-2 text-center` off it. A slot whose every use is the default plus a word wants to be a prop
- `ui/src/components/common/UserAvatar.vue` / `RecordPhoto.vue` - the read-only halves. UserAvatar is for people (initial-circle fallback, and a viewRule that is NOT org-scoped, so it belongs only on owner/admin or operator screens); RecordPhoto is for things and locations
- `ui/src/utils/nebula.ts` - rotation state derived from the CA's certificates (never a stored status), the rotation call, and the `/32` audit fetch. The audit swallows its own failure and returns an empty set: it drives an advisory badge, and a list view that refused to render because an advisory endpoint was down would be the worse outcome
- `ui/src/utils/managedExports.ts` - names the platform-provisioned export/import pairs (the managed helpdesk pair and the inventory-events pair) so the console can present them read-only; mirrors the names in `hooks/managed_org_exports.go` and `hooks/inventory_exports.go`, and the tests beside each read this file to keep them honest
