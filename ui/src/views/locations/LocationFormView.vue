<!-- ui/src/views/locations/LocationFormView.vue -->
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Location, LocationType } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

// Define a local interface for the dropdown options
interface LocationOption extends Location {
  displayName: string
  disabled?: boolean
}

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: Location): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const locationId = route.params.id as string | undefined
const isEdit = computed(() => !!locationId && !props.embedded)

// Form data
const formData = ref({
  name: '',
  description: '',
  code: '',
  type: '',
  parent: '',
  latitude: '',
  longitude: '',
  metadata: '',
})

// File upload
const floorplanFile = ref<File | null>(null)
const currentFloorplan = ref<string | null>(null)

// Relation options
const locationTypes = ref<LocationType[]>([])
const parentLocations = ref<LocationOption[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Helper: Sort locations into a hierarchy and return a flat list with indentation
 */
function sortLocationsHierarchically(items: Location[], currentId?: string): LocationOption[] {
  const result: LocationOption[] = []
  const childrenMap = new Map<string, Location[]>()
  const roots: Location[] = []

  // 1. Build Adjacency Map
  items.forEach(item => {
    if (!item.parent) {
      roots.push(item)
    } else {
      const list = childrenMap.get(item.parent) || []
      list.push(item)
      childrenMap.set(item.parent, list)
    }
  })

  // 2. Sort roots alphabetically (Handle possibly undefined name)
  roots.sort((a, b) => (a.name || '').localeCompare(b.name || ''))

  // 3. Recursive Traversal
  function traverse(node: Location, depth: number) {
    // Grug logic: Indent based on depth
    // \u00A0 is non-breaking space
    const prefix = depth > 0 ? '\u00A0\u00A0\u00A0'.repeat(depth) + '└ ' : ''
    
    result.push({
      ...node,
      displayName: prefix + (node.name || 'Unnamed'),
      // Disable self to prevent circular parent
      disabled: node.id === currentId
    })

    const children = childrenMap.get(node.id) || []
    // Sort children alphabetically (Handle possibly undefined name)
    children.sort((a, b) => (a.name || '').localeCompare(b.name || ''))
    
    children.forEach(child => traverse(child, depth + 1))
  }

  roots.forEach(root => traverse(root, 0))
  
  // 4. Handle Orphans (items whose parents were filtered out or don't exist)
  // If we missed any items in the traversal (e.g. circular refs or missing parents), add them at the end
  const processedIds = new Set(result.map(r => r.id))
  const orphans = items.filter(i => !processedIds.has(i.id))
  
  orphans.forEach(orphan => {
    result.push({
      ...orphan,
      displayName: `[Orphan] ${orphan.name || 'Unnamed'}`,
      disabled: orphan.id === currentId
    })
  })

  return result
}

/**
 * Load form options (types and parent locations)
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const [typesResult, locationsResult] = await Promise.all([
      pb.collection('location_types').getFullList<LocationType>({ sort: 'name' }),
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
    ])
    
    locationTypes.value = typesResult
    
    // Process hierarchy
    parentLocations.value = sortLocationsHierarchically(locationsResult, locationId)
    
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing location for editing
 */
async function loadLocation() {
  if (!locationId || props.embedded) return
  
  loading.value = true
  
  try {
    const location = await pb.collection('locations').getOne<Location>(locationId)
    
    formData.value = {
      name: location.name || '',
      description: location.description || '',
      code: location.code || '',
      type: location.type || '',
      parent: location.parent || '',
      latitude: location.coordinates?.lat?.toString() || '',
      longitude: location.coordinates?.lon?.toString() || '',
      metadata: location.metadata ? JSON.stringify(location.metadata, null, 2) : '',
    }
    
    // Store current floorplan
    if (location.floorplan) {
      currentFloorplan.value = pb.files.getUrl(location, location.floorplan)
    }
  } catch (err: any) {
    toast.error('Failed to load location')
    router.push('/locations')
  } finally {
    loading.value = false
  }
}

/**
 * Handle file selection
 */
function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    floorplanFile.value = target.files[0]
  }
}

/**
 * Validate metadata JSON
 */
function validateMetadata(): boolean {
  if (!formData.value.metadata.trim()) return true
  
  try {
    JSON.parse(formData.value.metadata)
    return true
  } catch {
    toast.error('Invalid JSON in metadata field')
    return false
  }
}

/**
 * Validate coordinates
 */
function validateCoordinates(): boolean {
  // Convert to string to handle both string and number types from number input
  const latStr = String(formData.value.latitude ?? '').trim()
  const lngStr = String(formData.value.longitude ?? '').trim()

  const hasLat = latStr !== ''
  const hasLng = lngStr !== ''

  // Both must be provided or both must be empty
  if (hasLat !== hasLng) {
    toast.error('Both latitude and longitude are required for coordinates')
    return false
  }

  if (hasLat && hasLng) {
    const lat = parseFloat(latStr)
    const lng = parseFloat(lngStr)

    if (isNaN(lat) || isNaN(lng)) {
      toast.error('Latitude and longitude must be valid numbers')
      return false
    }

    if (lat < -90 || lat > 90) {
      toast.error('Latitude must be between -90 and 90')
      return false
    }

    if (lng < -180 || lng > 180) {
      toast.error('Longitude must be between -180 and 180')
      return false
    }
  }

  return true
}

/**
 * Handle form submission
 */
async function handleSubmit() {
  if (!validateMetadata() || !validateCoordinates()) return
  
  loading.value = true
  
  try {
    const formDataToSend = new FormData()
    
    // Basic fields
    formDataToSend.append('name', formData.value.name)
    formDataToSend.append('description', formData.value.description || '')
    formDataToSend.append('code', formData.value.code || '')
    formDataToSend.append('type', formData.value.type || '')
    formDataToSend.append('parent', formData.value.parent || '')
    
    // Coordinates (as JSON object or empty)
    const latStr = String(formData.value.latitude ?? '').trim()
    const lngStr = String(formData.value.longitude ?? '').trim()

    if (latStr !== '' && lngStr !== '') {
      const coordinates = {
        lat: parseFloat(latStr),
        lon: parseFloat(lngStr),
      }
      formDataToSend.append('coordinates', JSON.stringify(coordinates))
    } else {
      formDataToSend.append('coordinates', '')
    }
    
    // Metadata
    if (formData.value.metadata.trim()) {
      formDataToSend.append('metadata', formData.value.metadata)
    } else {
      formDataToSend.append('metadata', '')
    }
    
    // Floorplan file
    if (floorplanFile.value) {
      formDataToSend.append('floorplan', floorplanFile.value)
    }
    
    let record: Location

    if (isEdit.value) {
      // Update existing location
      record = await pb.collection('locations').update<Location>(locationId!, formDataToSend)
      toast.success('Location updated')
    } else {
      // Create new location
      // IMPORTANT: Frontend must set organization
      formDataToSend.append('organization', authStore.currentOrgId!)
      
      record = await pb.collection('locations').create<Location>(formDataToSend)
      toast.success('Location created')
    }
    
    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/locations')
    }
  } catch (err: any) {
    toast.error(err.message || 'Failed to save location')
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

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadLocation()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header: Hide if embedded -->
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/locations">Locations</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Location' : 'Create Location' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Basic Info -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <!-- Name -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Name *</span>
                </label>
                <input 
                  v-model="formData.name"
                  type="text" 
                  placeholder="Enter location name"
                  class="input input-bordered"
                  required
                />
              </div>
              
              <!-- Description -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Description</span>
                </label>
                <textarea 
                  v-model="formData.description"
                  class="textarea textarea-bordered"
                  rows="3"
                  placeholder="Optional description"
                ></textarea>
              </div>
              
              <!-- Code -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Code</span>
                </label>
                <input 
                  v-model="formData.code"
                  type="text" 
                  placeholder="Optional code/identifier"
                  class="input input-bordered font-mono"
                />
              </div>
              
              <!-- Type -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Type</span>
                </label>
                <select v-model="formData.type" class="select select-bordered">
                  <option value="">Select a type (optional)</option>
                  <option v-for="type in locationTypes" :key="type.id" :value="type.id">
                    {{ type.name }}
                  </option>
                </select>
              </div>
              
              <!-- Parent Location -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Parent Location</span>
                </label>
                <select v-model="formData.parent" class="select select-bordered font-mono text-sm">
                  <option value="">None (top level)</option>
                  <option 
                    v-for="location in parentLocations" 
                    :key="location.id" 
                    :value="location.id"
                    :disabled="location.disabled"
                  >
                    {{ location.displayName }}
                  </option>
                </select>
                <label class="label">
                  <span class="label-text-alt">For hierarchical organization (e.g., Building > Floor > Room)</span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Coordinates, Floorplan, Metadata -->
        <div class="space-y-6">
          <BaseCard title="Geo Coordinates">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <!-- Latitude -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Latitude</span>
                </label>
                <input 
                  v-model="formData.latitude"
                  type="number"
                  step="any"
                  placeholder="e.g., 37.7749"
                  class="input input-bordered font-mono"
                />
              </div>
              
              <!-- Longitude -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Longitude</span>
                </label>
                <input 
                  v-model="formData.longitude"
                  type="number"
                  step="any"
                  placeholder="e.g., -122.4194"
                  class="input input-bordered font-mono"
                />
              </div>
            </div>
            <div class="alert alert-info mt-4">
              <span class="text-sm">Both latitude and longitude are required for map display.</span>
            </div>
          </BaseCard>
          
          <BaseCard title="Floorplan">
            <div class="form-control">
              <label class="label">
                <span class="label-text">Upload Floorplan Image</span>
                <span class="label-text-alt">Optional</span>
              </label>
              
              <!-- Current floorplan preview -->
              <div v-if="currentFloorplan && !floorplanFile" class="mb-4">
                <p class="text-sm text-base-content/70 mb-2">Current floorplan:</p>
                <img 
                  :src="currentFloorplan" 
                  alt="Current floorplan"
                  class="max-w-xs rounded border border-base-300"
                />
              </div>
              
              <input 
                type="file"
                accept="image/*"
                @change="handleFileChange"
                class="file-input file-input-bordered"
              />
              <label class="label">
                <span class="label-text-alt">Accepts: JPG, PNG, SVG, GIF, WebP</span>
              </label>
            </div>
          </BaseCard>
          
          <BaseCard title="Metadata (JSON)">
            <div class="form-control">
              <textarea 
                v-model="formData.metadata"
                class="textarea textarea-bordered font-mono"
                rows="6"
                placeholder='{"key": "value"}'
              ></textarea>
            </div>
          </BaseCard>
        </div>
      </div>
      
      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button 
          type="button" 
          @click="handleCancel" 
          class="btn btn-ghost order-2 sm:order-1"
          :disabled="loading"
        >
          Cancel
        </button>
        <button 
          type="submit" 
          class="btn btn-primary order-1 sm:order-2"
          :disabled="loading"
        >
          <span v-if="loading" class="loading loading-spinner"></span>
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Location</span>
        </button>
      </div>
    </form>
  </div>
</template>

