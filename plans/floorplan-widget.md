# Plan: the floorplan dashboard widget

Status: **Phases 1 and 2 implemented**, not yet seen with real data. Phase 3
is untouched. No backend change was needed and none should be — no
`schema.json`, no migration, no API rule, no `scripts/test-authz.sh` check. If
this starts to need one, stop and re-read **Authorization** below, because
that is the decision being changed.

Three things moved during implementation; each is marked **Changed in
implementation** below.

## The problem

A site's live state is spatial and the console has nowhere to show it that way.
`cameras`, card readers, door contacts, environmental probes — the question an
operator asks is "which door", not "which subject", and the answer is a
position on a plan.

The console already renders that picture, at
`ui/src/components/map/FloorPlanMap.vue`, on the Location detail view. It draws
`locations.floorplan` as a Leaflet image overlay in pixel space and places the
Things whose `floorplan_position` is set, with drag-to-position behind
`manageInventory`. What it does not do is show any live data: its markers are
inventory records, not bindings. And it is not available on a dashboard, which
is the screen people actually leave open.

This plan makes that picture a widget and gives its markers live state.

## What this is not

Not the SCADA mimic. Tanks, pumps, valves and animated flow lines are a picture
of a physical process with topology — the pipe between the pump and the tank
carries the information. Cameras and door readers are not connected by flow;
they are correlated by **place** and by **time**. A floorplan is the place
half, and it is the half this platform already has data for. If a process
mimic is ever wanted, it is a separate plan and a separate widget.

## Decisions already made

These are settled. Do not re-open them while implementing.

- **The image comes from a Location. There is no custom upload.** Picking a
  location is not merely the convenient option, it is what makes the rest of
  this plan small: the location carries the image, the positions
  (`things.floorplan_position`), and the editor that maintains them. A custom
  image has none of those. It would need a file home — dashboards live in
  `localStorage` or a NATS KV bucket, and neither should hold a 2 MB PNG — so a
  new collection, schema change and rules; and it would need positions of its
  own, which means a drag-and-place editor inside the config panel. One choice
  drags in the two largest things this plan avoids. The escape hatch for
  anything that is not a building is to make a Location of a suitable type and
  upload the picture as its floorplan: a site map, a rack elevation, a yard
  layout and a process diagram are all just images with Things placed on them.

- **Markers are inventory-driven. The widget stores no positions.** The widget
  config names a location; the markers are that location's Things that have a
  `floorplan_position`. The Location detail view stays the one place a position
  is set, so there is exactly one copy of where a device is, and a dashboard
  cannot drift from the floor. This is also why the widget needs no placement
  editor: the positions already have one.

- **Bindings are per Thing Type, not per Thing.** A per-Thing binding list is
  unusable at the size this widget is for — sixty doors is sixty hand-written
  subjects, and nobody will do it twice. A per-type binding is one template
  resolved against each Thing. This is the decision the widget lives or dies
  on, and it needs no new machinery: see **Subjects** below.

- **One shared subscription per binding template, not one per marker.**
  `MapWidget` subscribes per item, which is right for ten markers and wrong for
  sixty. Marker state subscribes once per template, wildcarded, and each marker
  filters the shared buffer by its own resolved subject. Panel items keep the
  per-item subscriptions, because only one panel is open at a time.

- **No clustering, ever.** On a map, two markers a few pixels apart are noise
  worth collapsing. On a floorplan they are two different doors, and collapsing
  them hides precisely what the plan exists to show. Do not add it because
  `useLeafletMap` has it.

- **Marker icons are `L.divIcon`, not Leaflet's default pin.** The reason is
  state, not decoration. The default marker is a raster PNG fetched from unpkg
  (`ui/src/utils/leafletIcons.ts`) — it cannot be tinted, and an unattended
  `dashboard` screen should not need a CDN round-trip to draw its own markers.
  A `divIcon` holding inline SVG takes a CSS class, which is how live state
  reaches it.

## Subjects

The per-type binding is not new machinery. `ui/src/utils/subjectResolver.ts`
already implements the whole contract, and `PublisherWidget` already uses it.

A binding template is a Thing Type's `subject_prefix` joined with a suffix —
the same `join(prefix, suffix)` the Publisher widget calls, defaulting to
`{thing_type_code}.{thing}`. From one template the widget derives two things:

- **The subscription**, via `resolveRolePattern(tmpl, { org, thingTypeCode })`,
  which turns `{thing}` into `*` — one subscription covering every Thing of
  that type (`camera.*.state`).
- **Each marker's filter**, via
  `resolveThing(tmpl, { org, location, thing: thing.code, thingTypeCode })` —
  the concrete subject for that Thing (`camera.CAM-01.state`), matched against
  `BufferedMessage.subject` in the shared buffer.

Both functions exist, are documented, and have a spec file. `hasUnresolved()`
is the config panel's validation: a template still carrying a token after
resolution is one the author has not finished.

KV bindings resolve the same way, against a key template rather than a subject,
read through `useNatsKvWatcher` with a prefix — one watcher per template, not
one per marker.

## Marker state

The one genuinely new behaviour. Neither `useLeafletMap` nor `useFloorPlan`
styles a marker from data today; both only toggle a `.marker-selected` class.

A type binding may declare a **state source** — a subject or KV template plus a
JSON path — and a list of `StatusMapping` rules, reusing the status widget's
existing value→colour→label shape rather than inventing a second one. The
resolved value picks a class, the class colours the `divIcon`, and the mapping's
label becomes the marker tooltip. Staleness reuses `StatusWidgetConfig`'s
`stalenessThreshold` / `staleColor`: a camera that stopped publishing must not
keep showing green.

`useFloorPlan` gains one function for this, roughly
`setMarkerState(id, className)`, alongside its existing `setSelected`. About
fifteen lines plus CSS.

## The marker panel

Reused from the map widget essentially whole. `MarkerDetailPanel` dispatches on
`item.type` to four components that import no Leaflet and know nothing about
coordinates; `MarkerItemEditor` is their config side. Both move over unchanged.
Only the panel footer is map-specific — it prints `marker.lat`/`marker.lon` —
and becomes conditional.

**Items to add, and why.** These extend the shared `MapItemType` union, so the
map widget gets them too; that is a feature, not a side effect. Adding members
to the union is backward-compatible with every saved dashboard, removing one is
not.

- **`slider`** — yes. It is the write-side counterpart to `switch`, setpoints
  are a real need on a floorplan (a thermostat, a dimmer, a damper), and
  `SliderWidget` already carries the same `mode: 'core' | 'kv'` shape `switch`
  does, so the editor row is the one that already exists with different fields.
- **`gauge`** — yes, bar style only. `GaugeWidgetConfig.style: 'bar'` is a
  horizontal level over a range with zones, which is a panel row. The arc wants
  a widget's worth of space and will look wrong in a popup.
- **`chart` / sparkline** — no. ECharts inside a marker popup is a performance
  trap on a screen that may hold sixty markers, and the question a trend answers
  is not the question somebody clicking a door on a plan is asking. Trends live
  on the Thing detail view; the panel can link there.
- **`status`** — no, deliberately. The marker's own colour is the status
  display. A panel row repeating it says the same thing twice and invites the
  two to disagree.

Keep `MAX_ITEMS_PER_MARKER` at 10.

**Changed in implementation.** A binding needs the same value-to-colour rows
the status widget has, so they were extracted from `ConfigStatus.vue` into
`config/StatusMappingEditor.vue` and both use it. The markup is unchanged and
only the model moved, from a flat form field to a prop. A second hand-written
copy is how two editors of one thing drift -- one grows a colour the other
does not offer -- while a dashboard saved in either has to render in both.

## Limits

`MAP_LIMITS` does not transfer: it is sized for hand-placed markers. A
`FLOORPLAN_LIMITS` alongside it, with markers capped around 200 — high enough
that a real building fits, low enough that a mis-scoped location does not open
hundreds of KV watchers. The per-template subscription design is what makes a
number that size affordable at all; without it this cap would have to be ~30.

## Authorization

No rule changes, and that is a decision rather than an omission.

The widget renders a building plan and its fixtures on a dashboard, and
dashboards are visible to `dashboard` — the zero-authority appliance role,
which the console otherwise does not let browse inventory at all. The API
permits the reads: reads are org-scoped, not role-scoped, and the root
`CLAUDE.md` says that is deliberate. The console restriction on `dashboard` is
navigation, not secrecy.

So this is accepted, with two conditions. First, it is written down here, so
nobody later reads the role table and concludes the widget is a bug. Second,
the config panel says plainly which location's plan is being published, because
hanging a floor plan on a lobby screen is a physical-security choice the
operator should make on purpose rather than discover.

If that trade is ever refused, the fix is a capability check inside the widget
that renders an empty state for roles without `viewInventory` — not a rule
change, which would break the Location detail view too.

## Traps

- **The floorplan file is protected, and the file token is not reactive.**
  `locations.floorplan` is `protected: true` like every file field (feature 18),
  so its URL needs a token minted by `POST /api/files/token`, cached for 180
  seconds and deliberately non-reactive. Resolve the image URL through
  `fileUrl()` **at init time and never cache the resolved URL across
  re-inits**. A widget that re-initialises an hour later with a stale URL gets a
  404 with nothing in the console. The token cache mints a fresh one; the bug is
  holding onto the string.

- **The XSS rule lives in the wrong file.** `useLeafletMap.ts` carried the note
  that Leaflet assigns a string tooltip straight to `innerHTML`, so nothing
  there hands Leaflet a string -- labels go through `textElement()`.
  `useFloorPlan` was safe only because it used `title:`, an attribute, and
  bound no tooltips.

  **Changed in implementation.** `textElement` moved to `utils/leafletText.ts`
  so both composables import it. More to the point, the rule turned out to be
  enforced already, by `composables/leafletContentSinks.spec.ts`, which bans
  `L.divIcon(` outright -- its `html` option is the same sink. The marker icon
  therefore builds its SVG from DOM nodes and passes the ELEMENT, which
  Leaflet 1.9 appends rather than assigning (`options.html instanceof
  Element`). `utils/leafletIcon.ts` is the one file allowed to call `divIcon`,
  the same shape as `textElement(` being the one allowed tooltip argument; a
  string `html` is still refused everywhere, that file included.

- **Copy MapWidget's init dance verbatim.** Every line of it is a bug somebody
  already found: `tryInit` retrying while `clientHeight === 0` (a widget in a
  grid cell is unsized at mount), the `unmounted` flag so a delayed timer cannot
  build a map after `cleanup()`, `invalidateSize()` at 100 ms and 300 ms, and a
  `ResizeObserver` on the container's parent.

- **Watch config by value, not deeply.** `MapWidget.vue:368` compares a
  `JSON.stringify` of the view settings because a deep watch fired whenever the
  config object was replaced — a shared dashboard's KV revision arriving, say —
  and tore the map down mid-load. A floorplan widget on a shared dashboard hits
  that identical bug.

- **There are already two `renderMarkers`.** `useLeafletMap`'s and
  `useFloorPlan`'s are both "add markers to a layer group, bind click, track
  instances, toggle a selected class", differing on input type, draggable and
  tooltips. Do not merge them for this: the widget becomes a second *caller* of
  `useFloorPlan`, not a third copy. If a third substrate ever appears, that is
  the moment to look again.

## Phases

**Phase 1 — the widget.** Location picker, image overlay, inventory markers,
per-type state bindings, marker colouring, and the existing four panel item
types. Roughly 350 new lines against roughly 1,300 reused: the widget shell
(~150, mostly the copied init dance), the config panel (~150, `ConfigMap`
minus its geographic sections), `setMarkerState` (~15), the `widgets.ts` entry,
and a gallery fixture.

**Phase 2 — the two new item types.** `slider` and `gauge`, shared with the map
widget. **Done.**

**Changed in implementation.** A third shared piece came out of this:
`utils/gaugeScale.ts`, which now owns where a value sits in its range, which
zone it lands in, and how it is written down. The widget and the panel row
both draw from it, so a bar in a 280px panel and an arc on a dashboard cannot
disagree about which band a value is in — and it is the only half of a gauge
any test can reach.

The slider row deliberately does NOT share code with `SliderWidget`.
`useSwitchState` exists because switch state is genuinely subtle: a missing key
is the configured default, a toggle is `pending` until confirmed or five
seconds pass. A slider has none of that — reading is "parse a number", writing
is one publish or one KV put — and the widget’s other 500 lines are its retry
button, card layout, status line and refresh handling, none of which a panel
row has. `MarkerItemKv` does not share with `KvWidget` for the same reason.
Extract if slider semantics ever grow switch’s subtlety, not before.

One asymmetry worth knowing: with no data, the **gauge shows an em dash** and
the **slider shows its configured default**. That is deliberate. A gauge is a
reading and must not invent one; a slider is a control, and the number on it is
the value it would write, which is true before anything has been read. Both
match the widgets they are named after.

**Phase 3 — only if asked.** A per-Thing override of a type binding, for the one
odd device. And `thing_types.icon`, which is the right eventual home for a
per-type symbol but is a schema change plus a migration, and should be justified
by showing up on the Location view and in lists too — not by this widget alone.

## Files

New:

- `ui/src/components/widgets/FloorPlanWidget.vue`
- `ui/src/components/dashboard/config/ConfigFloorPlan.vue`
- `ui/src/components/dashboard/config/TypeBindingEditor.vue` — the per-type
  binding rows; wraps the existing `MarkerItemEditor` for the panel items

Changed:

- `ui/src/types/dashboard.ts` — `'floorplan'` in `WIDGET_TYPES`, a
  `FloorPlanWidgetConfig`, its `WidgetConfigByType` entry, `FLOORPLAN_LIMITS`,
  and `'slider' | 'gauge'` added to `MapItemType`
- `ui/src/components/dashboard/widgets.ts` — one entry
- `ui/src/composables/useFloorPlan.ts` — `setMarkerState`, `divIcon` markers,
  and render from a binding-aware input rather than `Thing[]` alone
- `ui/src/components/widgets/map/MarkerDetailPanel.vue` — conditional footer,
  two new item types
- `ui/src/components/dashboard/config/MarkerItemEditor.vue` — two new item types
- `ui/src/views/dev/widgetFixtures.ts` — a fixture

## Testing

Vitest here is pure logic in a node environment and nothing renders in CI, so
the test plan is about what can be asserted without a DOM:

- template → wildcard subscription subject (`resolveRolePattern`)
- template → per-marker subject (`resolveThing`), including a Thing with no
  code and a type with no `subject_prefix`
- buffer message → which marker it belongs to
- value → state class, including the staleness branch
- the config round trip, the way `dashboard import/export` is already covered

`AddWidgetModal.spec.ts` already asserts the palette is complete, and
`Record<WidgetType, WidgetDefinition>` makes a missing entry a compile error.

The `/dev/widgets` fixture is the only thing that will ever look at this
rendering, since it mounts with no NATS and no login. Give it a marker in each
state.

Nothing to add to `scripts/test-authz.sh`: no rule changes. Do not add a check
that proves the existing rules still work — it would pass for the wrong reason
and the `dashboard` denial probes already cover the collections involved.
