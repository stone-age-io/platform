<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNatsKv } from '@/composables/useNatsKv'
import { useToast } from '@/composables/useToast'
import BaseCard from '@/components/ui/BaseCard.vue'

const props = defineProps<{
  bucket: string
  contextCode: string // e.g. "LOC_01" - used to prefix keys
}>()

const { entries, loading, exists, error, init, createBucket, put, del } = useNatsKv(props.bucket)
const toast = useToast()

// Form State
const showAddForm = ref(false)
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

// Handle Create Bucket
async function handleCreate() {
  await createBucket(`Digital Twin for ${props.contextCode}`)
}

// Handle Save Key
async function handleSave() {
  if (!inputKey.value) {
    toast.error('Key name required')
    return
  }

  // Enforce Naming Convention: PREFIX.keyName
  const fullKey = inputKey.value.startsWith(props.contextCode + '.') 
    ? inputKey.value 
    : `${props.contextCode}.${inputKey.value}`

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
    
    // Reset form
    inputKey.value = ''
    inputValue.value = ''
    showAddForm.value = false
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

// Watch for bucket prop changes (if user navigates from loc A to loc B)
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
          @click="showAddForm = !showAddForm" 
          class="btn btn-xs"
          :class="showAddForm ? 'btn-ghost' : 'btn-primary btn-outline'"
        >
          {{ showAddForm ? 'Cancel' : '+ Add Key' }}
        </button>
      </div>

      <!-- Add Key Form -->
      <div v-if="showAddForm" class="bg-base-200 p-3 rounded-lg space-y-3 animate-fade-in">
        <div class="flex flex-col sm:flex-row gap-2">
          <!-- Key Input -->
          <div class="join flex-1">
            <span class="join-item btn btn-sm btn-static bg-base-300 font-mono text-xs">{{ contextCode }}.</span>
            <input 
              v-model="inputKey" 
              type="text" 
              placeholder="key.name" 
              class="input input-sm input-bordered join-item flex-1 font-mono"
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
            class="input input-sm input-bordered w-full"
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
            rows="3"
            placeholder='{"foo": "bar", "tags": [1, 2]}'
          ></textarea>
        </div>

        <button @click="handleSave" class="btn btn-sm btn-primary w-full">Save Key</button>
      </div>

      <!-- Keys Table -->
      <div class="overflow-x-auto max-h-[400px]">
        <table class="table table-xs table-pin-rows">
          <thead>
            <tr>
              <th>Key</th>
              <th>Value</th>
              <th class="text-right">Rev</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="entry in sortedEntries" :key="entry.key" class="hover">
              <td class="font-mono text-primary">{{ entry.key }}</td>
              
              <td class="max-w-[200px]">
                <div class="flex items-center gap-2">
                  <span class="badge badge-xs badge-outline opacity-50">{{ getValueType(entry.value) }}</span>
                  <span class="truncate font-mono opacity-80">
                    {{ typeof entry.value === 'object' ? JSON.stringify(entry.value) : entry.value }}
                  </span>
                </div>
              </td>
              
              <td class="text-right font-mono opacity-50">{{ entry.revision }}</td>
              
              <td class="text-right">
                <button 
                  @click="del(entry.key)" 
                  class="btn btn-ghost btn-xs text-error opacity-50 hover:opacity-100"
                  title="Delete Key"
                >
                  ✕
                </button>
              </td>
            </tr>
            <tr v-if="sortedEntries.length === 0">
              <td colspan="4" class="text-center py-4 opacity-50 italic">No keys in bucket</td>
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
