# Contributing to the console

The console is the Vue 3 SPA in `ui/`. It is built into `pb_public/` and served
by the `stone-age` binary, so in production it is one more thing inside the Go
process; in development it runs on its own.

`ui/CLAUDE.md` is the long-form reference — the stack, the pinned dependencies
and why, the router guard, the org-switch remount, the digital twin. This file
is the short version of the two things people actually come here to do: get it
running, and add a widget.

## Running it

```bash
cd ui
npm ci
npm run dev
```

That serves the console on <http://localhost:5173> and proxies `/api`, `/_` and
`/branding` to a PocketBase on `127.0.0.1:8090`, so you need the platform
running next to it. The root `README.md` has the five first-run commands; after
that it is one:

```bash
./stone-age serve --nats
```

You can skip all of it for widget work — see the gallery below.

Two commands worth knowing before you file a bug:

```bash
npm test          # vitest
npm run build     # vue-tsc && vite build — the type check is the first half
```

`npm run build` writing to `../pb_public` is deliberate. Do not commit what it
writes.

## The widget gallery

```
npm run dev, then http://localhost:5173/dev/widgets
```

Every widget type, rendered against fixtures, with no NATS connection, no login
and no dashboard. It is the fastest way to see what a change did to a widget,
and it is where you check your own before you open a PR.

- The route is registered inside `if (import.meta.env.DEV)` in
  `src/router/index.ts`, so it is dropped from a production build rather than
  merely hidden.
- Fixtures live in `src/views/dev/widgetFixtures.ts`, one per type. The record
  is `Record<WidgetType, …>`, so a new widget type with no fixture does not
  compile.
- Widgets fed by the message buffer (text, stat, gauge, chart, console, stream
  table, status) render with plausible data. Widgets that read a KV bucket or a
  camera render their empty state, because the gallery mocks no server. Each
  fixture's `note` says which it is, so an empty panel is explained rather than
  suspicious.

## Adding a widget

Two files. It used to be six, and two of those failed silently.

### 1. The component

`src/components/widgets/MyWidget.vue`. It takes one prop:

```vue
<script setup lang="ts">
import type { WidgetConfigOf } from '@/types/dashboard'

const props = defineProps<{
  config: WidgetConfigOf<'mywidget'>
  layoutMode?: 'standard' | 'card'
  isFullscreen?: boolean
}>()
</script>
```

`WidgetConfigOf<'mywidget'>` is the narrowed shape, so `props.config.mywidgetConfig`
is present rather than asserted with `!`.

Data comes from the widget data store, keyed by the widget's id — not from a
NATS subscription you open yourself:

```ts
const dataStore = useWidgetDataStore()
const buffer = computed(() => dataStore.getBuffer(props.config.id))
```

For a single KV key, use `useKvKey(bucket, key)`; for a prefix, `useNatsKv` or
`useNatsKvWatcher`. Four widgets once carried their own copy of the KV logic and
had drifted apart on what a deleted key means — `ui/CLAUDE.md` has the detail.

### 2. One entry in `WIDGETS`

`src/components/dashboard/widgets.ts`:

```ts
mywidget: {
  component: MyWidget, config: ConfigMyWidget,
  label: 'My Widget', icon: '🧩', description: 'What it shows', category: 'Data Display',
  mobileTitled: false, mobileExpandable: false,
  standardDataSource: true, multiSubject: false,
},
```

That one entry is the renderer, the config panel, the Add Widget palette entry,
the two locked-mobile behaviours and the data-source panel. Declaration order is
the palette's order.

### What the compiler then makes you do

Adding `'mywidget'` to `WIDGET_TYPES` in `src/types/dashboard.ts` turns every
`Record<WidgetType, …>` in the console into a build error until you fill it in.
That is the checklist, and it is the reason this page is short rather than a
thing to remember:

| Build error in | What it wants |
|---|---|
| `WidgetConfigByType` | the interface for your config object |
| `WIDGET_DEFAULTS` | what a newly added widget starts as, including a title |
| `DEFAULT_WIDGET_SIZES` | its grid footprint |
| `WIDGETS` | the entry above |
| `widgetFixtures.ts` | a gallery fixture |

A config panel goes in `src/components/dashboard/config/ConfigMyWidget.vue` and
takes `form` and `errors` as props. If it needs form fields of its own, add them
to `WidgetFormState` in `src/types/config.ts` and give your type a handler in
`src/composables/useWidgetForm.ts` — `hydrate` reads the saved widget into the
form, `buildUpdates` writes the form back out, `validate` adds errors. Each
handler sees its own narrowed widget type.

Then open `/dev/widgets` and look at it.

### Two things that are deliberately not here

- **There is no plugin system.** No `import.meta.glob`, no manifest, no
  self-registration. `WIDGETS` is an explicit map and is meant to stay one —
  sixteen widget types that change twice a year do not need a framework, and the
  map is less code than the switch it replaced.
- **Widget config field names are frozen.** They are `chartConfig`,
  `gaugeConfig` and so on because every dashboard ever saved — in localStorage,
  in a KV bucket, in an exported JSON file somebody is keeping — has those keys
  in it. Renaming them is a data migration, not a refactor.

## Tests

`npm test` runs Vitest in a node environment. The suite is pure logic, with five
mounted components listed at the top of `vitest.config.ts`. That is a deliberate
shape, not a gap waiting to be filled:

- **Prefer a pure test.** Mounting a component to check what it renders tests
  the rendering, not the decision behind it. Most of what can go wrong here —
  widget defaults, threshold evaluation, the capability map, dashboard
  import/export, chart series extraction — is reachable without a DOM.
- **A spec that needs a DOM opts in** with `// @vitest-environment jsdom` on its
  first line.
- **What earns a mount** is a failure that is both silent and total: a dialog
  with no focus trap, a form that submits a body the API will refuse, a palette
  a widget has fallen out of. Not coverage.

If you are changing one of the provisioning forms, read
`src/views/things/ThingFormView.spec.ts` first. The bodies those forms send are
not shapes the compiler checks, and the member branch of `things.updateRule`
turns a field you merely *included* into a 404.

## Conventions

- `<script setup lang="ts">`, Composition API, strict TypeScript.
- Components `PascalCase`, composables `useThing`, views end in `View`.
- Types live in `src/types/`.
- **Do not add org-change handling to a page.** `MainLayout` keys its
  `<router-view>` on the active organization, so switching remounts the page.
  Eighteen hand-copied handlers used to do this and had drifted.
- Tailwind 3 + daisyUI 4 and TypeScript 6 are **pinned**. `ui/CLAUDE.md` says
  why at length; a routine "bump everything" pass must not drag them along, and
  CI fails if a major moves.
