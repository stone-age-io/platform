<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsKv, type KvEntry } from '@/composables/useNatsKv'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'
import { formatDate } from '@/utils/format' // Using generic format util

const props = defineProps<{
  bucket: string
  contextCode: string // e.g. "LOC_01" - used to prefix keys
}>()

const { entries, loading, exists, error, init, createBucket, put, del, getHistory } = useNatsKv(props.bucket)
const toast = useToast()

// UI State
const showForm = ref(false)
const isEditing = ref(false)
const historyLoading = ref(false)
const historyEntries = ref<KvEntry[]>([])

// Form Inputs
const inputKey = ref('')
const inputType = ref<'string' | 'number' | 'boolean' | 'json'>('string')
const inputValue = ref('')
const inputBool = ref(true)

// Computed list for display
const sortedEntries = computed(() => {
  return Array.from(entries.value.values()).sort((a, b) => a.key.localeCompare(b.key))
})

// Initialize
onMounted(() => init())

// Create Bucket
async function handleCreate() {
  await createBucket(`Digital Twin for ${props.contextCode}`)
}

// Reset Form
function resetForm() {
  showForm.value = false
  isEditing.value = false
  inputKey.value = ''
  inputValue.value = ''
  inputType.value = 'string'
  historyEntries.value = []
}

// Open "Add New" Mode
function openAddForm() {
  resetForm()
  showForm.value = true
  // Pre-fill context code for convenience
  // inputKey remains empty, user types suffix
}

// Open "Edit/View" Mode
async function selectEntry(entry: KvEntry) {
  isEditing.value = true
  showForm.value = true
  
  // Strip context code prefix for display if it exists, strictly for visual cleanliness
  // But strictly, we keep the full key in the input and disable it
  inputKey.value = entry.key
  
  // Detect Type
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

  // Load History
  historyLoading.value = true
  historyEntries.value = await getHistory(entry.key)
  historyLoading.value = false
}

// Restore a historical value to the input form
function restoreHistory(entry: KvEntry) {
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

// Handle Save (Put)
async function handleSave() {
  if (!inputKey.value) {
    toast.error('Key name required')
    return
  }

  // If adding new, prepend prefix. If editing, key is already full.
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
    
    // If editing, reload history to show the new revision immediately
    if (isEditing.value) {
      historyLoading.value = true
      historyEntries.value = await getHistory(fullKey)
      historyLoading.value = false
      toast.success('Updated')
    } else {
      resetForm()
    }
  } catch (e: any) {
    toast.error(`Invalid value: ${e.message}`)
  }
}

// Helper to determine type for badges
function getValueType(val: any): string {
  if (val === null) return 'null'
  if (Array.isArray(val)) return 'array'
  return typeof val
}

// Watch for bucket prop changes
watch(() => props.bucket, () => init())
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
      <div class="flex justify-between items-center">
        <div class="text-xs opacity-60 font-mono">{{ entries.size }} Keys</div>
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

      <!-- Editor Form (Add / Edit) -->
      <div v-if="showForm" class="bg-base-200 p-4 rounded-lg space-y-4 border border-base-300 animate-fade-in">
        <h4 class="font-bold text-xs uppercase opacity-50 tracking-wider">
          {{ isEditing ? 'Edit Key' : 'New Key' }}
        </h4>

        <div class="flex flex-col sm:flex-row gap-2">
          <!-- Key Input -->
          <div class="join flex-1">
            <span v-if="!isEditing" class="join-item btn btn-sm btn-static bg-base-300 font-mono text-xs text-base-content/70">
              {{ contextCode }}.
            </span>
            <input 
              v-model="inputKey" 
              type="text" 
              placeholder="key.name" 
              class="input input-sm input-bordered join-item flex-1 font-mono"
              :disabled="isEditing"
            />
          </div>
          
          <!-- Type Selector -->
          <select v-model="inputType" class="select select-sm select-bordered">
            <option value="string">String</option>
            <option value="number">Number</option>
            <option value="boolean">Boolean</option>
            <option value="json">JSON</option>
          </select>
        </div>

        <!-- Value Input -->
        <div>
          <input 
            v-if="inputType === 'string'" 
            v-model="inputValue" 
            type="text" 
            placeholder="Value..." 
            class="input input-sm input-bordered w-full font-mono"
          />
          
          <input 
            v-if="inputType === 'number'" 
            v-model="inputValue" 
            type="number" 
            placeholder="0.00" 
            class="input input-sm input-bordered w-full font-mono"
          />
          
          <div v-if="inputType === 'boolean'" class="form-control">
            <label class="label cursor-pointer justify-start gap-3">
              <input type="checkbox" v-model="inputBool" class="toggle toggle-sm toggle-success" />
              <span class="font-mono">{{ inputBool }}</span>
            </label>
          </div>

          <textarea 
            v-if="inputType === 'json'"
            v-model="inputValue"
            class="textarea textarea-bordered textarea-sm w-full font-mono leading-tight"
            rows="4"
            placeholder='{"foo": "bar", "tags": [1, 2]}'
          ></textarea>
        </div>

        <div class="flex justify-end gap-2">
          <button v-if="isEditing" @click="del(inputKey); resetForm()" class="btn btn-sm btn-ghost text-error">Delete Key</button>
          <button @click="handleSave" class="btn btn-sm btn-primary min-w-[100px]">
            {{ isEditing ? 'Update Value' : 'Save Key' }}
          </button>
        </div>

        <!-- History Table (Only in Edit Mode) -->
        <div v-if="isEditing" class="pt-2 border-t border-base-content/10">
          <div class="font-bold text-xs uppercase opacity-50 tracking-wider mb-2">Revision History</div>
          
          <div v-if="historyLoading" class="text-center py-4">
            <span class="loading loading-dots loading-xs"></span>
          </div>
          
          <div v-else class="overflow-x-auto max-h-[150px] border border-base-300 rounded bg-base-100">
            <table class="table table-xs table-pin-rows">
              <thead>
                <tr>
                  <th>Rev</th>
                  <th>Op</th>
                  <th>Value</th>
                  <th>Time</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="h in historyEntries" :key="h.revision" class="hover group">
                  <td class="font-mono text-xs opacity-50">{{ h.revision }}</td>
                  <td>
                    <span class="badge badge-xs" :class="h.operation === 'DEL' ? 'badge-error' : 'badge-success'">
                      {{ h.operation }}
                    </span>
                  </td>
                  <td class="max-w-[150px] truncate font-mono text-xs opacity-70">
                    {{ h.operation === 'DEL' ? '-' : JSON.stringify(h.value) }}
                  </td>
                  <td class="text-xs opacity-50 whitespace-nowrap">{{ formatDate(h.created) }}</td>
                  <td class="text-right">
                    <button 
                      v-if="h.operation !== 'DEL'"
                      @click="restoreHistory(h)"
                      class="btn btn-xs btn-ghost opacity-0 group-hover:opacity-100"
                      title="Load this value to editor"
                    >
                      Load
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Keys Table -->
      <div v-if="!showForm" class="overflow-x-auto max-h-[400px] border border-base-200 rounded-lg">
        <table class="table table-xs table-pin-rows">
          <thead>
            <tr>
              <th>Key</th>
              <th>Value</th>
              <th class="text-right">Rev</th>
            </tr>
          </thead>
          <tbody>
            <tr 
              v-for="entry in sortedEntries" 
              :key="entry.key" 
              class="hover cursor-pointer transition-colors"
              @click="selectEntry(entry)"
            >
              <td class="font-mono text-primary font-medium">{{ entry.key }}</td>
              
              <td class="max-w-[200px]">
                <div class="flex items-center gap-2">
                  <span class="badge badge-xs badge-outline opacity-50">{{ getValueType(entry.value) }}</span>
                  <span class="truncate font-mono opacity-80 text-xs">
                    {{ typeof entry.value === 'object' ? JSON.stringify(entry.value) : entry.value }}
                  </span>
                </div>
              </td>
              
              <td class="text-right font-mono opacity-50">{{ entry.revision }}</td>
            </tr>
            <tr v-if="sortedEntries.length === 0">
              <td colspan="3" class="text-center py-8 opacity-50 italic">No keys in bucket</td>
            </tr>
          </tbody>
        </table>
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
