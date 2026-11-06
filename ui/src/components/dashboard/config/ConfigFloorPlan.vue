<template>
  <div class="config-floorplan">
    <!-- ---------------- the plan ---------------- -->
    <div class="config-section">
      <div class="section-title">Floor Plan</div>

      <div class="form-group">
        <label>Location</label>
        <select v-model="form.floorplanLocationId" class="form-input">
          <option value="">Select a location…</option>
          <option
            v-for="loc in locations"
            :key="loc.id"
            :value="loc.id"
            :disabled="!loc.floorplan"
          >
            {{ loc.name }}<template v-if="loc.code"> ({{ loc.code }})</template>
            <template v-if="!loc.floorplan"> — no plan uploaded</template>
          </option>
        </select>
        <div v-if="errors.floorplanLocationId" class="error-text">
          {{ errors.floorplanLocationId }}
        </div>
        <div class="help-text">
          The image and the marker positions both come from the location.
          Positions are set on the location's own page, so there is nothing to
          place here.
        </div>
      </div>

      <!-- Dashboards are visible to every role, `dashboard` included -- the
           appliance login the console otherwise does not let browse inventory
           at all. That is accepted (plans/floorplan-widget.md, Authorization),
           but an author should be told rather than discover it, because a plan
           on a lobby screen is a decision about the building. -->
      <div v-if="selectedLocation" class="notice">
        <Info :size="14" aria-hidden="true" />
        <span>
          Anyone who can open this dashboard will see
          <strong>{{ selectedLocation.name }}</strong>'s floor plan and the
          things on it — including screens signed in with the dashboard role.
        </span>
      </div>

      <div class="form-group">
        <label class="checkbox-label">
          <input type="checkbox" v-model="form.floorplanShowUnbound" />
          <span>Show things with no binding</span>
        </label>
        <div class="help-text">
          Off, the plan draws only the types bound below. On, everything placed
          appears and unbound things simply have no state and no panel rows.
        </div>
      </div>

      <div v-if="loadError" class="error-text">{{ loadError }}</div>
    </div>

    <!-- ---------------- bindings ---------------- -->
    <div class="config-section">
      <div class="section-title">Bindings</div>
      <div class="help-text mb-2">
        One binding per thing type, applied to every thing of that type on the
        plan. That is why sixty doors take one binding rather than sixty.
      </div>

      <div v-if="form.floorplanBindings.length === 0" class="empty-state">
        No bindings yet — the plan will draw its markers in one colour.
      </div>

      <div class="bindings-list">
        <TypeBindingEditor
          v-for="(binding, index) in form.floorplanBindings"
          :key="binding.id"
          :binding="binding"
          :thing-types="thingTypes"
          :ctx="ctx"
          :match-count="matchCounts.get(binding.thingTypeId) ?? null"
          :duplicate="isDuplicate(index)"
          @remove="removeBinding(index)"
        />
      </div>

      <button class="btn-add" :disabled="atBindingLimit" @click="addBinding">
        + Add Binding
      </button>
      <div v-if="atBindingLimit" class="help-text warn">
        {{ FLOORPLAN_LIMITS.MAX_BINDINGS }} bindings is the maximum.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Info } from '@lucide/vue'
import { computed, onMounted, ref, watch } from 'vue'
import TypeBindingEditor from './TypeBindingEditor.vue'
import { useAuthStore } from '@/stores/auth'
import { pb } from '@/utils/pb'
import { FLOORPLAN_LIMITS, type FloorPlanTypeBinding } from '@/types/dashboard'
import { isPlaced, type FloorPlanContext } from '@/utils/floorplanBindings'
import type { WidgetFormState } from '@/types/config'
import type { Location, Thing, ThingType } from '@/types/pocketbase'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const authStore = useAuthStore()

const locations = ref<Location[]>([])
const thingTypes = ref<ThingType[]>([])
const placedThings = ref<Thing[]>([])
const loadError = ref('')

const selectedLocation = computed(
  () => locations.value.find(l => l.id === props.form.floorplanLocationId) || null
)

const ctx = computed<FloorPlanContext>(() => ({
  orgCode: authStore.currentOrg?.code || '',
  locationCode: selectedLocation.value?.code || '',
}))

/**
 * Placed things per type, shown on each binding so an author can tell a
 * binding that matches nothing from one whose devices are not publishing yet.
 */
const matchCounts = computed(() => {
  const counts = new Map<string, number>()
  for (const t of placedThings.value) {
    if (!t.type) continue
    counts.set(t.type, (counts.get(t.type) || 0) + 1)
  }
  return counts
})

const atBindingLimit = computed(
  () => props.form.floorplanBindings.length >= FLOORPLAN_LIMITS.MAX_BINDINGS
)

function isDuplicate(index: number): boolean {
  const id = props.form.floorplanBindings[index]?.thingTypeId
  if (!id) return false
  return props.form.floorplanBindings.findIndex(b => b.thingTypeId === id) !== index
}

function addBinding() {
  if (atBindingLimit.value) return
  const binding: FloorPlanTypeBinding = {
    id: `fpb_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    thingTypeId: '',
    items: [],
    mappings: [],
    showStale: true,
    stalenessThreshold: 60000,
  }
  props.form.floorplanBindings.push(binding)
}

function removeBinding(index: number) {
  props.form.floorplanBindings.splice(index, 1)
}

async function loadLists() {
  loadError.value = ''
  try {
    const [locs, types] = await Promise.all([
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
      pb.collection('thing_types').getFullList<ThingType>({ sort: 'name' }),
    ])
    locations.value = locs
    thingTypes.value = types
  } catch (err) {
    loadError.value = 'Could not load locations and thing types.'
    console.warn('Floor plan config load failed', err)
  }
}

/** Guards an out-of-order load when the location is changed twice quickly. */
let thingsSeq = 0

async function loadThings(locationId: string) {
  const seq = ++thingsSeq
  if (!locationId) {
    placedThings.value = []
    return
  }
  try {
    const list = await pb.collection('things').getFullList<Thing>({
      filter: pb.filter('location = {:id}', { id: locationId }),
    })
    if (seq !== thingsSeq) return
    placedThings.value = list.filter(isPlaced)
  } catch {
    if (seq !== thingsSeq) return
    placedThings.value = []
  }
}

onMounted(() => {
  void loadLists()
  void loadThings(props.form.floorplanLocationId)
})

watch(() => props.form.floorplanLocationId, (id) => void loadThings(id))
</script>

<style scoped>
.config-floorplan {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.config-section {
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
}

.section-title {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 12px;
  letter-spacing: 0.5px;
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

.notice {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 8px 10px;
  margin-bottom: 12px;
  background: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-radius: 6px;
  font-size: 12px;
  color: var(--text);
}

.bindings-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.empty-state {
  font-size: 12px;
  color: var(--muted);
  padding: 8px 0 12px;
}

.btn-add {
  width: 100%;
  padding: 8px;
  background: transparent;
  border: 1px dashed var(--border);
  color: var(--color-primary);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
}

.btn-add:hover:not(:disabled) {
  background: var(--color-primary-hover);
  color: white;
  border-style: solid;
}

.btn-add:disabled { opacity: 0.5; cursor: not-allowed; }

.error-text {
  font-size: 12px;
  color: var(--color-error);
}

.help-text.warn { color: var(--color-warning); }

.mb-2 { margin-bottom: 8px; }
</style>
