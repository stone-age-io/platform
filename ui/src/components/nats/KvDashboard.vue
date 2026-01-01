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
  baseKey: string 
}>()

const { entries, loading, exists, error, init, createBucket, put, del, getHistory } = useNatsKv(props.bucket || 'twin', props.baseKey)
const toast = useToast()

// UI State
const showForm = ref(false)
const isEditing = ref(false)
const historyLoading = ref(false)
const historyEntries = ref<KvEntryWithId[]>([])

// Search & Pagination State
const searchQuery = ref('')
const currentPage = ref(1)
const itemsPerPage = 10

// Form Inputs
const inputKey = ref('')
const inputType = ref<'string' | 'number' | 'boolean' | 'json'>('string')
const inputValue = ref('')
const inputBool = ref(true)

// Helper: Strip the baseKey prefix for display (e.g. location.bldg.temp -> temp)
function displayKey(fullKey: string) {
  if (!props.baseKey) return fullKey
  const prefix = `${props.baseKey}.`
  return fullKey.startsWith(prefix) ? fullKey.replace(prefix, '') : fullKey
}

// Data Processing
const sortedEntries = computed<KvEntryWithId[]>(() => {
  return Array.from(entries.value.values())
    .sort((a, b) => a.key.localeCompare(b.key))
    .map(entry => ({ ...entry, id: entry.key }))
})

const filteredEntries = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return sortedEntries.value
  return sortedEntries.value.filter(item => {
    return item.key.toLowerCase().includes(q) || 
           JSON.stringify(item.value).toLowerCase().includes(q)
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

// Reset pagination on search
watch(searchQuery, () => { currentPage.value = 1 })

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
</script>

<template>
  <BaseCard :title="`Digital Twin: ${baseKey}`" class="h-full">
    <div v-if="loading" class="flex justify-center p-8">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Initialization State -->
    <div v-else-if="!exists && !error" class="text-center py-8">
      <div class="text-5xl mb-3">🧊</div>
      <h3 class="font-bold text-lg">Bucket Not Initialized</h3>
      <p class="text-sm opacity-70 mb-4">The Organization Twin bucket ({{ bucket || 'twin' }}) does not exist yet.</p>
      <button @click="createBucket()" class="btn btn-primary btn-sm">Initialize Bucket</button>
    </div>

    <!-- Active State -->
    <div v-else class="space-y-4">
      
      <!-- Toolbar: Responsive Header -->
      <div class="flex flex-col sm:flex-row gap-3 justify-between items-start sm:items-center">
        <div v-if="!showForm" class="flex-1 w-full max-w-xs">
          <input 
            v-model="searchQuery" 
            type="text" 
            placeholder="Search keys/values..." 
            class="input input-sm input-bordered w-full" 
          />
        </div>
        <div v-else class="text-sm font-bold opacity-50 uppercase tracking-widest">
          {{ isEditing ? 'Edit Property' : 'New Property' }}
        </div>

        <button 
          v-if="!showForm" 
          @click="showForm = true" 
          class="btn btn-xs btn-primary btn-outline w-full sm:w-auto"
        >
          + Add Property
        </button>
        <button 
          v-else 
          @click="resetForm" 
          class="btn btn-xs btn-ghost w-full sm:w-auto"
        >
          Cancel
        </button>
      </div>

      <!-- Editor Form -->
      <div v-if="showForm" class="bg-base-200 p-4 rounded-lg space-y-4 border border-base-300">
        <div class="flex flex-col gap-3">
          <!-- Key Input with Prefix -->
          <div class="form-control">
            <label class="label p-0 mb-1"><span class="label-text text-xs opacity-60">Property Key</span></label>
            <div class="join w-full">
              <span class="join-item btn btn-sm btn-disabled font-mono text-[10px] hidden md:flex">
                {{ baseKey }}.
              </span>
              <input 
                v-model="inputKey" 
                type="text" 
                placeholder="property_name" 
                class="input input-sm input-bordered join-item flex-1 font-mono" 
                :disabled="isEditing" 
              />
            </div>
            <span class="text-[10px] opacity-40 mt-1 block md:hidden">Full path: {{ baseKey }}.{{ inputKey || '...' }}</span>
          </div>

          <!-- Type Selector -->
          <div class="form-control">
            <label class="label p-0 mb-1"><span class="label-text text-xs opacity-60">Value Type</span></label>
            <select v-model="inputType" class="select select-sm select-bordered w-full">
              <option value="string">String</option>
              <option value="number">Number</option>
              <option value="boolean">Boolean</option>
              <option value="json">JSON</option>
            </select>
          </div>
        </div>

        <!-- Value Input -->
        <div class="form-control">
          <label class="label p-0 mb-1"><span class="label-text text-xs opacity-60">Value</span></label>
          <div v-if="inputType === 'boolean'">
            <input type="checkbox" v-model="inputBool" class="toggle toggle-success" />
          </div>
          <textarea 
            v-else-if="inputType === 'json'" 
            v-model="inputValue" 
            class="textarea textarea-bordered textarea-sm w-full font-mono" 
            rows="4"
          ></textarea>
          <input 
            v-else 
            v-model="inputValue" 
            :type="inputType === 'number' ? 'number' : 'text'" 
            class="input input-sm input-bordered w-full font-mono" 
          />
        </div>
        
        <div class="flex justify-end gap-2">
          <button v-if="isEditing" @click="del(`${baseKey}.${inputKey}`); resetForm()" class="btn btn-sm btn-ghost text-error">Delete</button>
          <button @click="handleSave" class="btn btn-sm btn-primary px-6">Save Property</button>
        </div>

        <!-- History Preview for edits -->
        <div v-if="isEditing && historyEntries.length > 1" class="pt-4 border-t border-base-300">
           <span class="text-[10px] font-bold uppercase opacity-40">Previous Versions ({{ historyEntries.length }})</span>
           <!-- Note: You could add a mini list here if needed, or keep it simple for now -->
        </div>
      </div>

      <!-- List -->
      <div v-if="!showForm" class="border border-base-200 rounded-lg overflow-hidden">
        <ResponsiveList :items="paginatedEntries" :columns="keyColumns" @row-click="selectEntry">
          <!-- Desktop/Cell View -->
          <template #cell-key="{ item }">
            <div class="flex flex-col">
              <span class="font-mono text-primary font-bold">{{ displayKey(item.key) }}</span>
              <span class="text-[10px] opacity-40 truncate max-w-[200px]">{{ item.key }}</span>
            </div>
          </template>
          
          <template #cell-value="{ item }">
            <code class="text-xs opacity-80 break-all">{{ typeof item.value === 'object' ? JSON.stringify(item.value) : item.value }}</code>
          </template>

          <!-- Mobile Card View -->
          <template #card-key="{ item }">
            <div class="flex flex-col">
              <span class="text-[10px] font-bold text-base-content/40 uppercase tracking-widest">Property</span>
              <span class="font-mono text-primary font-bold text-base mt-0.5">{{ displayKey(item.key) }}</span>
              <span class="text-[9px] opacity-30 mt-0.5">{{ item.key }}</span>
            </div>
          </template>

          <template #card-value="{ item }">
            <div class="flex flex-col">
              <span class="text-[10px] font-bold text-base-content/40 uppercase tracking-widest">Value</span>
              <div class="bg-base-200 p-2 rounded mt-1 overflow-x-auto">
                <code class="text-xs opacity-90 whitespace-pre-wrap">{{ typeof item.value === 'object' ? JSON.stringify(item.value, null, 2) : item.value }}</code>
              </div>
            </div>
          </template>

          <template #empty>
            <div class="text-center py-12 opacity-40 italic text-sm">
              {{ searchQuery ? 'No matching properties found' : 'No properties defined for this twin' }}
            </div>
          </template>
        </ResponsiveList>
      </div>

      <!-- Pagination -->
      <div v-if="!showForm && totalPages > 1" class="flex flex-col sm:flex-row justify-between items-center gap-3 pt-2">
        <span class="text-[10px] opacity-50 uppercase font-bold">
          Showing {{ ((currentPage - 1) * itemsPerPage) + 1 }}-{{ Math.min(currentPage * itemsPerPage, totalItems) }} of {{ totalItems }}
        </span>
        <div class="join shadow-sm">
          <button class="join-item btn btn-xs px-3" :disabled="currentPage === 1" @click="currentPage--">«</button>
          <button class="join-item btn btn-xs px-4 cursor-default no-animation">Page {{ currentPage }} of {{ totalPages }}</button>
          <button class="join-item btn btn-xs px-3" :disabled="currentPage === totalPages" @click="currentPage++">»</button>
        </div>
      </div>
    </div>
  </BaseCard>
</template>
