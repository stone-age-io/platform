<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NebulaHost, NebulaNetwork } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const props = defineProps<{
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'success', record: NebulaHost): void
  (e: 'cancel'): void
}>()

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const hostId = route.params.id as string | undefined
const isEdit = computed(() => !!hostId && !props.embedded)

// Form data
const formData = ref({
  // Identity
  hostname: '',
  email: '',
  password: '',
  passwordConfirm: '',
  
  // Network
  network_id: '',
  groups: '', // Managed as string for input
  overlay_ip: '',
  
  // Config
  is_lighthouse: false,
  public_host_port: '',
  active: true,
})

// Relation options
const networks = ref<NebulaNetwork[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load options (networks)
 */
async function loadOptions() {
  loadingOptions.value = true
  
  try {
    networks.value = await pb.collection('nebula_networks').getFullList<NebulaNetwork>({ 
      sort: 'name',
      filter: 'active = true'
    })
    
    // Auto-select first network if creating and only one exists
    if (!isEdit.value && networks.value.length === 1) {
      formData.value.network_id = networks.value[0].id
    }
  } catch (err: any) {
    toast.error('Failed to load networks')
  } finally {
    loadingOptions.value = false
  }
}

/**
 * Load existing host
 */
async function loadHost() {
  if (!hostId || props.embedded) return
  
  loading.value = true
  
  try {
    const host = await pb.collection('nebula_hosts').getOne<NebulaHost>(hostId)
    
    // Convert groups array to comma-separated string
    const groupsStr = Array.isArray(host.groups) ? host.groups.join(', ') : ''
    
    formData.value = {
      hostname: host.hostname,
      email: host.email,
      password: '',
      passwordConfirm: '',
      network_id: host.network_id,
      groups: groupsStr,
      overlay_ip: host.overlay_ip || '',
      is_lighthouse: host.is_lighthouse || false,
      public_host_port: host.public_host_port || '',
      active: host.active ?? true,
    }
  } catch (err: any) {
    toast.error('Failed to load Nebula host')
    router.push('/nebula/hosts')
  } finally {
    loading.value = false
  }
}

/**
 * Handle form submission
 */
async function handleSubmit() {
  // Validation: Create mode requires password
  if (!isEdit.value && !formData.value.password) {
    toast.error('Password is required for new hosts')
    return
  }
  
  if (formData.value.password && formData.value.password !== formData.value.passwordConfirm) {
    toast.error('Passwords do not match')
    return
  }

  loading.value = true
  
  try {
    // Process groups string into array
    const groupsArray = formData.value.groups
      .split(',')
      .map(g => g.trim())
      .filter(g => g.length > 0)

    const data: any = {
      hostname: formData.value.hostname,
      email: formData.value.email,
      emailVisibility: true,
      network_id: formData.value.network_id,
      groups: groupsArray,
      is_lighthouse: formData.value.is_lighthouse,
      public_host_port: formData.value.public_host_port || null,
      active: formData.value.active,
    }
    
    // Only send password if entered
    if (formData.value.password) {
      data.password = formData.value.password
      data.passwordConfirm = formData.value.passwordConfirm
    }
    
    // Only send overlay_ip if explicitly set (usually backend handles this)
    if (formData.value.overlay_ip) {
      data.overlay_ip = formData.value.overlay_ip
    }
    
    let record: NebulaHost

    if (isEdit.value) {
      record = await pb.collection('nebula_hosts').update<NebulaHost>(hostId!, data)
      toast.success('Nebula host updated')
    } else {
      data.organization = authStore.currentOrgId
      record = await pb.collection('nebula_hosts').create<NebulaHost>(data)
      toast.success('Nebula host created')
    }
    
    if (props.embedded) {
      emit('success', record)
    } else {
      router.push('/nebula/hosts')
    }
  } catch (err: any) {
    toast.error(err.message || 'Failed to save Nebula host')
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
    loadHost()
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header: Hidden if Embedded -->
    <div v-if="!embedded">
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nebula/hosts">Nebula Hosts</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Nebula Host' : 'Provision Nebula Host' }}
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
          <BaseCard title="Identity">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Hostname *</span>
                </label>
                <input 
                  v-model="formData.hostname"
                  type="text" 
                  placeholder="e.g. laptop-01, server-prod"
                  class="input input-bordered font-mono"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">
                    Unique identifier for this host in the Nebula network
                  </span>
                </label>
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
                  placeholder="host-uuid@nebula.local"
                  class="input input-bordered"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">Unique email used for PocketBase authentication record.</span>
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

        <!-- Right Column: Network, Role, Status -->
        <div class="space-y-6">
          <BaseCard title="Network Configuration">
            <div class="space-y-4">
              <!-- Network -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Network *</span>
                </label>
                <select v-model="formData.network_id" class="select select-bordered" required>
                  <option value="">Select a network</option>
                  <option v-for="net in networks" :key="net.id" :value="net.id">
                    {{ net.name }} ({{ net.cidr_range }})
                  </option>
                </select>
              </div>

              <!-- Groups -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Groups</span>
                </label>
                <input 
                  v-model="formData.groups"
                  type="text" 
                  placeholder="server, ssh, http"
                  class="input input-bordered"
                />
                <label class="label">
                  <span class="label-text-alt">
                    Comma-separated list of groups for firewall rules
                  </span>
                </label>
              </div>

              <!-- Overlay IP (Read-onlyish) -->
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Overlay IP (Optional)</span>
                </label>
                <input 
                  v-model="formData.overlay_ip"
                  type="text" 
                  placeholder="Auto-assigned if empty"
                  class="input input-bordered font-mono"
                />
                <label class="label">
                  <span class="label-text-alt">
                    Leave empty to let the system auto-assign an IP from the network CIDR
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Role & Status">
            <div class="space-y-4">
              <!-- Is Lighthouse -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.is_lighthouse"
                    type="checkbox" 
                    class="checkbox checkbox-primary"
                  />
                  <span class="label-text">
                    <span class="font-medium">Is Lighthouse</span>
                    <span class="block text-sm text-base-content/70 mt-1">
                      This host will serve as a lighthouse for other nodes. Requires a static public IP.
                    </span>
                  </span>
                </label>
              </div>

              <!-- Public IP/Port (Conditional) -->
              <div v-if="formData.is_lighthouse" class="form-control pl-8 border-l-2 border-base-300">
                <label class="label">
                  <span class="label-text">Public Host:Port *</span>
                </label>
                <input 
                  v-model="formData.public_host_port"
                  type="text" 
                  placeholder="1.2.3.4:4242"
                  class="input input-bordered font-mono"
                  :required="formData.is_lighthouse"
                />
                <label class="label">
                  <span class="label-text-alt">
                    The publicly accessible address of this lighthouse
                  </span>
                </label>
              </div>

              <!-- Active -->
              <div class="form-control">
                <label class="label cursor-pointer justify-start gap-4">
                  <input 
                    v-model="formData.active"
                    type="checkbox" 
                    class="toggle toggle-success"
                  />
                  <span class="label-text">
                    <span class="font-medium">Active Status</span>
                    <span class="block text-sm text-base-content/70">
                      Allow this host to connect to the network
                    </span>
                  </span>
                </label>
              </div>
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
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Host</span>
        </button>
      </div>
    </form>
  </div>
</template>
