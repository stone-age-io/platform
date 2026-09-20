<!-- ui/src/views/locations/LocationFormView.vue -->
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Location, LocationType } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import MetadataEditor from '@/components/common/MetadataEditor.vue'
import ImageUploadField from '@/components/common/ImageUploadField.vue'
import RecordPicker from '@/components/common/RecordPicker.vue'
import { flattenLocationTree, type LocationNode } from '@/utils/locations'
import type { PickerOption } from '@/types/picker'

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
  // An object (or null), not a JSON string — MetadataEditor owns the text form.
  metadata: null as Record<string, any> | null,
})

const metadataEditor = ref<InstanceType<typeof MetadataEditor> | null>(null)

// Image uploads.
//
// The loaded record is kept whole rather than a pair of resolved URLs:
// ImageUploadField resolves its own (every file field is protected and needs a
// file token), so all this view has to hold is what Save should do -- a staged
// file, or a staged removal, per field.
const loadedLocation = ref<Location | null>(null)
const floorplanFile = ref<File | null>(null)
const floorplanRemoved = ref(false)
const photoFile = ref<File | null>(null)
const photoRemoved = ref(false)

// Relation options
const locationTypes = ref<LocationType[]>([])
const parentLocations = ref<LocationNode[]>([])

const typeOptions = computed<PickerOption[]>(() =>
  locationTypes.value.map(t => ({ id: t.id, label: t.name || 'Unnamed' }))
)

/**
 * Self AND everything below it are disabled rather than dropped: a greyed row
 * says why, where a missing row just looks like the record vanished.
 *
 * The descendant half matters as much as the self check. Re-parenting a location
 * under its own descendant makes a cycle, and `flattenLocationTree` can then
 * reach neither end of it -- both fall out of the tree and reappear at the bottom
 * flagged as orphans. That is recoverable rather than silent, but it is still a
 * building that vanished from the picker, and no server rule prevents it.
 */
const parentOptions = computed<PickerOption[]>(() =>
  parentLocations.value.map(n => ({
    id: n.id,
    label: n.orphan ? `${n.name} (orphaned)` : n.name,
    sublabel: n.path || undefined,
    depth: n.depth,
    disabled: !!locationId && (n.id === locationId || n.ancestorIds.includes(locationId)),
  }))
)

// Description of the selected location type, shown under the select.
const selectedTypeHint = computed(() => {
  const t = locationTypes.value.find(x => x.id === formData.value.type)
  return t?.description || ''
})

// The inventory schema declared by the selected location type, if it has one.
// Absent means MetadataEditor shows free-form key/value rows.
const selectedTypeMetadataSchema = computed(() => {
  const t = locationTypes.value.find(x => x.id === formData.value.type)
  return t?.metadata_schema || null
})

// State
const loading = ref(false)
const loadingOptions = ref(true)


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
    parentLocations.value = flattenLocationTree(locationsResult)

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
      metadata: location.metadata && Object.keys(location.metadata).length ? location.metadata : null,
    }
    
    loadedLocation.value = location
  } catch (err: any) {
    toast.error('Failed to load location')
    router.push('/locations')
  } finally {
    loading.value = false
  }
}


/**
 * Commit a metadata JSON tab left mid-edit. False (with a toast) if it doesn't
 * parse, so a bad blob blocks the save rather than being dropped from it.
 */
function validateMetadata(): boolean {
  return !metadataEditor.value || metadataEditor.value.commit()
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
    
    // Metadata. FormData carries strings, so the object is serialised here —
    // this is the one place the string representation still exists, and it is a
    // transport detail rather than the form's model.
    if (formData.value.metadata) {
      formDataToSend.append('metadata', JSON.stringify(formData.value.metadata))
    } else {
      formDataToSend.append('metadata', '')
    }
    
    // Images. An empty value clears a single-file field (PocketBase normalizes
    // it to an empty slice, core/field_file.go); omitting the key entirely
    // leaves the stored file alone. Those are three different intents and all
    // three have to be expressible, which is why each field has a `removed`
    // flag rather than relying on the absence of a staged file.
    if (floorplanFile.value) {
      formDataToSend.append('floorplan', floorplanFile.value)
    } else if (floorplanRemoved.value) {
      formDataToSend.append('floorplan', '')
    }
    if (photoFile.value) {
      formDataToSend.append('photo', photoFile.value)
    } else if (photoRemoved.value) {
      formDataToSend.append('photo', '')
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
                <RecordPicker
                  v-model="formData.type"
                  :options="typeOptions"
                  title="Type"
                  placeholder="Select a type (optional)"
                  clearable
                  clear-label="No type"
                  empty-text="No location types defined yet."
                />
                <!-- Same reasoning as the Thing form's type hint: anyone who can
                     pick a type can read its description. -->
                <label v-if="selectedTypeHint" class="label">
                  <span class="label-text-alt text-base-content/60">{{ selectedTypeHint }}</span>
                </label>
              </div>
              
              <!-- Parent Location -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Parent Location</span>
                </label>
                <RecordPicker
                  v-model="formData.parent"
                  :options="parentOptions"
                  title="Parent location"
                  placeholder="None (top level)"
                  clearable
                  clear-label="None (top level)"
                  empty-text="No other locations yet."
                />
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
          
          <BaseCard title="Site Photo">
            <div class="flex flex-col items-center gap-2">
              <ImageUploadField
                v-model:file="photoFile"
                v-model:removed="photoRemoved"
                :source="
                  loadedLocation?.photo
                    ? { record: loadedLocation, filename: loadedLocation.photo, thumb: '400x400' }
                    : null
                "
                :size="180"
                add-label="Add photo"
                empty-label="No photo"
              />
              <p class="text-xs text-base-content/60 text-center max-w-xs">
                What the site looks like on arrival. Large images are scaled down before upload.
              </p>
            </div>
          </BaseCard>

          <BaseCard title="Floorplan">
            <div class="flex flex-col items-center gap-2">
              <!-- Accepts SVG, which the photo field deliberately does not: a
                   floorplan is frequently exported as a drawing rather than
                   photographed. -->
              <ImageUploadField
                v-model:file="floorplanFile"
                v-model:removed="floorplanRemoved"
                :source="
                  loadedLocation?.floorplan
                    ? { record: loadedLocation, filename: loadedLocation.floorplan }
                    : null
                "
                :size="180"
                accept="image/jpeg,image/png,image/svg+xml,image/gif,image/webp"
                add-label="Add floorplan"
                empty-label="No floorplan"
              />
              <p class="text-xs text-base-content/60 text-center max-w-xs">
                Accepts JPG, PNG, SVG, GIF or WebP. Used as the backdrop for placing things.
              </p>
            </div>
          </BaseCard>
          
          <BaseCard title="Metadata">
            <MetadataEditor
              ref="metadataEditor"
              v-model="formData.metadata"
              :schema="selectedTypeMetadataSchema"
            />
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

