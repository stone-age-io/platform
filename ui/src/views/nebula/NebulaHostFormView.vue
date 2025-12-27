<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NebulaHost, NebulaNetwork } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const hostId = route.params.id as string | undefined
const isEdit = computed(() => !!hostId)

// Form data
const formData = ref({
  hostname: '',
  network_id: '',
  groups: '', // Managed as comma-separated string for input
  is_lighthouse: false,
  public_host_port: '',
  overlay_ip: '', // Read-only mostly
  active: true,
  regenerate: false, // For edit mode only
})

// Relation options
const networks = ref<NebulaNetwork[]>([])

// State
const loading = ref(false)
const loadingOptions = ref(true)

/**
 * Load available networks
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
 * Load existing host for editing
 */
async function loadHost() {
  if (!hostId) return
  
  loading.value = true
  
  try {
    const host = await pb.collection('nebula_hosts').getOne<NebulaHost>(hostId)
    
    // Convert groups array to comma-separated string
    const groupsStr = Array.isArray(host.groups) ? host.groups.join(', ') : ''
    
    formData.value = {
      hostname: host.hostname,
      network_id: host.network_id,
      groups: groupsStr,
      is_lighthouse: host.is_lighthouse || false,
      public_host_port: host.public_host_port || '',
      overlay_ip: host.overlay_ip || '',
      active: host.active ?? true, // Default to true if undefined
      regenerate: false,
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
  loading.value = true
  
  try {
    // Process groups string into array
    const groupsArray = formData.value.groups
      .split(',')
      .map(g => g.trim())
      .filter(g => g.length > 0)

    const data: any = {
      hostname: formData.value.hostname,
      network_id: formData.value.network_id,
      groups: groupsArray,
      is_lighthouse: formData.value.is_lighthouse,
      public_host_port: formData.value.public_host_port || null,
      active: formData.value.active,
    }
    
    // Only send overlay_ip if explicitly set (usually backend handles this)
    if (formData.value.overlay_ip) {
      data.overlay_ip = formData.value.overlay_ip
    }
    
    if (isEdit.value) {
      // Update existing host
      data.regenerate = formData.value.regenerate
      await pb.collection('nebula_hosts').update(hostId!, data)
      toast.success('Nebula host updated')
    } else {
      // Create new host
      data.organization = authStore.currentOrgId
      
      await pb.collection('nebula_hosts').create(data)
      toast.success('Nebula host created')
    }
    
    router.push('/nebula/hosts')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save Nebula host')
  } finally {
    loading.value = false
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
    <!-- Header -->
    <div>
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
      <BaseCard title="Identity">
        <div class="space-y-4">
          <!-- Hostname -->
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
        </div>
      </BaseCard>
      
      <!-- Network Configuration -->
      <BaseCard title="Network Configuration">
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
                  This host will serve as a lighthouse for other nodes to find each other.
                  Requires a static public IP/Port.
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
              placeholder="1.2.3.4:4242 or my.domain.com:4242"
              class="input input-bordered font-mono"
              :required="formData.is_lighthouse"
            />
            <label class="label">
              <span class="label-text-alt">
                The publicly accessible address of this lighthouse
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
      
      <!-- Options -->
      <BaseCard title="Lifecycle">
        <div class="space-y-4">
          <!-- Active -->
          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input 
                v-model="formData.active"
                type="checkbox" 
                class="checkbox"
              />
              <span class="label-text">
                <span class="font-medium">Active</span>
                <span class="block text-sm text-base-content/70 mt-1">
                  Disable to revoke access without deleting the record
                </span>
              </span>
            </label>
          </div>
          
          <!-- Regenerate (edit only) -->
          <div v-if="isEdit" class="form-control">
            <label class="label cursor-pointer justify-start gap-4">
              <input 
                v-model="formData.regenerate"
                type="checkbox" 
                class="checkbox checkbox-warning"
              />
              <span class="label-text">
                <span class="font-medium text-warning">Regenerate Certificate</span>
                <span class="block text-sm text-base-content/70 mt-1">
                  Create a new certificate/key pair. You will need to re-download the config.
                </span>
              </span>
            </label>
          </div>
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
          <span v-else>{{ isEdit ? 'Update' : 'Provision' }} Host</span>
        </button>
      </div>
    </form>
  </div>
</template>
