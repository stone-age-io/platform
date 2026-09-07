<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import type { ThingTypeCapability, ThingTypeOperation } from '@/types/pocketbase'

// An operation is a verb a Thing Type declares: a name, what kind of exchange it
// is, and the subject suffix it lands on. It used to carry a relation to a
// message_schemas record describing its payload; that collection was dropped
// (nothing validated against it, and only the Publisher widget's payload form
// ever read it), so this form is now one card rather than two.
//
// Rendered both as a route and embedded in the Thing Type form's quick-add
// modal, which is why `embedded` suppresses the heading and swaps navigation for
// events.

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: ThingTypeOperation): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const id = route.params.id as string | undefined
const isEdit = computed(() => !!id && !props.embedded)
const loading = ref(false)

const form = ref({
  name: '',
  capability: 'publish' as ThingTypeCapability,
  subject_suffix: '',
  description: '',
})

const availableCapabilities: ThingTypeCapability[] = ['publish', 'subscribe', 'request', 'reply']

async function loadData() {
  if (!id || props.embedded) return
  loading.value = true
  try {
    const rec = await pb.collection('thing_type_operations').getOne<ThingTypeOperation>(id)
    form.value = {
      name: rec.name,
      capability: rec.capability,
      subject_suffix: rec.subject_suffix,
      description: rec.description || '',
    }
  } catch {
    toast.error('Failed to load operation')
    router.push('/things/operations')
  } finally {
    loading.value = false
  }
}

async function submit() {
  loading.value = true
  try {
    let record: ThingTypeOperation
    if (isEdit.value) {
      record = await pb.collection('thing_type_operations').update<ThingTypeOperation>(id!, form.value)
      toast.success('Updated')
    } else {
      record = await pb.collection('thing_type_operations').create<ThingTypeOperation>({
        ...form.value,
        organization: authStore.currentOrgId,
      })
      toast.success('Created')
    }

    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/things/operations')
    }
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

function handleCancel() {
  if (props.embedded) {
    emit('cancel')
  } else {
    router.back()
  }
}

onMounted(async () => {
  if (isEdit.value) await loadData()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things/operations">Operations</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">{{ isEdit ? 'Edit' : 'Create' }} Operation</h1>
    </div>

    <form @submit.prevent="submit" class="space-y-6">
      <BaseCard title="Operation">
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">Name *</label>
            <input
              v-model="form.name"
              type="text"
              class="input input-bordered font-mono"
              required
              pattern="[a-z0-9_]+"
              placeholder="e.g. motion"
            />
            <label class="label"><span class="label-text-alt">Lowercase snake_case.</span></label>
          </div>

          <div class="form-control">
            <label class="label">Capability *</label>
            <select v-model="form.capability" class="select select-bordered" required>
              <option v-for="cap in availableCapabilities" :key="cap" :value="cap">{{ cap }}</option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                What kind of exchange this is. It is also what tells a request apart
                from a reply on the same subject suffix.
              </span>
            </label>
          </div>

          <div class="form-control">
            <label class="label">Subject Suffix *</label>
            <input
              v-model="form.subject_suffix"
              type="text"
              class="input input-bordered font-mono"
              required
              placeholder="e.g. motion or cmd.ptz"
            />
            <label class="label">
              <span class="label-text-alt">Appended to the Thing Type's subject prefix, separated by a dot.</span>
            </label>
          </div>

          <div class="form-control">
            <label class="label">Description</label>
            <textarea v-model="form.description" class="textarea textarea-bordered" rows="2"></textarea>
          </div>
        </div>
      </BaseCard>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-ghost" @click="handleCancel">Cancel</button>
        <button type="submit" class="btn btn-primary" :disabled="loading">
          <span v-if="loading" class="loading loading-spinner"></span>
          Save
        </button>
      </div>
    </form>
  </div>
</template>
