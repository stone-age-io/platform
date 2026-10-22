# Plan: make the console easier to contribute to

Status: **done** (2026-10-21). All five steps landed in one pass. Two things
went beyond what is written below:

- **The Add Widget palette joined `WIDGETS`.** It was a seventh touchpoint this
  plan did not count — sixteen hand-written `<button>` blocks — and the one
  whose omission is total, since a widget missing from it cannot be added at
  all. It now renders from the same object.
- **Config panels became lazy.** Folding them into `WIDGETS` would otherwise
  have pulled twenty editor components into the chunk that renders a dashboard
  nobody is editing, so they are `defineAsyncComponent` now and ship as their
  own chunks.

The step 4 note below said "narrow `config` by `type`". That was done by
splitting `WidgetConfig` into a union keyed on `type` while **keeping** the
sixteen `xxxConfig` field names: renaming them to a single `config` would
rename keys inside every dashboard ever saved to localStorage, to a KV bucket
or to an exported JSON file. That is a data migration, not a refactor.

The rest of this file is the plan as written, kept for the reasoning.

## Where the weight is

```
ui/src total            57,881 lines across 242 files
  components/           26,922
  views/                17,693
  composables/           5,769
  utils/                 3,084
  stores/                2,289
  types/                 1,665

dashboard + widgets     24,359   (42% of the console)
  components/dashboard   8,416
  components/widgets     8,045
```

Tests: 29 files, pure logic only. Nothing renders a component in CI, so every
visual regression is caught by hand or not at all. That is the risk this plan
addresses — not file size, and not architecture.

## The actual barrier

Adding one widget today means touching six places across three directories:

1. `components/widgets/NewWidget.vue`
2. the 16-case `switch` in `components/dashboard/WidgetContainer.vue`
3. a `NewWidgetConfig` interface in `types/dashboard.ts`
4. a 17th optional `newConfig?:` field on the widget interface — there are
   already 16 stacked at `types/dashboard.ts:461`
5. a `ConfigNew.vue` in `components/dashboard/config/` (there are 20)
6. `composables/useWidgetForm.ts`, 1,014 lines

Plus `MOBILE_TITLED_TYPES` and `MOBILE_EXPANDABLE_TYPES`, two hardcoded string
sets in `WidgetContainer.vue`. Forget those and nothing errors — the widget just
behaves wrong on a phone.

Six touchpoints, two of which fail silently, and a type system that cannot tell
you about any of them. That is the barrier. Not that the files are long.

## Steps

### 0. Write down how to add a widget

A section in `ui/CONTRIBUTING.md` listing the six places in order, with both
silent string sets called out by name.

An hour of work, no risk, and useful to the next person immediately. Writing it
is also what makes the duplication impossible to keep ignoring. Documenting the
mess before refactoring it is the cheapest move available.

*Done when:* someone who has never opened this repo can add a trivial widget by
following the list.

### 1. A `/dev` route that renders every widget against fixtures

One route, one fixture file per widget, behind a dev-only flag so it never
ships. Not Storybook, not a framework — a page with sixteen widgets on it.

This is the highest value per hour in the plan. It puts eyes on the 24,359 lines
that currently have none, and it makes steps 3 and 4 safe to attempt.

*Done when:* `npm run dev`, open `/dev`, and every widget renders with plausible
data without touching NATS.

### 2. Render tests on the form views only

`views/things/ThingFormView.vue` (1,002 lines) and
`views/nebula/NebulaHostFormView.vue` (826 lines). These are where the UI meets
provisioning, and where a silent regression stops somebody creating a device.

Use the Vitest setup that already exists. Ten components tested well beats 242
tested badly, and the forms are the ten that matter.

*Done when:* CI fails if either form stops rendering its required fields or
stops submitting the shape the API expects.

### 3. One `WIDGETS` object

Replace the `switch` and both string sets with a plain object literal:

```ts
export const WIDGETS = {
  gauge: { component: GaugeWidget, config: ConfigGauge, mobileTitled: true,  mobileExpandable: false },
  map:   { component: MapWidget,   config: ConfigMap,   mobileTitled: true,  mobileExpandable: true  },
  // one line per widget
}
```

**This is a map, not a plugin system.** No `import.meta.glob`, no manifest
files, no self-registration, no config-schema-driven form generation. Those
would be a framework to maintain forever for sixteen widgets that change twice
a year. The object is less code than what it replaces and the whole widget
surface fits on one screen.

After this, adding a widget is two files: the component, and one line here.

*Done when:* `WidgetContainer.vue` has no `switch` and no `Set`, and `/dev`
still renders everything.

### 4. Discriminated union on widget config

Sixteen optional `xxxConfig?` fields, each used by exactly one widget type,
means TypeScript cannot help anybody. Narrow `config` by `type` instead, so the
compiler becomes the checklist and the switch becomes exhaustiveness-checked.

**Last, deliberately.** This is the largest mechanical diff in the plan, and
attempting it before steps 1 and 2 exist means no way to tell whether anything
broke. Order matters more than the change does.

*Done when:* adding a widget type without handling it everywhere is a build
error rather than a runtime surprise.

## Not doing

- **No plugin system.** Covered above. The explicit map is the whole mechanism.
- **No splitting files to hit a line target.** `ThingFormView.vue` at 1,002
  lines in one place is easier for a newcomer than 1,002 lines across eight
  files they have to find first. Scattering is the problem; length is not.
- **No Storybook.** The `/dev` route gets most of the benefit for none of the
  dependency.
- **No extracting the dashboard into its own repo.** It is the obvious seam, and
  it halves the console if the size ever becomes the problem rather than the
  structure. It is not that today, and the live NATS dashboard does something
  Grafana cannot.
- **No component tests beyond the forms.** Coverage targets would turn this into
  a chore nobody finishes.

## If only two things get done

Steps 0 and 1. Between them they remove most of the barrier to a contribution
and most of the risk of a silent regression, and neither can break anything that
currently works.
