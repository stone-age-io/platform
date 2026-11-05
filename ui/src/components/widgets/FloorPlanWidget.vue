<template>
  <div class="floorplan-widget">
    <div :id="containerId" class="floorplan-container" @click="handleBackgroundClick" />

    <!-- One state source per binding. Renderless; see BindingState.vue. -->
    <BindingState
      v-for="src in stateSources"
      :key="src.bindingId"
      :binding-id="src.bindingId"
      :mode="src.mode"
      :subject="src.subject"
      :kv-bucket="src.kvBucket"
      :kv-pattern="src.kvPattern"
      :json-path="src.jsonPath"
      :marker-count="src.markerCount"
      @update="samples => onSamples(src.bindingId, samples)"
    />

    <div v-if="loading" class="fp-overlay">
      <div class="fp-spinner" />
      <span>Loading floor plan…</span>
    </div>

    <div v-else-if="problem" class="fp-overlay">
      <TriangleAlert class="fp-overlay-icon" :size="22" aria-hidden="true" />
      <span class="fp-overlay-text">{{ problem }}</span>
    </div>

    <!-- Fit, same affordance the map widget offers. A plan can be panned and
         zoomed, and on a small dashboard tile it is easy to lose the sheet. -->
    <div v-if="ready" class="fp-controls">
      <button class="fp-control-btn" title="Fit the whole plan" aria-label="Fit the whole plan" @click="handleFit">
        <Maximize :size="15" aria-hidden="true" />
      </button>
    </div>

    <div v-if="ready && markers.length === 0" class="fp-hint">
      <MapPin class="fp-hint-icon" :size="16" aria-hidden="true" />
      <span>No positioned things in this location</span>
    </div>

    <div v-if="ready && truncated" class="fp-hint fp-hint-left">
      <span>Showing {{ markers.length }} of {{ placedCount }}</span>
    </div>

    <MarkerDetailPanel
      v-if="selectedMarker"
      :marker="selectedMarker"
      :is-mobile="isMobile"
      :footer="selectedFooter"
      @close="closePanel"
    />
  </div>
</template>

<script setup lang="ts">
import { Maximize, MapPin, TriangleAlert } from '@lucide/vue'
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useFloorPlan } from '@/composables/useFloorPlan'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { pb } from '@/utils/pb'
import { fileUrl } from '@/utils/fileToken'
import { floorPlanMarkerIcon } from './floorplan/markerIcon'
import BindingState from './floorplan/BindingState.vue'
import MarkerDetailPanel from './map/MarkerDetailPanel.vue'
import {
  bindingForThing,
  bindingKvKeyForThing,
  bindingKvPattern,
  bindingSubjectForThing,
  bindingSubscriptionSubject,
  markerStateFor,
  placedThings,
  resolveItemsForThing,
  UNKNOWN_STATE,
  type FloorPlanContext,
  type MarkerState,
  type StateSample,
} from '@/utils/floorplanBindings'
import { FLOORPLAN_LIMITS, type MapMarker, type WidgetConfigOf } from '@/types/dashboard'
import type { Location, Thing, ThingType } from '@/types/pocketbase'

/**
 * A Location's floor plan as a dashboard widget, with live state on the Things
 * placed on it.
 *
 * WHAT IT DOES NOT STORE: positions. The markers are the Things in
 * `locationId` whose `floorplan_position` is set, and that field is maintained
 * on the Location detail view. One copy of where a device is, so a dashboard
 * cannot drift from the floor -- and no placement editor to build here.
 * plans/floorplan-widget.md has the rest of the reasoning.
 */

const props = withDefaults(defineProps<{
  config: WidgetConfigOf<'floorplan'>
  isFullscreen?: boolean
}>(), { isFullscreen: false })

const authStore = useAuthStore()
const uiStore = useUIStore()
const {
  initFloorPlan,
  renderMarkers,
  setSelected,
  setMarkerState,
  setMarkerTooltip,
  fitToImage,
  invalidateSize,
  cleanup,
} = useFloorPlan()

const containerId = computed(
  () => `floorplan-${props.config.id}${props.isFullscreen ? '-fullscreen' : ''}`
)

const cfg = computed(() => props.config.floorplanConfig)
const bindings = computed(() => cfg.value?.bindings || [])

const loading = ref(false)
const problem = ref('')
const ready = ref(false)
const isMobile = ref(window.innerWidth < 768)

const location = ref<Location | null>(null)
const things = ref<Thing[]>([])
const typesById = ref<Map<string, ThingType>>(new Map())

const selectedThingId = ref<string | null>(null)

/** Samples per binding, keyed by resolved subject or KV key. */
const samplesByBinding = ref<Map<string, Map<string, StateSample>>>(new Map())

/**
 * A clock, so staleness is reached without a message arriving.
 *
 * Nothing else would move: a device that stops publishing produces no event, so
 * a marker computed only from incoming data keeps its last colour forever --
 * the exact failure the stale branch exists to prevent.
 */
const now = ref(Date.now())
let clock: number | null = null

const ctx = computed<FloorPlanContext>(() => ({
  // Codes, never ids: ids are for storage, codes for addressing. An org with no
  // code leaves {org} literal in the subject, visibly unresolved, rather than
  // inventing a token nothing publishes under.
  orgCode: authStore.currentOrg?.code || '',
  locationCode: location.value?.code || '',
}))

// --- markers ----------------------------------------------------------------

const placed = computed(() =>
  things.value.filter(t => {
    const p = t.floorplan_position
    return !!p && typeof p.x === 'number' && typeof p.y === 'number'
  })
)
const placedCount = computed(() => placed.value.length)
const markers = computed(() => placedThings(things.value, FLOORPLAN_LIMITS.MAX_MARKERS))
const truncated = computed(() => placedCount.value > markers.value.length)

function typeOf(thing: Thing): ThingType | undefined {
  return thing.type ? typesById.value.get(thing.type) : undefined
}

/** Things whose type has a binding, unless the config asks for all of them. */
const visibleMarkers = computed(() => {
  if (cfg.value?.showUnbound !== false) return markers.value
  return markers.value.filter(t => !!bindingForThing(bindings.value, t))
})

// --- state sources ----------------------------------------------------------

interface StateSourceProps {
  bindingId: string
  mode: 'subject' | 'kv'
  subject?: string
  kvBucket?: string
  kvPattern?: string
  jsonPath?: string
  markerCount: number
}

const stateSources = computed<StateSourceProps[]>(() => {
  const out: StateSourceProps[] = []
  for (const b of bindings.value) {
    const src = b.stateSource
    if (!src) continue
    const type = typesById.value.get(b.thingTypeId)
    const count = visibleMarkers.value.filter(t => t.type === b.thingTypeId).length
    if (count === 0) continue

    if (src.mode === 'subject') {
      const subject = bindingSubscriptionSubject(b, type, ctx.value)
      if (subject) {
        out.push({ bindingId: b.id, mode: 'subject', subject, jsonPath: src.jsonPath, markerCount: count })
      }
    } else {
      const pattern = bindingKvPattern(b, type, ctx.value)
      if (pattern && src.kvBucket) {
        out.push({
          bindingId: b.id,
          mode: 'kv',
          kvBucket: src.kvBucket,
          kvPattern: pattern,
          jsonPath: src.jsonPath,
          markerCount: count,
        })
      }
    }
  }
  return out
})

function onSamples(bindingId: string, samples: Map<string, StateSample>) {
  // Replaced rather than mutated: a Map mutated in place is the same object and
  // the computed below would not re-run.
  const next = new Map(samplesByBinding.value)
  next.set(bindingId, samples)
  samplesByBinding.value = next
}

/** Every marker's current state, recomputed when a sample or the clock moves. */
const markerStates = computed<Map<string, MarkerState>>(() => {
  const out = new Map<string, MarkerState>()
  const t = now.value
  for (const thing of visibleMarkers.value) {
    const binding = bindingForThing(bindings.value, thing)
    if (!binding?.stateSource) continue
    const type = typeOf(thing)
    const key = binding.stateSource.mode === 'subject'
      ? bindingSubjectForThing(binding, type, thing, ctx.value)
      : bindingKvKeyForThing(binding, type, thing, ctx.value)
    const sample = key ? samplesByBinding.value.get(binding.id)?.get(key) : undefined
    out.set(
      thing.id,
      sample
        ? markerStateFor(binding, sample.value, sample.timestamp, t)
        : markerStateFor(binding, null, null, t)
    )
  }
  return out
})

// --- the open panel ---------------------------------------------------------

/**
 * The selected Thing as a MarkerDetailPanel marker.
 *
 * Built here rather than stored: the panel takes the map widget's `MapMarker`
 * shape unchanged, which is what lets all four item components and their editor
 * move over untouched. `lat`/`lon` are zero and the panel's footer no longer
 * prints them for a floor plan -- see MarkerDetailPanel.
 */
const selectedMarker = computed<MapMarker | null>(() => {
  const thing = visibleMarkers.value.find(t => t.id === selectedThingId.value)
  if (!thing) return null
  const binding = bindingForThing(bindings.value, thing)
  const type = typeOf(thing)
  const items = binding
    ? resolveItemsForThing(
        binding.items.slice(0, FLOORPLAN_LIMITS.MAX_ITEMS_PER_BINDING),
        type,
        thing,
        ctx.value
      )
    : []
  return {
    id: thing.id,
    label: thing.name || thing.code || thing.id,
    lat: 0,
    lon: 0,
    items,
  }
})

/**
 * The panel's footer line. A floor plan position is a pixel offset into an
 * image, which tells a reader nothing, so the Thing's code goes there instead:
 * it is what the label on the device says and what its subjects are built from.
 */
const selectedFooter = computed(() => {
  const thing = visibleMarkers.value.find(t => t.id === selectedThingId.value)
  if (!thing) return null
  const type = thing ? typeOf(thing) : undefined
  return thing.code
    ? { label: 'Code:', value: thing.code }
    : { label: 'Type:', value: type?.name || 'Unknown' }
})

// --- loading ----------------------------------------------------------------

/** Guards an out-of-order load: a slower earlier request must not win. */
let loadSeq = 0

async function load() {
  const seq = ++loadSeq
  problem.value = ''
  ready.value = false

  const locationId = cfg.value?.locationId
  if (!locationId) {
    problem.value = 'No location selected'
    return
  }

  loading.value = true
  try {
    const [loc, list, types] = await Promise.all([
      pb.collection('locations').getOne<Location>(locationId),
      pb.collection('things').getFullList<Thing>({
        filter: pb.filter('location = {:id}', { id: locationId }),
        sort: 'name',
      }),
      pb.collection('thing_types').getFullList<ThingType>({ sort: 'name' }),
    ])
    if (seq !== loadSeq) return

    location.value = loc
    things.value = list
    typesById.value = new Map(types.map(t => [t.id, t]))

    if (!loc.floorplan) {
      problem.value = `${loc.name || 'This location'} has no floor plan image`
      return
    }
    await drawPlan(loc, seq)
  } catch (err: any) {
    if (seq !== loadSeq) return
    problem.value = err?.status === 404 ? 'Location not found' : 'Could not load the floor plan'
    console.warn('Floor plan widget load failed', err)
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

async function drawPlan(loc: Location, seq: number) {
  // `locations.floorplan` is a PROTECTED file field (feature 18), so its URL
  // carries a short-lived file token. Resolved HERE, at every init, and never
  // cached across one: the token lasts 180s and is deliberately non-reactive,
  // so a widget that re-initialised an hour later with a held URL would get a
  // 404 with nothing in the console. fileUrl() mints a fresh one; the bug is
  // keeping the string.
  const imageUrl = await fileUrl({ record: loc, filename: loc.floorplan! })
  if (seq !== loadSeq || unmounted) return
  if (!imageUrl) {
    problem.value = 'Could not read the floor plan image'
    return
  }

  const img = new Image()
  img.onload = () => {
    if (seq !== loadSeq || unmounted) return
    initWhenSized(imageUrl, img.width, img.height, seq)
  }
  img.onerror = () => {
    if (seq !== loadSeq) return
    problem.value = 'Could not read the floor plan image'
  }
  img.src = imageUrl
}

/**
 * A widget in a grid cell has no height when it mounts, and Leaflet sizes
 * itself from its container at init. Retry until it has one. The `unmounted`
 * guard is why this is a named loop and not a plain setTimeout: the widget can
 * be deleted while a retry is pending, and building a map after cleanup() has
 * run leaks it. Copied from MapWidget, where every line of it is a bug somebody
 * already found.
 */
function initWhenSized(imageUrl: string, w: number, h: number, seq: number, attempts = 0) {
  if (unmounted || seq !== loadSeq) return
  const el = document.getElementById(containerId.value)
  if (!el || (el.clientHeight === 0 && attempts < 10)) {
    window.setTimeout(() => initWhenSized(imageUrl, w, h, seq, attempts + 1), 50)
    return
  }
  initFloorPlan(containerId.value, imageUrl, w, h)
  ready.value = true
  draw()
  window.setTimeout(() => invalidateSize(), 100)
  window.setTimeout(() => invalidateSize(), 300)
}

function draw() {
  if (!ready.value) return
  renderMarkers(visibleMarkers.value, {
    draggable: false,
    onMove: () => {},
    onClick: handleMarkerClick,
    icon: () => floorPlanMarkerIcon(),
    tooltip: (thing) => tooltipFor(thing),
  })
  paint()
}

function tooltipFor(thing: Thing): string {
  const name = thing.name || thing.code || thing.id
  const state = markerStates.value.get(thing.id)
  return state ? `${name} — ${state.label}` : name
}

/**
 * Colour and label every marker from the current states.
 *
 * The label is refreshed alongside the colour rather than only at render time:
 * a tooltip built when the layer was drawn keeps naming the state the device
 * had then, which is a word that disagrees with the colour next to it.
 */
function paint() {
  if (!ready.value) return
  for (const thing of visibleMarkers.value) {
    setMarkerState(thing.id, markerStates.value.get(thing.id) || UNKNOWN_STATE)
    setMarkerTooltip(thing.id, tooltipFor(thing))
  }
}

// --- interaction ------------------------------------------------------------

function handleMarkerClick(thingId: string) {
  if (selectedThingId.value === thingId) {
    closePanel()
    return
  }
  selectedThingId.value = thingId
  setSelected(thingId)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function closePanel() {
  selectedThingId.value = null
  setSelected(null)
  if (!isMobile.value) nextTick(() => invalidateSize())
}

function handleBackgroundClick(event: MouseEvent) {
  if (isMobile.value || !selectedThingId.value) return
  const target = event.target as HTMLElement
  if (
    target.closest('.leaflet-container') &&
    !target.closest('.leaflet-marker-icon') &&
    !target.closest('.marker-detail-panel')
  ) {
    closePanel()
  }
}

function handleFit() {
  fitToImage()
}

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

// --- watchers ---------------------------------------------------------------

// Compared BY VALUE, never deeply. A deep watch here fires whenever the config
// object is replaced -- a shared dashboard's KV revision arriving, say -- and
// tears the map down mid-load for nothing. MapWidget carries the same note.
watch(
  () => JSON.stringify([cfg.value?.locationId, cfg.value?.showUnbound]),
  () => {
    cleanup()
    ready.value = false
    closePanel()
    void load()
  }
)

// Bindings change what is subscribed and painted, not what is drawn underneath,
// so they repaint rather than re-initialise.
watch(() => JSON.stringify(bindings.value), () => paint())

watch(markerStates, () => paint(), { deep: true })
watch(visibleMarkers, () => draw())
// The basemap is an image, so a theme change repaints only the markers.
watch(() => uiStore.theme, () => paint())

let resizeObserver: ResizeObserver | null = null
let unmounted = false

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  void load()

  clock = window.setInterval(() => { now.value = Date.now() }, 10_000)

  window.setTimeout(() => {
    if (unmounted) return
    resizeObserver = new ResizeObserver(() => { if (ready.value) invalidateSize() })
    const el = document.getElementById(containerId.value)
    if (el?.parentElement) resizeObserver.observe(el.parentElement)
  }, 100)
})

onUnmounted(() => {
  unmounted = true
  window.removeEventListener('resize', checkMobile)
  if (clock) clearInterval(clock)
  if (resizeObserver) resizeObserver.disconnect()
  cleanup()
})
</script>

<style scoped>
.floorplan-widget {
  position: relative;
  height: 100%;
  width: 100%;
  background: var(--widget-bg);
  border-radius: 4px;
  overflow: hidden;
}

.floorplan-container { position: absolute; inset: 0; z-index: 0; }
.floorplan-container :deep(.leaflet-pane),
.floorplan-container :deep(.leaflet-control-container) { z-index: 0 !important; }
.floorplan-container :deep(.leaflet-top),
.floorplan-container :deep(.leaflet-bottom) { z-index: 1 !important; }

/* The marker's own colour is an inline style set per state; everything here is
   shape and motion, so nothing has to know the palette. The fallback is a
   literal rather than a theme token for the reason NEUTRAL_MARKER gives: this
   glyph sits on the operator's image, not on the console's surface. */
.floorplan-container :deep(.fp-marker) {
  color: #8b9099;
  transition: color 0.3s ease;
}
.floorplan-container :deep(.fp-marker svg) { display: block; }
.floorplan-container :deep(.fp-marker.marker-selected) {
  filter: drop-shadow(0 0 6px currentColor);
}
.floorplan-container :deep(.fp-state-stale) { opacity: 0.75; }
.floorplan-container :deep(.fp-state-blink) { animation: fp-blink 1.2s steps(1, end) infinite; }

@keyframes fp-blink {
  0%, 60% { opacity: 1; }
  61%, 100% { opacity: 0.25; }
}

/* A screen somebody left running should not strobe. */
@media (prefers-reduced-motion: reduce) {
  .floorplan-container :deep(.fp-state-blink) { animation: none; opacity: 1; }
}

.fp-overlay {
  position: absolute; inset: 0; z-index: 10;
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 10px; padding: 16px; text-align: center;
  background: var(--widget-bg); color: var(--muted); font-size: 14px;
}
.fp-overlay-icon { color: var(--color-warning); }
.fp-overlay-text { max-width: 28ch; }

.fp-spinner {
  width: 32px; height: 32px;
  border: 3px solid var(--border); border-top-color: var(--color-accent);
  border-radius: 50%; animation: fp-spin 1s linear infinite;
}
@keyframes fp-spin { to { transform: rotate(360deg); } }

.fp-controls {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fp-control-btn {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
}

.fp-control-btn:hover {
  background: var(--color-info-bg);
  border-color: var(--color-info-border);
}

.fp-control-btn:active { transform: scale(0.95); }

.fp-hint {
  position: absolute; bottom: 12px; left: 50%; transform: translateX(-50%); z-index: 5;
  display: flex; align-items: center; gap: 6px;
  padding: 6px 12px; background: var(--panel); border: 1px solid var(--border);
  border-radius: 4px; font-size: 12px; color: var(--muted); pointer-events: none;
}
.fp-hint-left { left: 10px; bottom: auto; top: 10px; transform: none; }
</style>
