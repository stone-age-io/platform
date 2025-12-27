<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
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
const regenerating = ref(false)
const showRegenerateModal = ref(false)

const hostId = route.params.id as string

/**
 * JSON Highlighter Helper
 */
function highlightJson(data: any) {
  if (!data) return '{}'
  const json = JSON.stringify(data, null, 2)
  return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, (match) => {
    let cls = 'text-warning'
    if (/^"/.test(match)) {
      if (/:$/.test(match)) { cls = 'text-primary font-bold' } 
      else { cls = 'text-secondary' }
    } else if (/true|false/.test(match)) { cls = 'text-info' } 
    else if (/null/.test(match)) { cls = 'text-error' }
    return `<span class="${cls}">${match}</span>`
  })
}

const highlightedOutbound = computed(() => highlightJson(host.value?.firewall_outbound))
const highlightedInbound = computed(() => highlightJson(host.value?.firewall_inbound))

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

async function confirmRegenerate() {
  if (!host.value) return

  regenerating.value = true
  try {
    await pb.collection('nebula_hosts').update(host.value.id, { regenerate: true })
    toast.success('Certificate regenerated')
    showRegenerateModal.value = false
    await loadHost()
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate certificate')
  } finally {
    regenerating.value = false
  }
}

function downloadFile(filename: string, content: string, contentType: string) {
  const blob = new Blob([content], { type: contentType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
  toast.success(`${filename} downloaded`)
}

function copyToClipboard(text: string, label: string) {
  navigator.clipboard.writeText(text)
  toast.success(`${label} copied`)
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
            <span v-if="host.is_lighthouse" class="badge badge-primary badge-outline gap-1">
              🚨 Lighthouse
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
      
      <!-- Content Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Identity & Network -->
        <div class="space-y-6">
          <BaseCard title="Identity & Network">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Email (Identity)</dt>
                <dd class="mt-1 text-sm">{{ host.email }}</dd>
              </div>
              
              <div>
                <dt class="text-sm font-medium text-base-content/70">Network</dt>
                <dd class="mt-1">
                  <router-link 
                    v-if="host.expand?.network_id"
                    :to="`/nebula/networks/${host.network_id}`"
                    class="link link-primary hover:no-underline"
                  >
                    🌐 {{ host.expand.network_id.name }}
                  </router-link>
                  <span v-else class="text-sm text-base-content/40">-</span>
                </dd>
              </div>

              <div>
                <dt class="text-sm font-medium text-base-content/70">Overlay IP</dt>
                <dd class="mt-1">
                  <code class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono">{{ host.overlay_ip }}</code>
                </dd>
              </div>

              <div v-if="host.public_host_port">
                <dt class="text-sm font-medium text-base-content/70">Public Endpoint</dt>
                <dd class="mt-1 font-mono text-sm">{{ host.public_host_port }}</dd>
              </div>

              <div v-if="host.groups && host.groups.length > 0">
                <dt class="text-sm font-medium text-base-content/70 mb-1">Groups</dt>
                <dd class="flex flex-wrap gap-2">
                  <span 
                    v-for="group in host.groups" 
                    :key="group"
                    class="badge badge-ghost border-base-300"
                  >
                    {{ group }}
                  </span>
                </dd>
              </div>
            </dl>
          </BaseCard>

          <!-- Firewall Rules (Stacked in Left Column) -->
          <BaseCard title="Firewall Rules">
            <div class="space-y-4">
              <div>
                <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-2">Inbound</div>
                <div class="bg-base-200 rounded-lg p-3 overflow-x-auto border border-base-300 max-h-60">
                  <pre class="text-xs font-mono" v-html="highlightedInbound"></pre>
                </div>
              </div>
              <div>
                <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-2">Outbound</div>
                <div class="bg-base-200 rounded-lg p-3 overflow-x-auto border border-base-300 max-h-60">
                  <pre class="text-xs font-mono" v-html="highlightedOutbound"></pre>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
        
        <!-- Right Column: Config & Security -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-4">
                <h3 class="card-title text-base">Configuration</h3>
                <div class="flex gap-2">
                  <button 
                    v-if="host.config_yaml"
                    @click="downloadFile(`${host.hostname}.yml`, host.config_yaml, 'text/yaml')"
                    class="btn btn-sm btn-outline"
                  >
                    <span class="text-lg">📥</span>
                    Config
                  </button>
                  <button 
                    @click="showRegenerateModal = true" 
                    class="btn btn-sm btn-outline btn-error"
                    title="Regenerate Certificate"
                  >
                    <span class="text-lg">🔄</span>
                  </button>
                </div>
              </div>
            </template>

            <div class="space-y-4">
              <!-- Status Indicators -->
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-xs text-base-content/50 uppercase block mb-1">Status</span>
                <div class="flex items-center gap-1.5" v-if="host.active">
                  <span class="w-2 h-2 rounded-full bg-success"></span>
                  <span class="font-medium text-sm">Active</span>
                </div>
                <div class="flex items-center gap-1.5" v-else>
                  <span class="w-2 h-2 rounded-full bg-error"></span>
                  <span class="font-medium text-sm">Inactive</span>
                </div>
              </div>

              <!-- Cert Expiry -->
              <div v-if="host.expires_at">
                <dt class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-1">Certificate Expires</dt>
                <dd class="text-sm">{{ formatDate(host.expires_at) }}</dd>
              </div>

              <!-- Config Preview -->
              <div v-if="host.config_yaml">
                <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-1">Config Preview</div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs overflow-x-auto whitespace-pre max-h-48 border border-base-300">
{{ host.config_yaml }}
                </div>
              </div>

              <!-- Host Certificate -->
              <div v-if="host.certificate">
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Host Certificate</div>
                  <div class="flex gap-1">
                    <button @click="copyToClipboard(host.certificate, 'Certificate')" class="btn btn-ghost btn-xs">Copy</button>
                    <button @click="downloadFile(`${host.hostname}.crt`, host.certificate, 'text/plain')" class="btn btn-ghost btn-xs">Download</button>
                  </div>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all max-h-32 overflow-y-auto border border-base-300">
{{ host.certificate }}
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>

    <!-- Regenerate Modal -->
    <dialog class="modal" :class="{ 'modal-open': showRegenerateModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Regenerate Certificate?</h3>
        <p class="py-4">
          This will invalidate the current certificate and generate a new key pair. You will need to 
          download the new configuration file and redeploy it to the host.
        </p>
        <div class="modal-action">
          <button 
            class="btn" 
            @click="showRegenerateModal = false"
            :disabled="regenerating"
          >
            Cancel
          </button>
          <button 
            class="btn btn-error" 
            @click="confirmRegenerate"
            :disabled="regenerating"
          >
            <span v-if="regenerating" class="loading loading-spinner"></span>
            Regenerate
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showRegenerateModal = false">close</button>
      </form>
    </dialog>
  </div>
</template>
