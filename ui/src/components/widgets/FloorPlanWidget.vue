<template>
  <div class="floorplan-widget">
    <div :id="containerId" class="floorplan-container" @click="handleBackgroundClick" />

    <!-- One renderless state source per binding. -->
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

    <div v-if="ready" class="fp-controls">
      <!-- The same glyph as the map widget's "Fit all markers". -->
      <button class="fp-control-btn" title="Fit the whole plan" aria-label="Fit the whole plan" @click="fitToImage">
        <span aria-hidden="true">⊡</span>
      </button>
    </div>

    <div v-if="ready && markers.length === 0" class="fp-hint">
      <MapPin class="fp-hint-icon" :size="16" aria-hidden="true" />
      <span>No positioned things in this location</span>
    </div>

    <div v-if="ready && truncated" class="fp-hint fp-hint-left">
      <span>Showing {{ markers.length }} of {{ placed.length }}</span>
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
import { MapPin, TriangleAlert } from '@lucide/vue'
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useFloorPlan } from '@/composables/useFloorPlan'
import { useAuthStore } from '@/stores/auth'
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
  isPlaced,
  markerStateFor,
  resolveItemsForThing,
  UNKNOWN_STATE,
  type FloorPlanContext,
  type MarkerState,
  type StateSample,
} from '@/utils/floorplanBindings'
import { FLOORPLAN_LIMITS, type MapMarker, type WidgetConfigOf } from '@/types/dashboard'
import type { Location, Thing, ThingType } from '@/types/pocketbase'

/**
 * A Location's floor plan with live state on the Things placed on it.
 *
 * The widget stores no positions: markers are the Things in `locationId` whose
 * `floorplan_position` is set on the Location page. plans/floorplan-widget.md
 * has the reasoning.
 */

const props = withDefaults(defineProps<{
  config: WidgetConfigOf<'floorplan'>
  isFullscreen?: boolean
}>(), { isFullscreen: false })

const authStore = useAuthStore()
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

/** Ticks so a device that goes quiet turns stale without a message arriving. */
const now = ref(Date.now())
let clock: number | null = null

const ctx = computed<FloorPlanContext>(() => ({
  orgCode: authStore.currentOrg?.code || '',
  locationCode: location.value?.code || '',
}))

// --- markers ----------------------------------------------------------------

const placed = computed(() => things.value.filter(isPlaced))
const markers = computed(() => placed.value.slice(0, FLOORPLAN_LIMITS.MAX_MARKERS))
const truncated = computed(() => placed.value.length > markers.value.length)

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
  // Replace the Map rather than mutate it, so the computed below re-runs.
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
    out.set(thing.id, markerStateFor(binding, sample?.value ?? null, sample?.timestamp ?? null, t))
  }
  return out
})

// --- the open panel ---------------------------------------------------------

const selectedThing = computed(
  () => visibleMarkers.value.find(t => t.id === selectedThingId.value) || null
)

/** The selected Thing in the map widget's `MapMarker` shape, so the panel is reused as is. */
const selectedMarker = computed<MapMarker | null>(() => {
  const thing = selectedThing.value
  if (!thing) return null
  const binding = bindingForThing(bindings.value, thing)
  const items = binding
    ? resolveItemsForThing(
        binding.items.slice(0, FLOORPLAN_LIMITS.MAX_ITEMS_PER_BINDING),
        typeOf(thing),
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

/** The panel footer shows the Thing's code: a pixel position means nothing to a reader. */
const selectedFooter = computed(() => {
  const thing = selectedThing.value
  if (!thing) return null
  return thing.code
    ? { label: 'Code:', value: thing.code }
    : { label: 'Type:', value: typeOf(thing)?.name || 'Unknown' }
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
  // Protected file: mint a fresh token URL on every init, never keep one.
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
 * A grid cell has no height when it mounts and Leaflet sizes itself at init,
 * so retry until it has one. Same loop as MapWidget.
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
  // The cell is often still settling its size, so the first fit was measured
  // against the wrong box: refit, or the plan sits with a gap around it.
  const settle = () => {
    if (unmounted || seq !== loadSeq) return
    invalidateSize()
    fitToImage()
  }
  window.setTimeout(settle, 100)
  window.setTimeout(settle, 300)
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

/** Colour and tooltip for every marker, so the hover text matches the colour. */
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

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

// --- watchers ---------------------------------------------------------------

// Only a new location re-initialises the map. Watched by value, not deeply: a
// replaced config object (a shared dashboard's KV revision) must not tear it down.
watch(
  () => cfg.value?.locationId,
  () => {
    cleanup()
    ready.value = false
    closePanel()
    void load()
  }
)

watch(markerStates, () => paint())
watch(visibleMarkers, () => draw())

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

/* The state colour is an inline style. The fallback is a literal, as with
   NEUTRAL_MARKER, because the marker sits on the uploaded image. */
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
  font-size: 16px;
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
