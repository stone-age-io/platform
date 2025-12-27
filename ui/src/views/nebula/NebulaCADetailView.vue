<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
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

/**
 * Check if CA is currently valid
 */
const isValid = computed(() => {
  if (!ca.value?.expires_at) return false
  return new Date(ca.value.expires_at) > new Date()
})

/**
 * Load CA details
 */
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

/**
 * Copy to clipboard
 */
async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch (err) {
    toast.error('Failed to copy')
  }
}

/**
 * Download CA Cert
 */
function downloadCert() {
  if (!ca.value?.certificate) return
  
  const blob = new Blob([ca.value.certificate], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'ca.crt'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
  
  toast.success('Certificate downloaded')
}

onMounted(() => {
  loadCA()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="ca">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nebula/cas">Nebula CAs</router-link></li>
            <li class="truncate max-w-[200px]">{{ ca.name }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ ca.name }}</h1>
            <div class="flex items-center gap-1.5 px-3 py-1 bg-base-200 rounded-full">
              <span class="w-2 h-2 rounded-full" :class="isValid ? 'bg-success' : 'bg-error'"></span>
              <span class="text-xs font-medium">{{ isValid ? 'Valid' : 'Expired' }}</span>
            </div>
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
                <dt class="text-sm font-medium text-base-content/70">Name</dt>
                <dd class="mt-1 text-sm">{{ ca.name }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Cryptography Curve</dt>
                <dd class="mt-1">
                  <code class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono">{{ ca.curve || 'P256' }}</code>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Validity Period</dt>
                <dd class="mt-1 text-sm">{{ ca.validity_years || 10 }} years</dd>
              </div>
              <div v-if="ca.expires_at">
                <dt class="text-sm font-medium text-base-content/70">Expiration Date</dt>
                <dd class="mt-1 text-sm">{{ formatDate(ca.expires_at) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(ca.created) }}</dd>
              </div>
            </dl>
          </BaseCard>
        </div>
        
        <!-- Right Column: Certificate -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-4">
                <h3 class="card-title text-base">Public Certificate</h3>
                <div class="flex gap-2">
                  <button 
                    @click="downloadCert" 
                    class="btn btn-sm btn-outline"
                  >
                    <span class="text-lg">📥</span>
                    ca.crt
                  </button>
                  <button 
                    @click="copyToClipboard(ca.certificate!, 'Certificate')" 
                    class="btn btn-sm btn-outline"
                  >
                    📋
                  </button>
                </div>
              </div>
            </template>

            <div class="space-y-4">
              <div class="alert alert-info py-2 text-sm">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                <span>Used by Nebula nodes to verify identity trust within the network.</span>
              </div>
              
              <div v-if="ca.certificate" class="bg-base-200 p-4 rounded-lg font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all border border-base-300 max-h-[500px]">
{{ ca.certificate.trim() }}
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
