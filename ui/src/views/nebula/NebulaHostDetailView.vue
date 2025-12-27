<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const host = ref<NebulaHost | null>(null)
const loading = ref(true)

const hostId = route.params.id as string

/**
 * Load host details
 */
async function loadHost() {
  loading.value = true
  
  try {
    host.value = await pb.collection('nebula_hosts').getOne<NebulaHost>(hostId, {
      expand: 'network_id',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load Nebula host')
    router.push('/nebula/hosts')
  } finally {
    loading.value = false
  }
}

/**
 * Handle delete
 */
async function handleDelete() {
  if (!host.value) return
  if (!confirm(`Delete Nebula host "${host.value.hostname}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nebula_hosts').delete(host.value.id)
    toast.success('Nebula host deleted')
    router.push('/nebula/hosts')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete Nebula host')
  }
}

/**
 * Copy to clipboard
 */
async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied to clipboard`)
  } catch (err) {
    toast.error('Failed to copy to clipboard')
  }
}

/**
 * Download file
 */
function downloadFile(content: string, filename: string, mimeType: string = 'text/plain') {
  const blob = new Blob([content], { type: mimeType })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  
  toast.success(`${filename} downloaded`)
}

/**
 * Download config YAML
 */
function downloadConfig() {
  if (!host.value?.config_yaml) return
  downloadFile(host.value.config_yaml, `${host.value.hostname}.yml`, 'text/yaml')
}

/**
 * Download certificate
 */
function downloadCertificate() {
  if (!host.value?.certificate) return
  downloadFile(host.value.certificate, `${host.value.hostname}.crt`, 'text/plain')
}

/**
 * Download CA certificate
 */
function downloadCACertificate() {
  if (!host.value?.ca_certificate) return
  downloadFile(host.value.ca_certificate, 'ca.crt', 'text/plain')
}

onMounted(() => {
  loadHost()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="host">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nebula/hosts">Nebula Hosts</router-link></li>
            <li class="truncate max-w-[200px] font-mono">{{ host.hostname }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold font-mono break-words">{{ host.hostname }}</h1>
            <span 
              class="badge"
              :class="host.is_lighthouse ? 'badge-primary' : 'badge-ghost'"
            >
              {{ host.is_lighthouse ? 'Lighthouse' : 'Node' }}
            </span>
            <span 
              class="badge"
              :class="host.active ? 'badge-success' : 'badge-error'"
            >
              {{ host.active ? 'Active' : 'Inactive' }}
            </span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nebula/hosts/${host.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">
              Delete
            </button>
          </div>
        </div>
      </div>
      
      <!-- Details Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Basic Info -->
        <BaseCard title="Basic Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Hostname</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ host.hostname }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Overlay IP</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ host.overlay_ip }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Network</dt>
              <dd class="mt-1">
                <router-link 
                  v-if="host.expand?.network_id"
                  :to="`/nebula/networks/${host.network_id}`"
                  class="link link-hover"
                >
                  {{ host.expand.network_id.name }}
                </router-link>
                <span v-else>-</span>
              </dd>
            </div>
            <div v-if="host.groups && host.groups.length > 0">
              <dt class="text-sm font-medium text-base-content/70">Groups</dt>
              <dd class="mt-1 flex flex-wrap gap-2">
                <span 
                  v-for="group in host.groups" 
                  :key="group"
                  class="badge badge-outline badge-sm"
                >
                  {{ group }}
                </span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Lighthouse Config -->
        <BaseCard title="Network Configuration">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Type</dt>
              <dd class="mt-1">
                <span 
                  class="badge"
                  :class="host.is_lighthouse ? 'badge-primary' : 'badge-ghost'"
                >
                  {{ host.is_lighthouse ? 'Lighthouse' : 'Node' }}
                </span>
              </dd>
            </div>
            <div v-if="host.public_host_port">
              <dt class="text-sm font-medium text-base-content/70">Public Host:Port</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ host.public_host_port }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Status</dt>
              <dd class="mt-1">
                <span 
                  class="badge"
                  :class="host.active ? 'badge-success' : 'badge-error'"
                >
                  {{ host.active ? 'Active' : 'Inactive' }}
                </span>
              </dd>
            </div>
            <div v-if="host.validity_years">
              <dt class="text-sm font-medium text-base-content/70">Certificate Validity</dt>
              <dd class="mt-1 text-sm">{{ host.validity_years }} years</dd>
            </div>
            <div v-if="host.expires_at">
              <dt class="text-sm font-medium text-base-content/70">Expires</dt>
              <dd class="mt-1 text-sm">{{ formatDate(host.expires_at) }}</dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Configuration File -->
        <BaseCard v-if="host.config_yaml" title="Configuration File" class="lg:col-span-2">
          <div class="flex items-center justify-between gap-4 mb-4">
            <div>
              <p class="text-sm">
                Download the Nebula configuration file for this host
              </p>
              <p class="text-xs text-base-content/60 mt-1">
                File: <code>{{ host.hostname }}.yml</code>
              </p>
            </div>
            <button 
              @click="downloadConfig"
              class="btn btn-primary btn-sm"
            >
              📥 Download Config
            </button>
          </div>
          <div class="bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto max-h-64 whitespace-pre">{{ host.config_yaml }}</div>
        </BaseCard>
        
        <!-- Host Certificate -->
        <BaseCard v-if="host.certificate" title="Host Certificate" class="lg:col-span-2">
          <div class="flex items-start gap-2 mb-2">
            <button 
              @click="downloadCertificate"
              class="btn btn-ghost btn-sm"
            >
              📥 Download
            </button>
            <button 
              @click="copyToClipboard(host.certificate!, 'Certificate')"
              class="btn btn-ghost btn-sm"
              title="Copy to clipboard"
            >
              📋 Copy
            </button>
          </div>
          <div class="bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all max-h-48">
            {{ host.certificate }}
          </div>
        </BaseCard>
        
        <!-- CA Certificate -->
        <BaseCard v-if="host.ca_certificate" title="CA Certificate" class="lg:col-span-2">
          <div class="flex items-start gap-2 mb-2">
            <button 
              @click="downloadCACertificate"
              class="btn btn-ghost btn-sm"
            >
              📥 Download
            </button>
            <button 
              @click="copyToClipboard(host.ca_certificate!, 'CA Certificate')"
              class="btn btn-ghost btn-sm"
              title="Copy to clipboard"
            >
              📋 Copy
            </button>
          </div>
          <div class="bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all max-h-48">
            {{ host.ca_certificate }}
          </div>
        </BaseCard>
        
        <!-- Firewall Outbound -->
        <BaseCard v-if="host.firewall_outbound" title="Firewall - Outbound Rules">
          <pre class="text-xs bg-base-200 p-4 rounded overflow-x-auto">{{ JSON.stringify(host.firewall_outbound, null, 2) }}</pre>
        </BaseCard>
        
        <!-- Firewall Inbound -->
        <BaseCard v-if="host.firewall_inbound" title="Firewall - Inbound Rules">
          <pre class="text-xs bg-base-200 p-4 rounded overflow-x-auto">{{ JSON.stringify(host.firewall_inbound, null, 2) }}</pre>
        </BaseCard>
        
        <!-- System Info -->
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ host.id }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Created</dt>
              <dd class="mt-1 text-sm">{{ formatDate(host.created) }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
              <dd class="mt-1 text-sm">{{ formatDate(host.updated) }}</dd>
            </div>
          </dl>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
