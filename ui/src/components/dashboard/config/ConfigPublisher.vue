<template>
  <div class="config-publisher">
    <div class="form-group">
      <label>Thing</label>
      <select v-model="form.publisherThingId" class="form-input" @change="onThingChange">
        <option value="">— None (manual subject) —</option>
        <option v-for="t in things" :key="t.id" :value="t.id">
          {{ t.name || t.code || t.id }}<span v-if="t.code"> ({{ t.code }})</span>
        </option>
      </select>
      <div class="help-text">
        When selected, pick an operation below to auto-resolve the subject and render a schema-driven payload form.
      </div>
    </div>

    <div class="form-group">
      <label>Operation</label>
      <select
        v-model="form.publisherThingTypeOperationId"
        class="form-input"
        :disabled="!form.publisherThingId || operations.length === 0"
      >
        <option value="">— None —</option>
        <option v-for="op in operations" :key="op.id" :value="op.id">
          {{ op.name }} ({{ op.capability }}) · {{ op.subject_suffix }}
        </option>
      </select>
      <div v-if="form.publisherThingId && operations.length === 0" class="help-text">
        This Thing's type has no operations linked.
      </div>
    </div>

    <div class="form-group">
      <label>Default Subject</label>
      <input
        v-model="form.publisherDefaultSubject"
        type="text"
        class="form-input"
        :disabled="!!form.publisherThingTypeOperationId"
        placeholder="service.action"
      />
      <div class="help-text">
        Initial subject when widget loads. Ignored when a Thing + Operation are bound.
      </div>
    </div>

    <div class="form-group">
      <label>Default Payload</label>
      <textarea
        v-model="form.publisherDefaultPayload"
        class="form-textarea"
        rows="6"
        :disabled="!!form.publisherThingTypeOperationId"
        placeholder='{"key": "value"}'
      />
      <div class="help-text">
        Initial payload. Ignored when the operation has a linked message schema.
      </div>
    </div>

    <div class="form-group">
      <label>Request Timeout (ms)</label>
      <input
        v-model.number="form.publisherTimeout"
        type="number"
        class="form-input"
        min="100"
        step="100"
        placeholder="2000"
      />
      <div class="help-text">
        Max time to wait for a reply in Request mode.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import type { WidgetFormState } from '@/types/config'
import type { Thing, ThingType, ThingTypeOperation } from '@/types/pocketbase'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const authStore = useAuthStore()

const things = ref<Thing[]>([])
const operations = ref<ThingTypeOperation[]>([])

async function loadThings() {
  const orgId = authStore.currentOrgId
  if (!orgId) { things.value = []; return }
  things.value = await pb.collection('things').getFullList<Thing>({
    filter: `organization = "${orgId}"`,
    sort: 'name',
  })
}

async function loadOperationsForThing(thingId: string) {
  operations.value = []
  if (!thingId) return
  try {
    const thing = await pb.collection('things').getOne<Thing>(thingId)
    if (!thing.type) return
    const tt = await pb.collection('thing_types').getOne<ThingType>(thing.type)
    const opIds = tt.operations || []
    if (opIds.length === 0) return
    const filter = opIds.map(id => `id = "${id}"`).join(' || ')
    operations.value = await pb.collection('thing_type_operations').getFullList<ThingTypeOperation>({
      filter,
      sort: 'name',
    })
  } catch {
    operations.value = []
  }
}

function onThingChange() {
  props.form.publisherThingTypeOperationId = ''
  loadOperationsForThing(props.form.publisherThingId)
}

watch(() => props.form.publisherThingId, (id) => {
  if (id) loadOperationsForThing(id)
  else operations.value = []
}, { immediate: false })

onMounted(async () => {
  await loadThings()
  if (props.form.publisherThingId) {
    await loadOperationsForThing(props.form.publisherThingId)
  }
})
</script>
