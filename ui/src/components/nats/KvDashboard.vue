<!-- ui/src/components/nats/KvDashboard.vue -->
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useNatsKv, type KvEntry } from '@/composables/useNatsKv'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import BaseCard from '@/components/ui/BaseCard.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'

interface KvEntryWithId extends KvEntry {
  id: string
}

const props = defineProps<{
  bucket?: string
  baseKey: string 
}>()

const { entries, loading, exists, error, init, createBucket, put, del, getHistory } = useNatsKv(props.bucket || 'twin', props.baseKey)
const toast = useToast()

// --- UI & Pagination State ---
const selectedEntry = ref<KvEntryWithId | null>(null)
const isAddingNew = ref(false)
const historyLoading = ref(false)
const historyEntries = ref<KvEntryWithId[]>([])

const currentPage = ref(1)
const itemsPerPage = 10 // CHANGED: Increased to 10

// --- Form Inputs ---
const inputKey = ref('')
const inputType = ref<'string' | 'number' | 'boolean' | 'json' | 'datetime'>('string')
const inputValue = ref('')
const inputBool = ref(true)
const inputDate = ref('')

function displayKey(fullKey: string) {
  if (!props.baseKey) return fullKey
  const prefix = `${props.baseKey}.`
  return fullKey.startsWith(prefix) ? fullKey.replace(prefix, '') : fullKey
}

// --- Data Processing ---
const allEntries = computed<KvEntryWithId[]>(() => {
  return Array.from(entries.value.values())
    .sort((a, b) => a.key.localeCompare(b.key))
    .map(entry => ({ ...entry, id: entry.key }))
})

const totalPages = computed(() => Math.ceil(allEntries.value.length / itemsPerPage))
const paginatedEntries = computed(() => {
  const start = (currentPage.value - 1) * itemsPerPage
  return allEntries.value.slice(start, start + itemsPerPage)
})

// --- Actions ---
onMounted(() => init())

async function handleSave() {
  if (!inputKey.value) return toast.error('Property name required')
  let finalValue: any = inputValue.value
  
  try {
    if (inputType.value === 'number') finalValue = Number(inputValue.value)
    else if (inputType.value === 'boolean') finalValue = inputBool.value
    else if (inputType.value === 'json') finalValue = JSON.parse(inputValue.value)
    else if (inputType.value === 'datetime') finalValue = new Date(inputDate.value).toISOString()
    
    await put(inputKey.value, finalValue)
    toast.success('Property Saved')
    
    if (isAddingNew.value) {
      isAddingNew.value = false
      const newKey = `${props.baseKey}.${inputKey.value}`
      const entry = entries.value.get(newKey)
      if (entry) openEdit({ ...entry, id: entry.key })
    } else {
      loadHistory(selectedEntry.value!.key)
    }
  } catch (e: any) {
    toast.error(`Invalid value: ${e.message}`)
  }
}

function openEdit(entry: KvEntryWithId) {
  isAddingNew.value = false
  selectedEntry.value = entry
  inputKey.value = displayKey(entry.key)
  
  const val = entry.value
  if (typeof val === 'boolean') {
    inputType.value = 'boolean'
    inputBool.value = val
  } else if (typeof val === 'number') {
    inputType.value = 'number'
    inputValue.value = String(val)
  } else if (typeof val === 'object') {
    inputType.value = 'json'
    inputValue.value = JSON.stringify(val, null, 2)
  } else if (typeof val === 'string' && !isNaN(Date.parse(val)) && val.includes('T')) {
    inputType.value = 'datetime'
    inputDate.value = val.substring(0, 16)
  } else {
    inputType.value = 'string'
    inputValue.value = String(val)
  }
  
  loadHistory(entry.key)
}

function openAdd() {
  isAddingNew.value = true
  selectedEntry.value = null
  inputKey.value = ''
  inputValue.value = ''
  inputType.value = 'string'
  historyEntries.value = []
}

function closeEditor() {
  selectedEntry.value = null
  isAddingNew.value = false
}

async function loadHistory(key: string) {
  historyLoading.value = true
  try {
    const raw = await getHistory(key)
    historyEntries.value = raw.map(h => ({ ...h, id: `${h.key}-${h.revision}` }))
  } finally {
    historyLoading.value = false
  }
}

function isObject(val: any) {
  return typeof val === 'object' && val !== null
}
</script>

<template>
  <BaseCard :title="`Digital Twin: ${baseKey}`" :no-padding="true" class="w-full overflow-hidden">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner text-primary"></span>
    </div>

    <!-- Initialization State -->
    <div v-else-if="!exists && !error" class="text-center py-12">
      <div class="text-5xl mb-3">🧊</div>
      <h3 class="font-bold text-lg">Bucket Not Initialized</h3>
      <button @click="createBucket()" class="btn btn-primary btn-sm mt-4">Initialize Bucket</button>
    </div>

    <!-- Main Layout -->
    <div v-else class="flex flex-col lg:flex-row min-h-[700px] relative">
      
      <!-- LEFT: Property List (65% on Desktop) -->
      <div class="list-pane">
        <div class="pane-header">
          <span class="text-[10px] font-bold uppercase tracking-widest opacity-50">Properties ({{ allEntries.length }})</span>
          <button @click="openAdd" class="btn btn-xs btn-primary">+ Add</button>
        </div>

        <div class="flex-1 overflow-y-auto">
          <table class="table table-sm w-full border-separate border-spacing-0">
            <thead class="sticky top-0 bg-base-100 z-10 shadow-sm">
              <tr>
                <th class="bg-base-100 py-3">Property</th>
                <th class="bg-base-100 py-3">Value</th>
                <th class="bg-base-100 py-3 text-right">Rev</th>
              </tr>
            </thead>
            <tbody>
              <tr 
                v-for="item in paginatedEntries" 
                :key="item.key" 
                @click="openEdit(item)"
                class="hover:bg-primary/5 cursor-pointer transition-colors group"
                :class="{ 'bg-primary/10': selectedEntry?.key === item.key }"
              >
                <td class="py-4 align-top">
                  <div class="flex flex-col">
                    <span class="font-mono font-bold text-primary group-hover:underline">{{ displayKey(item.key) }}</span>
                  </div>
                </td>
                <td class="py-4 align-top">
                  <!-- CHANGED: max-h-[7.5rem] allows for ~5 lines of JSON preview -->
                  <div v-if="isObject(item.value)" class="max-h-[7.5rem] overflow-hidden relative opacity-80">
                    <JsonViewer :data="item.value" />
                    <div class="absolute bottom-0 left-0 right-0 h-6 bg-gradient-to-t from-base-100 to-transparent"></div>
                  </div>
                  <span v-else-if="typeof item.value === 'boolean'" class="badge badge-sm" :class="item.value ? 'badge-success' : 'badge-ghost'">
                    {{ item.value ? 'TRUE' : 'FALSE' }}
                  </span>
                  <span v-else class="text-xs font-medium truncate block max-w-xs">{{ item.value }}</span>
                </td>
                <td class="py-4 text-right align-top font-mono text-[10px] opacity-40">
                  {{ item.revision }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination Footer -->
        <div class="p-3 border-t border-base-300 flex justify-between items-center bg-base-200/10">
          <span class="text-[10px] opacity-50">Page {{ currentPage }} of {{ totalPages || 1 }}</span>
          <div class="join">
            <button class="join-item btn btn-xs" :disabled="currentPage === 1" @click="currentPage--">«</button>
            <button class="join-item btn btn-xs" :disabled="currentPage >= totalPages" @click="currentPage++">»</button>
          </div>
        </div>
      </div>

      <!-- RIGHT: Editor Pane (35% on Desktop) -->
      <Transition name="mobile-drawer">
        <div 
          v-if="selectedEntry || isAddingNew || true" 
          class="editor-pane"
          :class="{ 'mobile-active': selectedEntry || isAddingNew }"
        >
          <!-- Mobile Backdrop -->
          <div class="mobile-backdrop lg:hidden" @click="closeEditor"></div>

          <!-- Editor Content -->
          <div class="editor-content-wrapper">
            <div class="pane-header bg-base-200/50">
              <h3 class="font-bold uppercase text-[10px] tracking-widest opacity-70">
                {{ isAddingNew ? 'New Property' : 'Property Details' }}
              </h3>
              <button @click="closeEditor" class="btn btn-xs btn-ghost btn-circle">✕</button>
            </div>

            <div class="flex-1 overflow-y-auto p-5">
              <div v-if="selectedEntry || isAddingNew" class="space-y-6">
                <!-- Form -->
                <div class="space-y-4">
                  <div class="form-control">
                    <label class="label p-0 mb-1"><span class="label-text text-[10px] font-bold opacity-50 uppercase">Key Name</span></label>
                    <input v-model="inputKey" type="text" class="input input-sm input-bordered font-mono" :disabled="!isAddingNew" />
                  </div>

                  <div class="form-control">
                    <label class="label p-0 mb-1"><span class="label-text text-[10px] font-bold opacity-50 uppercase">Type</span></label>
                    <select v-model="inputType" class="select select-sm select-bordered w-full">
                      <option value="string">String</option>
                      <option value="number">Number</option>
                      <option value="boolean">Boolean</option>
                      <option value="datetime">Date / Time</option>
                      <option value="json">JSON Object</option>
                    </select>
                  </div>

                  <div class="form-control">
                    <label class="label p-0 mb-1"><span class="label-text text-[10px] font-bold opacity-50 uppercase">Value</span></label>
                    <input v-if="inputType === 'boolean'" type="checkbox" v-model="inputBool" class="toggle toggle-success" />
                    <input v-else-if="inputType === 'datetime'" type="datetime-local" v-model="inputDate" class="input input-sm input-bordered w-full font-mono" />
                    <textarea v-else-if="inputType === 'json'" v-model="inputValue" class="textarea textarea-bordered font-mono text-xs h-32"></textarea>
                    <input v-else v-model="inputValue" :type="inputType === 'number' ? 'number' : 'text'" class="input input-sm input-bordered w-full font-mono" />
                  </div>

                  <div class="flex gap-2 pt-2">
                    <button v-if="!isAddingNew" @click="del(selectedEntry!.key); closeEditor()" class="btn btn-sm btn-error btn-outline flex-1">Delete</button>
                    <button @click="handleSave" class="btn btn-sm btn-primary flex-1">Save</button>
                  </div>
                </div>

                <!-- History -->
                <div v-if="!isAddingNew" class="pt-4 border-t border-base-300">
                  <span class="text-[10px] font-bold uppercase opacity-30 block mb-4">Revision History</span>
                  <div v-if="historyLoading" class="flex justify-center"><span class="loading loading-dots loading-xs"></span></div>
                  <div v-else class="space-y-4">
                    <div v-for="rev in historyEntries" :key="rev.revision" class="relative pl-4 border-l border-base-300 group">
                      <div class="absolute -left-[4.5px] top-1 w-2 h-2 rounded-full bg-base-300 group-hover:bg-primary transition-colors"></div>
                      <div class="flex justify-between text-[9px] opacity-50 mb-1">
                        <span class="font-bold">REV #{{ rev.revision }}</span>
                        <span>{{ formatDate(rev.created, 'MMM d, HH:mm') }}</span>
                      </div>
                      <div class="text-[10px] font-mono break-all opacity-80">
                        {{ isObject(rev.value) ? JSON.stringify(rev.value) : rev.value }}
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Desktop Empty State -->
              <div v-else class="hidden lg:flex h-full flex-col items-center justify-center text-center opacity-30 py-20">
                <span class="text-4xl mb-2">👈</span>
                <p class="text-xs font-bold uppercase tracking-widest">Select a property<br>to view or edit</p>
              </div>
            </div>
          </div>
        </div>
      </Transition>

    </div>
  </BaseCard>
</template>

<style scoped>
.pane-header {
  padding: 1rem;
  border-bottom: 1px solid oklch(var(--b3));
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: oklch(var(--b2) / 0.2);
  height: 48px;
  flex-shrink: 0;
}

/* --- DESKTOP STYLES --- */
@media (min-width: 1024px) {
  .list-pane {
    flex: 0 0 65%;
    width: 65%;
    display: flex;
    flex-direction: column;
    border-right: 1px solid oklch(var(--b3));
  }
  
  .editor-pane {
    flex: 0 0 35%;
    width: 35%;
    display: flex;
    flex-direction: column;
    background: oklch(var(--b2) / 0.2);
  }
  
  .editor-content-wrapper {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .mobile-backdrop { display: none; }
}

/* --- MOBILE STYLES --- */
@media (max-width: 1023px) {
  .list-pane {
    width: 100%;
    display: flex;
    flex-direction: column;
  }

  .editor-pane {
    position: fixed;
    inset: 0;
    z-index: 100;
    display: none;
  }
  
  .editor-pane.mobile-active {
    display: flex;
  }

  .mobile-backdrop {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(4px);
  }

  .editor-content-wrapper {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    width: 90%;
    max-width: 400px;
    background: oklch(var(--b1));
    display: flex;
    flex-direction: column;
    box-shadow: -10px 0 30px rgba(0, 0, 0, 0.5);
  }
}

/* Transitions */
.mobile-drawer-enter-active, .mobile-drawer-leave-active {
  transition: all 0.3s ease;
}

@media (max-width: 1023px) {
  .mobile-drawer-enter-from, .mobile-drawer-leave-to {
    transform: translateX(100%);
    opacity: 0;
  }
}

.overflow-y-auto::-webkit-scrollbar {
  width: 4px;
}
.overflow-y-auto::-webkit-scrollbar-thumb {
  background: oklch(var(--bc) / 0.1);
  border-radius: 10px;
}
</style>
