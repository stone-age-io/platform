<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { NebulaNetwork, NebulaHost } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const network = ref<NebulaNetwork | null>(null)
const hosts = ref<NebulaHost[]>([])
const loading = ref(true)

const networkId = route.params.id as string

// Columns for Attached Hosts List
const hostColumns: Column<NebulaHost>[] = [
  { key: 'hostname', label: 'Hostname', mobileLabel: 'Host' },
  { key: 'overlay_ip', label: 'Overlay IP', mobileLabel: 'IP' },
  { 
    key: 'is_lighthouse', 
    label: 'Type', 
    mobileLabel: 'Type',
    format: (val) => val ? 'Lighthouse' : 'Node' 
  },
  { 
    key: 'active', 
    label: 'Status', 
    mobileLabel: 'Status' 
  },
]

async function loadNetwork() {
  loading.value = true
  try {
    // 1. Get Network
    network.value = await pb.collection('nebula_networks').getOne<NebulaNetwork>(networkId, {
      expand: 'ca_id',
    })

    // 2. Get Attached Hosts
    hosts.value = await pb.collection('nebula_hosts').getFullList<NebulaHost>({
      filter: `network_id = "${networkId}"`,
      sort: 'overlay_ip',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load network')
    router.push('/nebula/networks')
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  if (!network.value) return
  if (!confirm(`Delete network "${network.value.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nebula_networks').delete(network.value.id)
    toast.success('Network deleted')
    router.push('/nebula/networks')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete network')
  }
}

onMounted(() => {
  loadNetwork()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="network">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nebula/networks">Nebula Networks</router-link></li>
            <li class="truncate max-w-[200px]">{{ network.name }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ network.name }}</h1>
            <!-- Status Dot -->
            <div class="flex items-center gap-1.5 px-3 py-1 bg-base-200 rounded-full">
              <span class="w-2 h-2 rounded-full" :class="network.active ? 'bg-success' : 'bg-error'"></span>
              <span class="text-xs font-medium">{{ network.active ? 'Active' : 'Inactive' }}</span>
            </div>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nebula/networks/${network.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">
              Delete
            </button>
          </div>
        </div>
      </div>
      
      <!-- Content Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Info -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ network.description || '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">CIDR Range</dt>
                <dd class="mt-1">
                  <code class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono">{{ network.cidr_range }}</code>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Certificate Authority</dt>
                <dd class="mt-1">
                  <router-link 
                    v-if="network.expand?.ca_id" 
                    :to="`/nebula/cas/${network.ca_id}`"
                    class="link link-primary hover:no-underline flex items-center gap-1"
                  >
                    🔐 {{ network.expand.ca_id.name }}
                  </router-link>
                  <span v-else class="text-sm text-base-content/40">Unknown CA</span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(network.created) }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>
        
        <!-- Right Column: Stats placeholder / Instructions -->
        <div class="space-y-6">
          <div class="alert alert-info shadow-sm">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
            <div>
              <h3 class="font-bold text-sm">Network Overlay</h3>
              <div class="text-xs">
                Hosts added to this network will automatically be assigned an IP from the 
                <span class="font-mono">{{ network.cidr_range }}</span> block.
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Bottom: Attached Hosts -->
      <BaseCard title="Attached Hosts" :no-padding="true">
        <ResponsiveList
          :items="hosts"
          :columns="hostColumns"
          :clickable="true"
          @row-click="(item) => router.push(`/nebula/hosts/${item.id}`)"
        >
          <template #cell-hostname="{ item }">
            <span class="font-mono font-medium">{{ item.hostname }}</span>
          </template>
          
          <template #cell-active="{ item }">
            <div class="flex items-center gap-1.5" v-if="item.active">
              <span class="w-2 h-2 rounded-full bg-success"></span>
              <span>Active</span>
            </div>
            <div class="flex items-center gap-1.5" v-else>
              <span class="w-2 h-2 rounded-full bg-error"></span>
              <span>Inactive</span>
            </div>
          </template>

          <template #empty>
            <div class="text-center py-8 text-base-content/50">
              No hosts have been provisioned on this network yet.
              <div class="mt-2">
                <router-link to="/nebula/hosts/new" class="btn btn-primary btn-sm">Provision Host</router-link>
              </div>
            </div>
          </template>
        </ResponsiveList>
      </BaseCard>

    </template>
  </div>
</template>
