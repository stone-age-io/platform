<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsKv, type KvEntry } from '@/composables/useNatsKv'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'

interface KvEntryWithId extends KvEntry {
  id: string
}

const props = defineProps<{
  bucket?: string
  baseKey: string // Required: e.g. "thing.S01"
}>()

const { entries, loading, exists, error, init, createBucket, put, del, getHistory } = useNatsKv(props.bucket || 'twin', props.baseKey)
const toast = useToast()

// UI State
const showForm = ref(false)
const isEditing = ref(false)
const historyLoading = ref(false)
const historyEntries = ref<KvEntryWithId[]>([])

// Search & Pagination
const searchQuery = ref('')
const currentPage = ref(1)
const itemsPerPage = 10

// Form Inputs
const inputKey = ref('')
const inputType = ref<'string' | 'number' | 'boolean' | 'json'>('string')
const inputValue = ref('')
const inputBool = ref(true)

// Helper: Strip the baseKey prefix for display (e.g. thing.S01.temp -> temp)
function displayKey(fullKey: string) {
  if (!props.baseKey) return fullKey
  return fullKey.replace(`${props.baseKey}.`, '')
}

// Computeds
const sortedEntries = computed<KvEntryWithId[]>(() => {
  return Array.from(entries.value.values())
    .sort((a, b) => a.key.localeCompare(b.key))
    .map(entry => ({ ...entry, id: entry.key }))
})

const filteredEntries = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return sortedEntries.value
  return sortedEntries.value.filter(item => {
    return item.key.toLowerCase().includes(q) || String(item.value).toLowerCase().includes(q)
  })
})

const totalItems = computed(() => filteredEntries.value.length)
const totalPages = computed(() => Math.ceil(totalItems.value / itemsPerPage) || 1)
const paginatedEntries = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return filteredEntries.value.slice(start, start + itemsPerPage)
})

const keyColumns: Column<KvEntryWithId>[] = [
  { key: 'key', label: 'Property', mobileLabel: 'Prop' },
  { key: 'value', label: 'Value', mobileLabel: 'Value' },
  { key: 'revision', label: 'Rev', mobileLabel: 'Rev', class: 'text-right w-20' }
]

// Actions
onMounted(() => init())

async function handleSave() {
  if (!inputKey.value) return toast.error('Property name required')
  
  let finalValue: any = inputValue.value
  try {
    if (inputType.value === 'number') finalValue = Number(inputValue.value)
    else if (inputType.value === 'boolean') finalValue = inputBool.value
    else if (inputType.value === 'json') finalValue = JSON.parse(inputValue.value)
    
    await put(inputKey.value, finalValue)
    toast.success(isEditing.value ? 'Updated' : 'Created')
    resetForm()
  } catch (e: any) {
    toast.error(`Invalid value: ${e.message}`)
  }
}

async function selectEntry(entry: KvEntryWithId) {
  isEditing.value = true
  showForm.value = true
  inputKey.value = displayKey(entry.key)
  
  const val = entry.value
  if (typeof val === 'number') inputType.value = 'number'
  else if (typeof val === 'boolean') { inputType.value = 'boolean'; inputBool.value = val }
  else if (typeof val === 'object') { inputType.value = 'json'; inputValue.value = JSON.stringify(val, null, 2) }
  else { inputType.value = 'string'; inputValue.value = String(val) }

  historyLoading.value = true
  const rawHistory = await getHistory(entry.key)
  historyEntries.value = rawHistory.map(h => ({ ...h, id: h.revision.toString() }))
  historyLoading.value = false
}

function resetForm() {
  showForm.value = false; isEditing.value = false; inputKey.value = ''; inputValue.value = ''
  inputType.value = 'string'; historyEntries.value = []
}

watch(searchQuery, () => { currentPage.value = 1 })
</script>

<template>
  <BaseCard :title="`Digital Twin: ${baseKey}`" class="h-full">
    <div v-if="loading" class="flex justify-center p-8"><span class="loading loading-spinner text-primary"></span></div>

    <div v-else-if="!exists && !error" class="text-center py-8">
      <div class="text-5xl mb-3">🧊</div>
      <h3 class="font-bold text-lg">Bucket Not Initialized</h3>
      <p class="text-sm opacity-70 mb-4">The Organization Twin bucket ({{ bucket || 'twin' }}) does not exist yet.</p>
      <button @click="createBucket()" class="btn btn-primary btn-sm">Initialize Bucket</button>
    </div>

    <div v-else class="space-y-4">
      <div class="flex justify-between items-center">
        <div class="text-xs opacity-60 font-mono">{{ totalItems }} properties for {{ baseKey }}</div>
        <button v-if="!showForm" @click="showForm = true" class="btn btn-xs btn-primary btn-outline">+ Add Property</button>
        <button v-else @click="resetForm" class="btn btn-xs btn-ghost">Cancel</button>
      </div>

      <!-- Editor -->
      <div v-if="showForm" class="bg-base-200 p-4 rounded-lg space-y-4 border border-base-300">
        <div class="flex flex-col sm:flex-row gap-2">
          <div class="join flex-1">
            <span class="join-item btn btn-sm btn-disabled font-mono text-xs">{{ baseKey }}.</span>
            <input v-model="inputKey" type="text" placeholder="property_name" class="input input-sm input-bordered join-item flex-1 font-mono" :disabled="isEditing" />
          </div>
          <select v-model="inputType" class="select select-sm select-bordered">
            <option value="string">String</option><option value="number">Number</option><option value="boolean">Boolean</option><option value="json">JSON</option>
          </select>
        </div>
        <div v-if="inputType === 'boolean'"><input type="checkbox" v-model="inputBool" class="toggle toggle-success" /></div>
        <textarea v-else-if="inputType === 'json'" v-model="inputValue" class="textarea textarea-bordered textarea-sm w-full font-mono" rows="4"></textarea>
        <input v-else v-model="inputValue" type="text" class="input input-sm input-bordered w-full font-mono" />
        
        <div class="flex justify-end gap-2">
          <button v-if="isEditing" @click="del(`${baseKey}.${inputKey}`); resetForm()" class="btn btn-sm btn-ghost text-error">Delete</button>
          <button @click="handleSave" class="btn btn-sm btn-primary">Save Property</button>
        </div>
      </div>

      <!-- List -->
      <div v-if="!showForm" class="border border-base-200 rounded-lg">
        <ResponsiveList :items="paginatedEntries" :columns="keyColumns" @row-click="selectEntry">
          <template #cell-key="{ item }">
            <span class="font-mono text-primary font-medium">{{ displayKey(item.key) }}</span>
          </template>
          <template #cell-value="{ item }">
            <code class="text-xs opacity-80">{{ typeof item.value === 'object' ? JSON.stringify(item.value) : item.value }}</code>
          </template>
        </ResponsiveList>
      </div>

      <!-- Pagination -->
      <div v-if="!showForm && totalItems > 10" class="flex justify-center border-t border-base-200 pt-2">
        <div class="join">
          <button class="join-item btn btn-xs" :disabled="currentPage === 1" @click="currentPage--">«</button>
          <button class="join-item btn btn-xs">Page {{ currentPage }}</button>
          <button class="join-item btn btn-xs" :disabled="currentPage === totalPages" @click="currentPage++">»</button>
        </div>
      </div>
    </div>
  </BaseCard>
</template>
