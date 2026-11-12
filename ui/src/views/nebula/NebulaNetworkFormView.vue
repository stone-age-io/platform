<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import type { NebulaNetwork, NebulaCA } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import RecordPicker from '@/components/common/RecordPicker.vue'
import type { PickerOption } from '@/types/picker'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const networkId = route.params.id as string | undefined
const isEdit = computed(() => !!networkId)

const formData = ref({
  name: '',
  description: '',
  cidr_range: '',
  ca_id: '',
  active: true,
})

const cas = ref<NebulaCA[]>([])

const caOptions = computed<PickerOption[]>(() =>
  cas.value.map(ca => ({ id: ca.id, label: ca.name }))
)

const loading = ref(false)
const loadingOptions = ref(true)

async function loadOptions() {
  loadingOptions.value = true
  
  try {
    const result = await pb.collection('nebula_ca').getFullList<NebulaCA>({ 
      sort: 'name',
      // No expired CAs for a new network
      filter: isEdit.value ? '' : `expires_at > "${new Date().toISOString()}"`
    })
    
    cas.value = result
    
    if (!isEdit.value && cas.value.length === 1) {
      formData.value.ca_id = cas.value[0].id
    }
  } catch (err: any) {
    toast.error('Failed to load Certificate Authorities')
  } finally {
    loadingOptions.value = false
  }
}

async function loadNetwork() {
  if (!networkId) return
  
  loading.value = true
  
  try {
    const network = await pb.collection('nebula_networks').getOne<NebulaNetwork>(networkId)
    
    formData.value = {
      name: network.name,
      description: network.description || '',
      cidr_range: network.cidr_range,
      ca_id: network.ca_id,
      active: network.active ?? true,
    }
  } catch (err: any) {
    toast.error('Failed to load network')
    router.push('/nebula/networks')
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  loading.value = true
  
  try {
    const data: any = {
      name: formData.value.name,
      description: formData.value.description || null,
      ca_id: formData.value.ca_id,
      active: formData.value.active,
    }
    
    if (isEdit.value) {
      // cidr_range is frozen after create (nebula_networks.updateRule), so it is not sent.
      await pb.collection('nebula_networks').update(networkId!, data)
      toast.success('Network updated')
    } else {
      data.organization = authStore.currentOrgId
      data.cidr_range = formData.value.cidr_range
      await pb.collection('nebula_networks').create(data)
      toast.success('Network created')
    }
    
    router.push('/nebula/networks')
  } catch (err: any) {
    toast.error(err.message || 'Failed to save network')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadOptions()
  if (isEdit.value) {
    loadNetwork()
  }
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <div class="breadcrumbs text-sm">
        <ul>
          <li><router-link to="/nebula/networks">Nebula Networks</router-link></li>
          <li>{{ isEdit ? 'Edit' : 'New' }}</li>
        </ul>
      </div>
      <h1 class="text-3xl font-bold">
        {{ isEdit ? 'Edit Network' : 'Create Network' }}
      </h1>
    </div>
    
    <div v-if="loadingOptions" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <form v-else @submit.prevent="handleSubmit" class="space-y-6">
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <div class="space-y-6">
          <BaseCard title="Identity">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Network Name *</span>
                </label>
                <input 
                  v-model="formData.name"
                  type="text" 
                  placeholder="e.g. Production Overlay"
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
                  rows="3"
                  placeholder="Optional description"
                ></textarea>
              </div>
            </div>
          </BaseCard>

          <BaseCard title="Status">
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
                    Enable or disable this network overlay
                  </span>
                </span>
              </label>
            </div>
          </BaseCard>
        </div>

        <div class="space-y-6">
          <BaseCard title="Configuration">
            <div class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">CIDR Range *</span>
                </label>
                <input 
                  v-model="formData.cidr_range"
                  type="text" 
                  placeholder="e.g. 10.100.0.0/24"
                  class="input input-bordered font-mono"
                  :disabled="isEdit"
                  required
                />
                <label class="label">
                  <span class="label-text-alt">
                    <template v-if="isEdit">
                      Fixed once the network exists. Host certificates carry it, and a change
                      would leave every host unable to reach its peers. Create a new network instead.
                    </template>
                    <template v-else>
                      Defines the IP pool for this network overlay. It cannot be changed later.
                    </template>
                  </span>
                </label>
              </div>
              
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Certificate Authority *</span>
                </label>
                <RecordPicker
                  v-model="formData.ca_id"
                  :options="caOptions"
                  title="Certificate authority"
                  placeholder="Select a CA"
                  empty-text="No certificate authorities yet."
                />
                <label class="label">
                  <span class="label-text-alt">
                    The CA used to sign certificates for this network.
                  </span>
                </label>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
      
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
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Network</span>
        </button>
      </div>
    </form>
  </div>
</template>
