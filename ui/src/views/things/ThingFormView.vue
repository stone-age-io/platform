<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { Thing, ThingType, Location, NatsUser, NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const thingId = route.params.id as string | undefined
const isEdit = computed(() => !!thingId)

// Form data
const formData = ref({
  // Basic
  name: '',
  description: '',
  code: '',
  type: '',
  location: '',
  
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
const thingTypes = ref<ThingType[]>([])
const locations = ref<Location[]>([])
const natsUsers = ref<NatsUser[]>([])
const nebulaHosts = ref<NebulaHost[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load form options (types, locations, nats users, nebula hosts)
 * Backend automatically filters these by organization via API rules
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const [typesRes, locsRes, natsRes, nebulaRes] = await Promise.all([
      pb.collection('thing_types').getFullList<ThingType>({ sort: 'name' }),
      pb.collection('locations').getFullList<Location>({ sort: 'name' }),
      pb.collection('nats_users').getFullList<NatsUser>({ sort: 'nats_username' }),
      pb.collection('nebula_hosts').getFullList<NebulaHost>({ sort: 'hostname' }),
    ])
    
    thingTypes.value = typesRes
    locations.value = locsRes
    natsUsers.value = natsRes
    nebulaHosts.value = nebulaRes
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
      
      // Auth (email only, password hidden)
      email: thing.email,
      password: '',
      passwordConfirm: '',
      
      // Infra
      nats_user: thing.nats_user || '',
      nebula_host: thing.nebula_host || '',
      
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
  
  // Validation: Create mode requires password
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new things')
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
      location: formData.value.location || null,
      email: formData.value.email,
      emailVisibility: true,
      nats_user: formData.value.nats_user || null,
      nebula_host: formData.value.nebula_host || null,
      metadata: formData.value.metadata ? JSON.parse(formData.value.metadata) : null,
    }
    
    // Only send password if it was entered
    if (formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }
    
    if (isEdit.value) {
      await pb.collection('things').update(thingId!, data)
      toast.success('Thing updated')
    } else {
      // Frontend must set organization on create
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
        {{ isEdit ? 'Edit Thing' : 'Provision Thing' }}
      </h1>
    </div>
    
    <!-- Loading State -->
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <!-- Basic Information -->
      <BaseCard title="Basic Information">
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">
              <span class="label-text">Name *</span>
            </label>
            <input 
              v-model="formData.name"
              type="text" 
              placeholder="e.g. Sensor-01"
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
                <option v-for="t in thingTypes" :key="t.id" :value="t.id">
                  {{ t.name }}
                </option>
              </select>
            </div>
          </div>

          <div class="form-control">
            <label class="label">
              <span class="label-text">Location</span>
            </label>
            <select v-model="formData.location" class="select select-bordered">
              <option value="">Select Location...</option>
              <option v-for="loc in locations" :key="loc.id" :value="loc.id">
                {{ loc.name }}
              </option>
            </select>
          </div>
        </div>
      </BaseCard>
      
      <!-- Authentication -->
      <BaseCard title="Authentication">
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">
              <span class="label-text">Email (Identity) *</span>
            </label>
            <input 
              v-model="formData.email"
              type="email" 
              placeholder="thing-uuid@device.local"
              class="input input-bordered"
              required
            />
            <label class="label">
              <span class="label-text-alt">Used by the device to bootstrap/login.</span>
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

      <!-- Infrastructure -->
      <BaseCard title="Connectivity">
        <div class="space-y-4">
          <div class="form-control">
            <label class="label">
              <span class="label-text">NATS User</span>
            </label>
            <select v-model="formData.nats_user" class="select select-bordered font-mono">
              <option value="">None</option>
              <option v-for="user in natsUsers" :key="user.id" :value="user.id">
                {{ user.nats_username }}
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                Links this device to a specific NATS identity.
              </span>
            </label>
          </div>
          
          <div class="form-control">
            <label class="label">
              <span class="label-text">Nebula Host</span>
            </label>
            <select v-model="formData.nebula_host" class="select select-bordered font-mono">
              <option value="">None</option>
              <option v-for="host in nebulaHosts" :key="host.id" :value="host.id">
                {{ host.hostname }} ({{ host.overlay_ip }})
              </option>
            </select>
            <label class="label">
              <span class="label-text-alt">
                Links this device to a Nebula VPN node.
              </span>
            </label>
          </div>
        </div>
      </BaseCard>
      
      <!-- Metadata -->
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
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Thing</span>
        </button>
      </div>
    </form>
  </div>
</template>
