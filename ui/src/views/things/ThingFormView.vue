<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Thing, ThingType, Location } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const thingId = route.params.id as string | undefined
const isEdit = computed(() => !!thingId)

// Form data
const formData = ref({
  name: '',
  description: '',
  code: '',
  type: '',
  location: '',
  metadata: '',
})

// Relation options
const thingTypes = ref<ThingType[]>([])
const locations = ref<Location[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load form options (types and locations)
 * Backend automatically filters these by organization
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const [typesResult, locationsResult] = await Promise.all([
      pb.collection('thing_types').getFullList<ThingType>({ sort: 'name' }),
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
    ])
    
    thingTypes.value = typesResult
    locations.value = locationsResult
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing thing for editing
 */
async function loadThing() {
  if (!thingId) return
  
  loading.value = true
  
  try {
    const thing = await pb.collection('things').getOne<Thing>(thingId)
    
    formData.value = {
      name: thing.name || '',
      description: thing.description || '',
      code: thing.code || '',
      type: thing.type || '',
      location: thing.location || '',
      metadata: thing.metadata ? JSON.stringify(thing.metadata, null, 2) : '',
    }
  } catch (err: any) {
    toast.error('Failed to load thing')
    router.push('/things')
  } finally {
    loading.value = false
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
 * Handle form submission
 */
async function handleSubmit() {
  if (!validateMetadata()) return
  
  loading.value = true
  
  try {
    const data: any = {
      name: formData.value.name,
      description: formData.value.description || null,
      code: formData.value.code || null,
      type: formData.value.type || null,
      location: formData.value.location || null,
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
    }
    
    if (isEdit.value) {
      // Update existing thing
      await pb.collection('things').update(thingId!, data)
      toast.success('Thing updated')
    } else {
      // Create new thing
      // IMPORTANT: Frontend must set organization
      data.organization = authStore.currentOrgId
      
      await pb.collection('things').create(data)
      toast.success('Thing created')
    }
    
    router.push('/things')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save thing')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadThing()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/things">Things</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Thing' : 'Create Thing' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
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
              placeholder="Enter thing name"
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
              <option v-for="type in thingTypes" :key="type.id" :value="type.id">
                {{ type.name }}
              </option>
            </select>
          </div>
          
          <!-- Location -->
          <div class="form-control">
            <label class="label">
              <span class="label-text">Location</span>
            </label>
            <select v-model="formData.location" class="select select-bordered">
              <option value="">Select a location (optional)</option>
              <option v-for="location in locations" :key="location.id" :value="location.id">
                {{ location.name }}
              </option>
            </select>
          </div>
        </div>
      </BaseCard>
      
      <!-- Metadata -->
      <BaseCard title="Metadata (JSON)">
        <div class="form-control">
          <label class="label">
            <span class="label-text">Custom metadata as JSON</span>
            <span class="label-text-alt">Optional</span>
          </label>
          <textarea 
            v-model="formData.metadata"
            class="textarea textarea-bordered font-mono"
            rows="6"
            placeholder='{"key": "value"}'
          ></textarea>
          <label class="label">
            <span class="label-text-alt">Must be valid JSON</span>
          </label>
        </div>
      </BaseCard>
      
      <!-- Actions -->
      <div class="flex flex-col sm:flex-row justify-end gap-2 sm:gap-4">
        <button 
          type="button" 
          @click="router.back()" 
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
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Thing</span>
        </button>
      </div>
    </form>
  </div>
</template>
