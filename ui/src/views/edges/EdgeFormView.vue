<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Edge, EdgeType, NatsUser, NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import NatsUserFormView from '@/views/nats/NatsUserFormView.vue'
import NebulaHostFormView from '@/views/nebula/NebulaHostFormView.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const edgeId = route.params.id as string | undefined
const isEdit = computed(() => !!edgeId)

// Form data
const formData = ref({
  // Basic
  name: '',
  description: '',
  code: '',
  type: '',
  
  // Auth
  email: '',
  password: '',
  passwordConfirm: '',
  
  // Infrastructure
  nats_user: '',
  nebula_host: '',
  
  // Meta
  metadata: '',
})

// Relation options
const edgeTypes = ref<EdgeType[]>([])
const natsUsers = ref<NatsUser[]>([])
const nebulaHosts = ref<NebulaHost[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

// Modal State
const showNatsModal = ref(false)
const showNebulaModal = ref(false)

/**
 * Load form options
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const [typesRes, natsRes, nebulaRes] = await Promise.all([
      pb.collection('edge_types').getFullList<EdgeType>({ sort: 'name' }),
      pb.collection('nats_users').getFullList<NatsUser>({ sort: 'nats_username' }),
      pb.collection('nebula_hosts').getFullList<NebulaHost>({ sort: 'hostname' }),
    ])
    
    edgeTypes.value = typesRes
    natsUsers.value = natsRes
    nebulaHosts.value = nebulaRes
  } catch (err: any) {
    toast.error('Failed to load form options')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing edge
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
      
      // Auth (email only)
      email: edge.email,
      password: '',
      passwordConfirm: '',
      
      // Infra
      nats_user: edge.nats_user || '',
      nebula_host: edge.nebula_host || '',
      
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
  
  // Validation: Create mode requires password
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new edges')
    return
  }
  
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }
  
  loading.value = true
  
  try {
    const data: any = {
      name: formData.value.name,
      description: formData.value.description || null,
      code: formData.value.code || null,
      type: formData.value.type || null,
      email: formData.value.email,
      emailVisibility: true,
      nats_user: formData.value.nats_user || null,
      nebula_host: formData.value.nebula_host || null,
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
    }
    
    if (formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }
    
    if (isEdit.value) {
      await pb.collection('edges').update(edgeId!, data)
      toast.success('Edge updated')
    } else {
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

// Handlers for Quick Add Success
function onNatsCreated(record: NatsUser) {
  natsUsers.value.push(record)
  natsUsers.value.sort((a, b) => a.nats_username.localeCompare(b.nats_username))
  formData.value.nats_user = record.id
  showNatsModal.value = false
}

function onNebulaCreated(record: NebulaHost) {
  nebulaHosts.value.push(record)
  nebulaHosts.value.sort((a, b) => a.hostname.localeCompare(b.hostname))
  formData.value.nebula_host = record.id
  showNebulaModal.value = false
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
        {{ isEdit ? 'Edit Edge' : 'Provision Edge' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Identity & Auth -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Name *</span>
                </label>
                <input 
                  v-model="formData.name"
                  type="text" 
                  placeholder="e.g. Gateway-01"
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
              
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Code / ID</span>
                  </label>
                  <input 
                    v-model="formData.code"
                    type="text" 
                    placeholder="Unique Identifier"
                    class="input input-bordered font-mono"
                  />
                </div>
                
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Type</span>
                  </label>
                  <select v-model="formData.type" class="select select-bordered">
                    <option value="">Select Type...</option>
                    <option v-for="t in edgeTypes" :key="t.id" :value="t.id">
                      {{ t.name }}
                    </option>
                  </select>
                </div>
              </div>
            </div>
          </BaseCard>
          
          <BaseCard title="Authentication">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Email (Identity) *</span>
                </label>
                <input 
                  v-model="formData.email"
                  type="email" 
                  placeholder="edge-uuid@device.local"
                  class="input input-bordered"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">Used by the edge device to bootstrap/login.</span>
                </label>
              </div>
              
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.password"
                    type="password" 
                    class="input input-bordered"
                    :required="!isEdit"
                    minlength="8"
                  />
                </div>
                
                <div class="form-control">
                  <label class="label">
                    <span class="label-text">Confirm Password {{ isEdit ? '(Optional)' : '*' }}</span>
                  </label>
                  <input 
                    v-model="formData.passwordConfirm"
                    type="password" 
                    class="input input-bordered"
                    :required="!!formData.password"
                  />
                </div>
              </div>
            </div>
          </BaseCard>
        </div>

        <!-- Right Column: Connectivity & Metadata -->
        <div class="space-y-6">
          <BaseCard title="Connectivity">
            <div class="space-y-4">
              <!-- NATS User -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">NATS User</span>
                </label>
                <div class="flex gap-2">
                  <select v-model="formData.nats_user" class="select select-bordered font-mono flex-1">
                    <option value="">None</option>
                    <option v-for="user in natsUsers" :key="user.id" :value="user.id">
                      {{ user.nats_username }}
                    </option>
                  </select>
                  <button 
                    type="button" 
                    class="btn btn-square btn-outline"
                    @click="showNatsModal = true"
                    title="Quick Add NATS User"
                  >
                    +
                  </button>
                </div>
                <label class="label">
                  <span class="label-text-alt">
                    Links this gateway to a specific NATS identity.
                  </span>
                </label>
              </div>
              
              <!-- Nebula Host -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Nebula Host</span>
                </label>
                <div class="flex gap-2">
                  <select v-model="formData.nebula_host" class="select select-bordered font-mono flex-1">
                    <option value="">None</option>
                    <option v-for="host in nebulaHosts" :key="host.id" :value="host.id">
                      {{ host.hostname }} ({{ host.overlay_ip }})
                    </option>
                  </select>
                  <button 
                    type="button" 
                    class="btn btn-square btn-outline"
                    @click="showNebulaModal = true"
                    title="Quick Add Nebula Host"
                  >
                    +
                  </button>
                </div>
                <label class="label">
                  <span class="label-text-alt">
                    Links this gateway to a Nebula VPN node.
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>
          
          <BaseCard title="Metadata (JSON)">
            <div class="form-control">
              <textarea 
                v-model="formData.metadata"
                class="textarea textarea-bordered font-mono"
                rows="10"
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
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Edge</span>
        </button>
      </div>
    </form>

    <!-- NATS Quick Add Modal -->
    <dialog class="modal" :class="{ 'modal-open': showNatsModal }">
      <div class="modal-box w-11/12 max-w-3xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add NATS User</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showNatsModal = false">✕</button>
        </div>
        <div v-if="showNatsModal">
          <NatsUserFormView :embedded="true" @success="onNatsCreated" @cancel="showNatsModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showNatsModal = false"><button>close</button></form>
    </dialog>

    <!-- Nebula Quick Add Modal -->
    <dialog class="modal" :class="{ 'modal-open': showNebulaModal }">
      <div class="modal-box w-11/12 max-w-3xl">
        <div class="flex justify-between items-center mb-4">
          <h3 class="font-bold text-lg">Quick Add Nebula Host</h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="showNebulaModal = false">✕</button>
        </div>
        <div v-if="showNebulaModal">
          <NebulaHostFormView :embedded="true" @success="onNebulaCreated" @cancel="showNebulaModal = false" />
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showNebulaModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
