<template>
  <div class="binding-editor">
    <div class="binding-header">
      <div class="binding-title">
        <Shapes :size="14" aria-hidden="true" />
        <span>{{ typeName || 'Choose a thing type' }}</span>
        <span v-if="matchCount !== null" class="binding-count">
          {{ matchCount }} on this plan
        </span>
      </div>
      <button class="btn-icon danger" title="Remove binding" @click="$emit('remove')">✕</button>
    </div>

    <div class="form-group">
      <label>Thing Type</label>
      <select v-model="binding.thingTypeId" class="form-input">
        <option value="">Select a type…</option>
        <option v-for="t in thingTypes" :key="t.id" :value="t.id">
          {{ t.name }}<template v-if="t.code"> ({{ t.code }})</template>
        </option>
      </select>
      <div v-if="duplicate" class="help-text warn">
        Another binding already covers this type. The first one wins.
      </div>
    </div>

    <!-- ---------------- marker state ---------------- -->
    <div class="sub-section">
      <div class="sub-title">Marker State</div>
      <div class="help-text mb-2">
        One subscription covers every thing of this type. Use
        <code>{thing}</code> for the thing code; it is filled in per marker.
      </div>

      <div class="form-group">
        <label>Source</label>
        <select v-model="stateMode" class="form-input">
          <option value="none">None — markers draw in one colour</option>
          <option value="subject">NATS subject</option>
          <option value="kv">KV bucket</option>
        </select>
      </div>

      <template v-if="stateMode === 'subject'">
        <div class="form-group">
          <label>Subject suffix</label>
          <input
            v-model="subjectSuffix"
            type="text"
            class="form-input"
            placeholder="state"
          />
          <div class="help-text">
            Joined onto the type's own subject prefix, the same way the
            Publisher widget and the Thing page build a subject.
          </div>
        </div>
      </template>

      <template v-else-if="stateMode === 'kv'">
        <div class="form-row">
          <div class="input-group">
            <label>Bucket</label>
            <input v-model="kvBucket" type="text" class="form-input" placeholder="twin" />
          </div>
          <div class="input-group">
            <label>Key template</label>
            <input
              v-model="kvKeyTemplate"
              type="text"
              class="form-input"
              placeholder="thing.{thing}.state"
            />
          </div>
        </div>
      </template>

      <template v-if="stateMode !== 'none'">
        <div class="form-group">
          <label>JSON path (optional)</label>
          <input v-model="jsonPath" type="text" class="form-input" placeholder="$.state" />
        </div>

        <!-- The resolved form, so an author sees what will actually be watched
             rather than discovering it from an empty plan. -->
        <div v-if="preview" class="preview">
          <div class="preview-row">
            <span class="preview-label">Watches</span>
            <code class="preview-value">{{ preview.pattern }}</code>
          </div>
          <div v-if="preview.example" class="preview-row">
            <span class="preview-label">e.g.</span>
            <code class="preview-value">{{ preview.example }}</code>
          </div>
          <div v-if="preview.unresolved" class="help-text warn">
            {{ preview.unresolved }}
          </div>
        </div>

        <StatusMappingEditor :mappings="mappings" />

        <div class="form-row">
          <div class="input-group">
            <label>Default colour</label>
            <select
              v-model="defaultColor"
              class="form-input"
              :style="{ color: defaultColor }"
            >
              <option value="var(--color-info)" style="color: var(--color-info)">Info</option>
              <option value="var(--muted)" style="color: var(--muted)">Muted</option>
              <option value="var(--color-success)" style="color: var(--color-success)">Success</option>
              <option value="var(--color-warning)" style="color: var(--color-warning)">Warning</option>
              <option value="var(--color-error)" style="color: var(--color-error)">Error</option>
            </select>
          </div>
          <div class="input-group">
            <label>Default label</label>
            <input v-model="defaultLabel" type="text" class="form-input" placeholder="Unknown" />
          </div>
        </div>

        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="showStale" />
            <span>Grey out when nothing has arrived recently</span>
          </label>
          <div class="help-text">
            A device that stops publishing produces no message, so without this a
            marker keeps its last colour indefinitely.
          </div>
        </div>

        <template v-if="showStale">
          <div class="form-row">
            <div class="input-group">
              <label>After (ms)</label>
              <input
                v-model.number="stalenessThreshold"
                type="number"
                class="form-input"
                min="1000"
                step="1000"
              />
            </div>
            <div class="input-group">
              <label>Stale label</label>
              <input v-model="staleLabel" type="text" class="form-input" placeholder="Stale" />
            </div>
          </div>
        </template>
      </template>
    </div>

    <!-- ---------------- panel items ---------------- -->
    <div class="sub-section">
      <div class="sub-title">Panel Rows</div>
      <div class="help-text mb-2">
        Shown when a marker is clicked. Subjects and keys here take
        <code>{thing}</code> too.
      </div>

      <div v-if="binding.items.length === 0" class="empty-items">
        No rows — clicking a marker shows its name only.
      </div>

      <MarkerItemEditor
        v-for="(item, index) in binding.items"
        :key="item.id"
        :item="item"
        per-thing
        @remove="removeItem(index)"
        @update:item="updateItem(index, $event)"
      />

      <div class="add-row">
        <button
          v-for="t in ITEM_TYPES"
          :key="t.type"
          class="btn-add-small"
          :disabled="atItemLimit"
          @click="addItem(t.type)"
        >
          + {{ t.label }}
        </button>
      </div>
      <div v-if="atItemLimit" class="help-text warn">
        {{ FLOORPLAN_LIMITS.MAX_ITEMS_PER_BINDING }} rows is the maximum.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Shapes } from '@lucide/vue'
import { computed } from 'vue'
import MarkerItemEditor from './MarkerItemEditor.vue'
import StatusMappingEditor from './StatusMappingEditor.vue'
import {
  createDefaultItem,
  FLOORPLAN_LIMITS,
  type FloorPlanStateMode,
  type FloorPlanTypeBinding,
  type MapItemType,
  type MapMarkerItem,
  type StatusMapping,
} from '@/types/dashboard'
import {
  bindingKvKeyForThing,
  bindingKvPattern,
  bindingSubjectForThing,
  bindingSubscriptionSubject,
  type FloorPlanContext,
} from '@/utils/floorplanBindings'
import { hasUnresolved } from '@/utils/subjectResolver'
import type { ThingType } from '@/types/pocketbase'

/**
 * One per-Thing-Type binding. The binding is mutated in place: it lives in the
 * form state the config panel already holds by reference.
 */

const props = defineProps<{
  binding: FloorPlanTypeBinding
  thingTypes: ThingType[]
  /** Resolution context, so the preview shows real subjects rather than tokens. */
  ctx: FloorPlanContext
  /** How many placed things of this type are on the plan. Null while unknown. */
  matchCount?: number | null
  /** Another binding already names this type. */
  duplicate?: boolean
}>()

defineEmits<{ remove: [] }>()

const ITEM_TYPES: Array<{ type: MapItemType; label: string }> = [
  { type: 'text', label: 'Text' },
  { type: 'kv', label: 'KV' },
  { type: 'gauge', label: 'Gauge' },
  { type: 'switch', label: 'Switch' },
  { type: 'slider', label: 'Slider' },
  { type: 'publish', label: 'Publish' },
]

const thingType = computed(() => props.thingTypes.find(t => t.id === props.binding.thingTypeId))
const typeName = computed(() => thingType.value?.name || '')

const atItemLimit = computed(
  () => props.binding.items.length >= FLOORPLAN_LIMITS.MAX_ITEMS_PER_BINDING
)

// --- state source -----------------------------------------------------------

/**
 * `none` is the absence of `stateSource`, not a stored mode, so a binding never
 * carries a half-filled source that nothing reads.
 */
const stateMode = computed<FloorPlanStateMode | 'none'>({
  get: () => props.binding.stateSource?.mode ?? 'none',
  set: (mode) => {
    if (mode === 'none') {
      delete props.binding.stateSource
      return
    }
    props.binding.stateSource = { ...(props.binding.stateSource || {}), mode }
  },
})

function sourceField<K extends 'subjectSuffix' | 'kvBucket' | 'kvKeyTemplate' | 'jsonPath'>(key: K) {
  return computed<string>({
    get: () => props.binding.stateSource?.[key] || '',
    set: (v) => {
      if (!props.binding.stateSource) return
      props.binding.stateSource[key] = v
    },
  })
}

const subjectSuffix = sourceField('subjectSuffix')
const kvBucket = sourceField('kvBucket')
const kvKeyTemplate = sourceField('kvKeyTemplate')
const jsonPath = sourceField('jsonPath')

// --- state presentation -----------------------------------------------------

const mappings = computed<StatusMapping[]>(() => {
  if (!props.binding.mappings) props.binding.mappings = []
  return props.binding.mappings
})

function bindingField<K extends 'defaultColor' | 'defaultLabel' | 'staleLabel'>(
  key: K,
  fallback: string,
) {
  return computed<string>({
    get: () => props.binding[key] ?? fallback,
    set: (v) => { props.binding[key] = v },
  })
}

const defaultColor = bindingField('defaultColor', 'var(--color-info)')
const defaultLabel = bindingField('defaultLabel', '')
const staleLabel = bindingField('staleLabel', '')

const showStale = computed<boolean>({
  get: () => props.binding.showStale ?? true,
  set: (v) => { props.binding.showStale = v },
})

const stalenessThreshold = computed<number>({
  get: () => props.binding.stalenessThreshold ?? 60000,
  set: (v) => { props.binding.stalenessThreshold = v },
})

// --- preview ----------------------------------------------------------------

/** A stand-in Thing for the preview's example line. */
const EXAMPLE_THING = { id: 'example', code: 'THING-01' }

/**
 * What this binding will actually watch, through the same helpers the widget
 * uses. A wrong binding otherwise shows up only as markers that never change.
 */
const preview = computed(() => {
  const src = props.binding.stateSource
  if (!src) return null
  const b = props.binding
  const type = thingType.value

  let pattern: string | null
  let example: string | null
  if (src.mode === 'subject') {
    pattern = bindingSubscriptionSubject(b, type, props.ctx)
    example = bindingSubjectForThing(b, type, EXAMPLE_THING, props.ctx)
  } else {
    const key = bindingKvPattern(b, type, props.ctx)
    const exampleKey = bindingKvKeyForThing(b, type, EXAMPLE_THING, props.ctx)
    pattern = key && `${src.kvBucket} / ${key}`
    example = exampleKey && `${src.kvBucket} / ${exampleKey}`
  }
  if (!pattern) return null

  let unresolved = ''
  if (src.mode === 'kv' && !src.kvKeyTemplate?.includes('{thing}')) {
    unresolved = 'This key has no {thing}, so every marker reads the same value.'
  } else if (hasUnresolved(pattern)) {
    unresolved = 'Some variables could not be filled in — check the organization and type have codes.'
  }
  return { pattern, example, unresolved }
})

// --- items ------------------------------------------------------------------

function addItem(type: MapItemType) {
  if (atItemLimit.value) return
  props.binding.items.push(createDefaultItem(type))
}

function removeItem(index: number) {
  props.binding.items.splice(index, 1)
}

function updateItem(index: number, item: MapMarkerItem) {
  props.binding.items.splice(index, 1, item)
}
</script>

<style scoped>
.binding-editor {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.binding-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.binding-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}

.binding-count {
  font-size: 11px;
  font-weight: 500;
  color: var(--muted);
}

.sub-section {
  border-top: 1px solid var(--border);
  padding-top: 12px;
}

.sub-title {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--muted);
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.form-group > label {
  font-size: 11px;
  font-weight: 500;
  color: var(--muted);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 12px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.input-group label {
  font-size: 11px;
  font-weight: 500;
  color: var(--muted);
}

.preview {
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 8px;
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.preview-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.preview-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted);
  flex: 0 0 auto;
}

.preview-value {
  font-size: 11px;
  color: var(--text);
  overflow-wrap: anywhere;
}

.empty-items {
  font-size: 12px;
  color: var(--muted);
  padding: 8px 0;
}

.add-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.btn-add-small {
  flex: 1 1 auto;
  padding: 6px 10px;
  background: transparent;
  border: 1px dashed var(--border);
  color: var(--color-primary);
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
}

.btn-add-small:hover:not(:disabled) {
  background: var(--color-primary-hover);
  color: white;
  border-style: solid;
}

.btn-add-small:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-icon {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 4px;
}

.btn-icon.danger:hover { color: var(--color-error); }

.help-text.warn { color: var(--color-warning); }

.mb-2 { margin-bottom: 8px; }
</style>
