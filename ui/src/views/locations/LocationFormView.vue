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

// The record is kept whole because ImageUploadField resolves its own (protected,
// file-token) URLs; this view holds only a staged file or removal per field.
const loadedLocation = ref<Location | null>(null)

// A type, once set, is frozen by locations.updateRule (ADR 0003): generated codes
// carry its prefix.
const typeLocked = computed(() => isEdit.value && !!loadedLocation.value?.type)
const floorplanFile = ref<File | null>(null)
const floorplanRemoved = ref(false)
const photoFile = ref<File | null>(null)
const photoRemoved = ref(false)

const locationTypes = ref<LocationType[]>([])
const parentLocations = ref<LocationNode[]>([])

const typeOptions = computed<PickerOption[]>(() =>
  locationTypes.value.map(t => ({ id: t.id, label: t.name || 'Unnamed' }))
)

/**
 * Self and its descendants are disabled (not dropped, so the row says why):
 * re-parenting under a descendant makes a cycle. The server also refuses that
 * save (hooks/location_path.go); this is the friendly half.
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

const selectedTypeHint = computed(() => {
  const t = locationTypes.value.find(x => x.id === formData.value.type)
  return t?.description || ''
})

// Null means free-form key/value rows.
const selectedTypeMetadataSchema = computed(() => {
  const t = locationTypes.value.find(x => x.id === formData.value.type)
  return t?.metadata_schema || null
})

const loading = ref(false)
const loadingOptions = ref(true)

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

function validateCoordinates(): boolean {
  // A number input can yield a string or a number
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

async function handleSubmit() {
  if (!validateMetadata() || !validateCoordinates()) return
  
  loading.value = true
  
  try {
    const formDataToSend = new FormData()
    
    formDataToSend.append('name', formData.value.name)
    formDataToSend.append('description', formData.value.description || '')
    formDataToSend.append('code', formData.value.code || '')
    formDataToSend.append('type', formData.value.type || '')
    formDataToSend.append('parent', formData.value.parent || '')
    
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
    
    // FormData carries strings; serialising here is a transport detail only.
    if (formData.value.metadata) {
      formDataToSend.append('metadata', JSON.stringify(formData.value.metadata))
    } else {
      formDataToSend.append('metadata', '')
    }
    
    // An empty value clears a file field; omitting the key keeps the stored file.
    // Hence the explicit `removed` flag per field.
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
      record = await pb.collection('locations').update<Location>(locationId!, formDataToSend)
      toast.success('Location updated')
    } else {
      // The client must set organization on create
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
    
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <!-- Photo beside Name and Description only, as in ThingFormView.
                   Unlike a Thing's, it works on create too: locations use the
                   plain record API with FormData. -->
              <div class="flex flex-col sm:flex-row sm:items-start gap-5">
                <div class="shrink-0 sm:order-last">
                  <ImageUploadField
                    v-model:file="photoFile"
                    v-model:removed="photoRemoved"
                    :source="
                      loadedLocation?.photo
                        ? { record: loadedLocation, filename: loadedLocation.photo, thumb: '400x400' }
                        : null
                    "
                    :size="140"
                    add-label="Add photo"
                    empty-label="No photo"
                  />
                </div>

                <div class="flex-1 min-w-0 space-y-4">
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

                  <div class="form-control">
                    <label class="label">
                      <span class="label-text">Description</span>
                    </label>
                    <textarea
                      v-model="formData.description"
                      class="textarea textarea-bordered"
                      rows="2"
                      placeholder="Optional description"
                    ></textarea>
                  </div>
                </div>
              </div>
              
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Code</span>
                </label>
                <input 
                  v-model="formData.code"
                  type="text" 
                  :placeholder="isEdit ? '' : 'e.g. RM-204, or leave blank to generate one'"
                  class="input input-bordered font-mono"
                  :readonly="isEdit"
                />
                <!-- An installer code (RM-204 on the door) is preferred; the
                     generated one (hooks/codes.go) is the fallback. ADR 0003. -->
                <label class="label">
                  <span class="label-text-alt">
                    {{ !isEdit
                      ? 'Use the name on the door or drawing if there is one. Blank gets a generated code under the type prefix. Cannot be changed later.'
                      : 'Frozen: it is printed on labels and built into subjects.' }}
                  </span>
                </label>
              </div>
              
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Type</span>
                </label>
                <RecordPicker
                  v-model="formData.type"
                  :options="typeOptions"
                  title="Type"
                  placeholder="Select a type (optional)"
                  :clearable="!typeLocked"
                  :disabled="typeLocked"
                  clear-label="No type"
                  empty-text="No location types defined yet."
                />
                <label v-if="typeLocked" class="label">
                  <span class="label-text-alt text-base-content/60">
                    A type cannot be changed once set. To fix a wrong one, delete and recreate the Location.
                  </span>
                </label>
                <!-- Anyone who can pick a type can read its description. -->
                <label v-if="selectedTypeHint" class="label">
                  <span class="label-text-alt text-base-content/60">{{ selectedTypeHint }}</span>
                </label>
              </div>
              
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

        <div class="space-y-6">
          <BaseCard title="Geo Coordinates">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
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
            <div class="flex flex-col items-center gap-2">
              <!-- Accepts SVG (the photo field deliberately does not): floorplans
                   are often exported drawings. -->
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

