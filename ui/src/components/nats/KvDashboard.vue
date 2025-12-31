<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsKv, type KvEntry } from '@/composables/useNatsKv'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList, { type Column } from '@/components/ui/ResponsiveList.vue'
import { formatDate } from '@/utils/format'

// Adapter type to satisfy ResponsiveList requirement
interface KvEntryWithId extends KvEntry {
  id: string
}

const props = defineProps<{
  bucket: string
  contextCode: string
}>()

const { entries, loading, exists, error, init, createBucket, put, del, getHistory } = useNatsKv(props.bucket, `${props.contextCode}.>`)
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

// --- Computeds ---

// 1. Full Sorted List (All data)
const sortedEntries = computed<KvEntryWithId[]>(() => {
  return Array.from(entries.value.values())
    .sort((a, b) => a.key.localeCompare(b.key))
    .map(entry => ({
      ...entry,
      id: entry.key
    }))
})

// 2. Filtered List (Search)
const filteredEntries = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return sortedEntries.value

  return sortedEntries.value.filter(item => {
    // Search Key
    if (item.key.toLowerCase().includes(q)) return true
    
    // Search Value (handle objects/arrays by stringifying)
    const valStr = typeof item.value === 'object' 
      ? JSON.stringify(item.value) 
      : String(item.value)
    
    if (valStr.toLowerCase().includes(q)) return true
    
    return false
  })
})

// 3. Pagination Logic (Based on Filtered List)
const totalItems = computed(() => filteredEntries.value.length)
const totalPages = computed(() => Math.ceil(totalItems.value / itemsPerPage) || 1)

const paginatedEntries = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  const end = start + itemsPerPage
  return filteredEntries.value.slice(start, end)
})

// --- Columns ---

const keyColumns: Column<KvEntryWithId>[] = [
  { key: 'key', label: 'Key', mobileLabel: 'Key' },
  { key: 'value', label: 'Value', mobileLabel: 'Value' },
  { key: 'revision', label: 'Rev', mobileLabel: 'Rev', class: 'text-right w-20' }
]

const historyColumns: Column<KvEntryWithId>[] = [
  { key: 'revision', label: 'Rev', mobileLabel: 'Rev', class: 'w-16' },
  { key: 'operation', label: 'Op', mobileLabel: 'Op', class: 'w-20' },
  { key: 'value', label: 'Value', mobileLabel: 'Value' },
  { key: 'created', label: 'Time', mobileLabel: 'Time', class: 'text-right', format: (v) => formatDate(v, 'PP p') }
]

// --- Actions ---

onMounted(() => init())

async function handleCreate() {
  await createBucket(`Digital Twin for ${props.contextCode}`)
}

// Pagination Controls
function nextPage() {
  if (currentPage.value < totalPages.value) currentPage.value++
}
function prevPage() {
  if (currentPage.value > 1) currentPage.value--
}

function resetForm() {
  showForm.value = false
  isEditing.value = false
  inputKey.value = ''
  inputValue.value = ''
  inputType.value = 'string'
  historyEntries.value = []
}

function openAddForm() {
  resetForm()
  showForm.value = true
}

async function selectEntry(entry: KvEntryWithId) {
  isEditing.value = true
  showForm.value = true
  inputKey.value = entry.key
  
  const val = entry.value
  if (typeof val === 'number') inputType.value = 'number'
  else if (typeof val === 'boolean') {
    inputType.value = 'boolean'
    inputBool.value = val
  }
  else if (typeof val === 'object') {
    inputType.value = 'json'
    inputValue.value = JSON.stringify(val, null, 2)
  }
  else {
    inputType.value = 'string'
    inputValue.value = String(val)
  }

  historyLoading.value = true
  const rawHistory = await getHistory(entry.key)
  historyEntries.value = rawHistory.map(h => ({ ...h, id: h.revision.toString() }))
  historyLoading.value = false
}

function restoreHistory(entry: KvEntryWithId) {
  if (entry.operation === 'DEL') return
  
  const val = entry.value
  if (typeof val === 'number') {
    inputType.value = 'number'
    inputValue.value = String(val)
  } else if (typeof val === 'boolean') {
    inputType.value = 'boolean'
    inputBool.value = val
  } else if (typeof val === 'object') {
    inputType.value = 'json'
    inputValue.value = JSON.stringify(val, null, 2)
  } else {
    inputType.value = 'string'
    inputValue.value = String(val)
  }
  toast.info(`Loaded value from Rev ${entry.revision}`)
}

async function handleSave() {
  if (!inputKey.value) {
    toast.error('Key name required')
    return
  }

  let fullKey = inputKey.value
  if (!isEditing.value) {
    fullKey = inputKey.value.startsWith(props.contextCode + '.') 
      ? inputKey.value 
      : `${props.contextCode}.${inputKey.value}`
  }

  let finalValue: any = inputValue.value

  try {
    if (inputType.value === 'number') {
      finalValue = Number(inputValue.value)
      if (isNaN(finalValue)) throw new Error('Invalid number')
    } else if (inputType.value === 'boolean') {
      finalValue = inputBool.value
    } else if (inputType.value === 'json') {
      finalValue = JSON.parse(inputValue.value)
    }
    
    await put(fullKey, finalValue)
    
    if (isEditing.value) {
      historyLoading.value = true
      const rawHistory = await getHistory(fullKey)
      historyEntries.value = rawHistory.map(h => ({ ...h, id: h.revision.toString() }))
      historyLoading.value = false
      toast.success('Updated')
    } else {
      resetForm()
    }
  } catch (e: any) {
    toast.error(`Invalid value: ${e.message}`)
  }
}

function getValueType(val: any): string {
  if (val === null) return 'null'
  if (Array.isArray(val)) return 'array'
  return typeof val
}

// Reset pagination when search changes
watch(searchQuery, () => {
  currentPage.value = 1
})
</script>

<template>
  <BaseCard :title="`Digital Twin: ${bucket}`" class="h-full">
    
    <!-- Loading -->
    <div v-if="loading" class="flex justify-center p-8">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Create Bucket State -->
    <div v-else-if="!exists && !error" class="text-center py-8">
      <div class="text-5xl mb-3">🧊</div>
      <h3 class="font-bold text-lg">No Digital Twin Store</h3>
      <p class="text-sm opacity-70 mb-4">
        A NATS KV bucket named <code class="font-mono bg-base-200 px-1 rounded">{{ bucket }}</code> does not exist.
      </p>
      <button @click="handleCreate" class="btn btn-primary btn-sm">
        Initialize Bucket
      </button>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="alert alert-error">
      <span>{{ error }}</span>
      <button class="btn btn-xs btn-ghost" @click="init">Retry</button>
    </div>

    <!-- Active Dashboard -->
    <div v-else class="space-y-4">
      
      <!-- Toolbar -->
      <div class="flex flex-col gap-3">
        <div class="flex justify-between items-center">
          <div class="text-xs opacity-60 font-mono">{{ sortedEntries.length }} Keys (Filter: {{ contextCode }}.&gt;)</div>
          <button 
            v-if="!showForm"
            @click="openAddForm" 
            class="btn btn-xs btn-primary btn-outline"
          >
            + Add Key
          </button>
          <button 
            v-else
            @click="resetForm" 
            class="btn btn-xs btn-ghost"
          >
            Cancel
          </button>
        </div>

        <!-- Search Input -->
        <div v-if="!showForm" class="form-control">
          <input 
            v-model="searchQuery"
            type="text" 
            placeholder="Search keys or values..." 
            class="input input-sm input-bordered w-full"
          />
        </div>
      </div>

      <!-- Editor Form (Add / Edit) -->
      <div v-if="showForm" class="bg-base-200 p-4 rounded-lg space-y-4 border border-base-300 animate-fade-in">
        <h4 class="font-bold text-xs uppercase opacity-50 tracking-wider">
          {{ isEditing ? 'Edit Key' : 'New Key' }}
        </h4>

        <div class="flex flex-col sm:flex-row gap-2">
          <!-- Key Input -->
          <div class="join w-full sm:w-auto flex-1">
            <span v-if="!isEditing" class="join-item btn btn-sm btn-static bg-base-300 font-mono text-xs text-base-content/70">
              {{ contextCode }}.
            </span>
            <input 
              v-model="inputKey" 
              type="text" 
              placeholder="key.name" 
              class="input input-sm input-bordered join-item flex-1 font-mono min-w-0"
              :disabled="isEditing"
            />
          </div>
          
          <!-- Type Selector -->
          <select v-model="inputType" class="select select-sm select-bordered w-full sm:w-auto">
            <option value="string">String</option>
            <option value="number">Number</option>
            <option value="boolean">Boolean</option>
            <option value="json">JSON</option>
          </select>
        </div>

        <!-- Value Input -->
        <div>
          <input v-if="inputType === 'string'" v-model="inputValue" type="text" placeholder="Value..." class="input input-sm input-bordered w-full font-mono" />
          <input v-if="inputType === 'number'" v-model="inputValue" type="number" placeholder="0.00" class="input input-sm input-bordered w-full font-mono" />
          <div v-if="inputType === 'boolean'" class="form-control">
            <label class="label cursor-pointer justify-start gap-3">
              <input type="checkbox" v-model="inputBool" class="toggle toggle-sm toggle-success" />
              <span class="font-mono">{{ inputBool }}</span>
            </label>
          </div>
          <textarea v-if="inputType === 'json'" v-model="inputValue" class="textarea textarea-bordered textarea-sm w-full font-mono leading-tight" rows="4" placeholder='{"foo": "bar"}'></textarea>
        </div>

        <div class="flex justify-end gap-2">
          <button v-if="isEditing" @click="del(inputKey); resetForm()" class="btn btn-sm btn-ghost text-error">Delete Key</button>
          <button @click="handleSave" class="btn btn-sm btn-primary min-w-[100px]">
            {{ isEditing ? 'Update Value' : 'Save Key' }}
          </button>
        </div>

        <!-- History Table (Responsive) -->
        <div v-if="isEditing" class="pt-2 border-t border-base-content/10">
          <div class="font-bold text-xs uppercase opacity-50 tracking-wider mb-2">Revision History</div>
          <div v-if="historyLoading" class="text-center py-4"><span class="loading loading-dots loading-xs"></span></div>
          <div v-else class="border border-base-300 rounded bg-base-100 max-h-[300px] overflow-y-auto">
            <ResponsiveList :items="historyEntries" :columns="historyColumns" :clickable="false">
              <template #cell-revision="{ item }"><span class="font-mono text-xs opacity-70">{{ item.revision }}</span></template>
              <template #card-revision="{ item }"><span class="font-mono text-xs opacity-70">Rev {{ item.revision }}</span></template>
              <template #cell-operation="{ item }">
                <span class="badge badge-xs" :class="item.operation === 'DEL' ? 'badge-error' : 'badge-success'">{{ item.operation }}</span>
              </template>
              <template #card-operation="{ item }">
                <span class="badge badge-xs" :class="item.operation === 'DEL' ? 'badge-error' : 'badge-success'">{{ item.operation }}</span>
              </template>
              <template #cell-value="{ item }">
                <span class="truncate font-mono text-xs opacity-70 block max-w-[200px]">{{ item.operation === 'DEL' ? '-' : (typeof item.value === 'object' ? JSON.stringify(item.value) : item.value) }}</span>
              </template>
              <template #card-value="{ item }">
                <div class="font-mono text-xs bg-base-200 p-1 rounded mt-1 break-all">{{ item.operation === 'DEL' ? 'DELETED' : (typeof item.value === 'object' ? JSON.stringify(item.value) : item.value) }}</div>
              </template>
              <template #actions="{ item }">
                <button v-if="item.operation !== 'DEL'" @click="restoreHistory(item)" class="btn btn-ghost btn-xs">Load</button>
              </template>
            </ResponsiveList>
          </div>
        </div>
      </div>

      <!-- Main Keys List (Responsive) -->
      <div v-if="!showForm" class="border border-base-200 rounded-lg">
        <ResponsiveList :items="paginatedEntries" :columns="keyColumns" @row-click="selectEntry">
          <template #cell-key="{ item }"><span class="font-mono text-primary font-medium text-xs sm:text-sm">{{ item.key }}</span></template>
          <template #card-key="{ item }"><span class="font-mono text-primary font-medium text-sm">{{ item.key }}</span></template>
          <template #cell-value="{ item }">
            <div class="flex items-center gap-2 max-w-[300px]">
              <span class="badge badge-xs badge-outline opacity-50 shrink-0">{{ getValueType(item.value) }}</span>
              <span class="truncate font-mono opacity-80 text-xs">{{ typeof item.value === 'object' ? JSON.stringify(item.value) : item.value }}</span>
            </div>
          </template>
          <template #card-value="{ item }">
            <div class="mt-1">
              <span class="badge badge-xs badge-outline opacity-50 mb-1 mr-2">{{ getValueType(item.value) }}</span>
              <div class="font-mono text-xs bg-base-200 p-2 rounded break-all">{{ typeof item.value === 'object' ? JSON.stringify(item.value) : item.value }}</div>
            </div>
          </template>
          <template #cell-revision="{ item }"><span class="font-mono opacity-50 text-xs">{{ item.revision }}</span></template>
          <template #card-revision="{ item }"><span class="font-mono opacity-50 text-xs">Rev: {{ item.revision }}</span></template>
          <template #actions="{ item }">
            <button @click.stop="del(item.key)" class="btn btn-ghost btn-xs text-error hover:bg-error/10" title="Delete Key">✕</button>
          </template>
          <template #empty>
            <div class="text-center py-8 opacity-50 italic">
              {{ searchQuery ? 'No matches found' : `No keys found for ${contextCode}` }}
            </div>
          </template>
        </ResponsiveList>
      </div>

      <!-- Pagination Controls (Client-Side) -->
      <div v-if="!showForm && totalItems > 0" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-2 border-t border-base-200">
        <span class="text-xs text-base-content/70">
          Showing {{ ((currentPage - 1) * itemsPerPage) + 1 }} - {{ Math.min(currentPage * itemsPerPage, totalItems) }} of {{ totalItems }}
        </span>
        <div class="join">
          <button class="join-item btn btn-xs" :disabled="currentPage === 1" @click="prevPage()">«</button>
          <button class="join-item btn btn-xs cursor-default">Page {{ currentPage }}</button>
          <button class="join-item btn btn-xs" :disabled="currentPage === totalPages" @click="nextPage()">»</button>
        </div>
      </div>

    </div>
  </BaseCard>
</template>

<style scoped>
.animate-fade-in {
  animation: fadeIn 0.2s ease-out;
}
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
