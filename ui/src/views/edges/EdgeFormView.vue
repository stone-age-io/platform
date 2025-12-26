<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Edge, EdgeType } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const edgeId = route.params.id as string | undefined
const isEdit = computed(() => !!edgeId)

// Form data
const formData = ref({
  name: '',
  description: '',
  code: '',
  type: '',
  metadata: '',
})

// Relation options
const edgeTypes = ref<EdgeType[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load form options (types)
 * Backend automatically filters these by organization
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const typesResult = await pb.collection('edge_types').getFullList<EdgeType>({ 
      sort: 'name' 
    })
    
    edgeTypes.value = typesResult
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing edge for editing
 */
async function loadEdge() {
  if (!edgeId) return
  
  loading.value = true
  
  try {
    const edge = await pb.collection('edges').getOne<Edge>(edgeId)
    
    formData.value = {
      name: edge.name || '',
      description: edge.description || '',
      code: edge.code || '',
      type: edge.type || '',
      metadata: edge.metadata ? JSON.stringify(edge.metadata, null, 2) : '',
    }
  } catch (err: any) {
    toast.error('Failed to load edge')
    router.push('/edges')
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
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
    }
    
    if (isEdit.value) {
      // Update existing edge
      await pb.collection('edges').update(edgeId!, data)
      toast.success('Edge updated')
    } else {
      // Create new edge
      // IMPORTANT: Frontend must set organization
      data.organization = authStore.currentOrgId
      
      await pb.collection('edges').create(data)
      toast.success('Edge created')
    }
    
    router.push('/edges')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save edge')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadEdge()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/edges">Edges</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Edge' : 'Create Edge' }}
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
              placeholder="Enter edge name"
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
              <option v-for="type in edgeTypes" :key="type.id" :value="type.id">
                {{ type.name }}
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
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Edge</span>
        </button>
      </div>
    </form>
  </div>
</template>
