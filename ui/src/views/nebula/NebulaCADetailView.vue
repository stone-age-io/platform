<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { NebulaCA } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const ca = ref<NebulaCA | null>(null)
const loading = ref(true)

const caId = route.params.id as string

async function loadCA() {
  loading.value = true
  
  try {
    ca.value = await pb.collection('nebula_ca').getOne<NebulaCA>(caId)
  } catch (err: any) {
    toast.error(err.message || 'Failed to load Nebula CA')
    router.push('/nebula/cas')
  } finally {
    loading.value = false
  }
}

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied to clipboard`)
  } catch (err) {
    toast.error('Failed to copy to clipboard')
  }
}

onMounted(() => {
  loadCA()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="ca">
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nebula/cas">Nebula CAs</router-link></li>
            <li class="truncate max-w-[200px]">{{ ca.name }}</li>
          </ul>
        </div>
        <h1 class="text-3xl font-bold break-words">{{ ca.name }}</h1>
      </div>
      
      <div class="alert alert-info">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
        <span>Nebula CAs are automatically managed by the system. Create Networks and Hosts to use this CA.</span>
      </div>
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <BaseCard title="Basic Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Name</dt>
              <dd class="mt-1 text-sm">{{ ca.name }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Curve</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ ca.curve || 'P256' }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Validity Period</dt>
              <dd class="mt-1 text-sm">{{ ca.validity_years || 10 }} years</dd>
            </div>
            <div v-if="ca.expires_at">
              <dt class="text-sm font-medium text-base-content/70">Expires</dt>
              <dd class="mt-1 text-sm">{{ formatDate(ca.expires_at) }}</dd>
            </div>
          </dl>
        </BaseCard>
        
        <BaseCard v-if="ca.certificate" title="CA Certificate" class="lg:col-span-2">
          <div class="flex items-start gap-2">
            <div class="flex-1 bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all">
              {{ ca.certificate }}
            </div>
            <button 
              @click="copyToClipboard(ca.certificate!, 'Certificate')"
              class="btn btn-ghost btn-sm"
              title="Copy to clipboard"
            >
              📋
            </button>
          </div>
        </BaseCard>
        
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ ca.id }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Created</dt>
              <dd class="mt-1 text-sm">{{ formatDate(ca.created) }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
              <dd class="mt-1 text-sm">{{ formatDate(ca.updated) }}</dd>
            </div>
          </dl>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
