<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useNatsStore } from '@/stores/nats'
import { formatDate } from '@/utils/format'
import type { Edge, NatsUser, NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import KvDashboard from '@/components/nats/KvDashboard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const natsStore = useNatsStore()

const edge = ref<Edge | null>(null)
const loading = ref(true)
const regenerating = ref(false)
const showRegenerateModal = ref(false)

const edgeId = route.params.id as string

/**
 * JSON Highlighter for Metadata
 */
const highlightedMetadata = computed(() => {
  if (!edge.value?.metadata) return '{}'
  const json = JSON.stringify(edge.value.metadata, null, 2)
  return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, (match) => {
    let cls = 'text-warning'
    if (/^"/.test(match)) {
      if (/:$/.test(match)) {
        cls = 'text-primary font-bold'
      } else {
        cls = 'text-secondary'
      }
    } else if (/true|false/.test(match)) {
      cls = 'text-info'
    } else if (/null/.test(match)) {
      cls = 'text-error'
    }
    return `<span class="${cls}">${match}</span>`
  })
})

async function loadEdge() {
  loading.value = true
  try {
    edge.value = await pb.collection('edges').getOne<Edge>(edgeId, {
      expand: 'type,nats_user,nebula_host',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load edge')
    router.push('/edges')
  } finally {
    loading.value = false
  }
}

/**
 * Infrastructure File Handlers
 */
function downloadFile(filename: string, content: string, contentType: string) {
  if (!content) return toast.error('No content available')
  const blob = new Blob([content], { type: contentType })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function downloadNatsCreds() {
  const natsUser = edge.value?.expand?.nats_user as NatsUser
  if (!natsUser?.creds_file) return toast.error('No credentials file found')
  downloadFile(`${natsUser.nats_username}.creds`, natsUser.creds_file, 'text/plain')
  toast.success('Credentials downloaded')
}

async function confirmRegenerate() {
  const natsUser = edge.value?.expand?.nats_user as NatsUser
  if (!natsUser) return

  regenerating.value = true
  try {
    await pb.collection('nats_users').update(natsUser.id, { regenerate: true })
    toast.success('Credentials regenerated')
    showRegenerateModal.value = false
    await loadEdge()
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate credentials')
  } finally {
    regenerating.value = false
  }
}

function downloadNebulaConfig() {
  const host = edge.value?.expand?.nebula_host as NebulaHost
  if (!host?.config_yaml) return toast.error('No configuration available')
  downloadFile(`${host.hostname}.yaml`, host.config_yaml, 'text/yaml')
  toast.success('Nebula config downloaded')
}

async function handleDelete() {
  if (!edge.value) return
  if (!confirm(`Delete "${edge.value.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('edges').delete(edge.value.id)
    toast.success('Edge deleted')
    router.push('/edges')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete edge')
  }
}

onMounted(() => {
  loadEdge()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="edge">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/edges">Edges</router-link></li>
            <li class="truncate max-w-[200px]">{{ edge.name || 'Unnamed' }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ edge.name || 'Unnamed Edge' }}</h1>
            <span v-if="edge.expand?.type" class="badge badge-lg badge-ghost">{{ edge.expand.type.name }}</span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/edges/${edge.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">
              Delete
            </button>
          </div>
        </div>
      </div>
      
      <!-- Metadata & Infra Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column: Identity & Metadata -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ edge.description || '-' }}</dd>
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Code / ID</dt>
                  <dd class="mt-1">
                    <code v-if="edge.code" class="text-sm bg-base-200 px-1 py-0.5 rounded font-mono">{{ edge.code }}</code>
                    <span v-else class="text-sm">-</span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Created</dt>
                  <dd class="mt-1 text-sm">{{ formatDate(edge.created) }}</dd>
                </div>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Metadata" v-if="edge.metadata && Object.keys(edge.metadata).length > 0">
            <div class="bg-base-200 rounded-lg p-4 overflow-x-auto border border-base-300">
              <pre class="text-sm font-mono leading-relaxed" v-html="highlightedMetadata"></pre>
            </div>
          </BaseCard>
        </div>
        
        <!-- Right Column: Infrastructure -->
        <div class="space-y-6">
          <!-- NATS Connectivity -->
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">NATS Connectivity</h3>
                <div class="flex gap-2" v-if="edge.expand?.nats_user">
                  <button @click="downloadNatsCreds" class="btn btn-sm btn-outline h-8 min-h-0" title="Download .creds">
                    <span class="text-lg">📥</span>
                    <span class="hidden sm:inline">.creds</span>
                  </button>
                  <button @click="showRegenerateModal = true" class="btn btn-sm btn-outline btn-error h-8 min-h-0" title="Regenerate">
                    🔄
                  </button>
                </div>
              </div>
            </template>

            <div v-if="edge.expand?.nats_user" class="flex flex-col gap-4">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <div class="flex justify-between items-start mb-1">
                  <span class="text-[10px] font-bold text-base-content/50 uppercase tracking-wider">NATS Username</span>
                  <div class="badge badge-xs" :class="edge.expand.nats_user.active ? 'badge-success' : 'badge-error'">
                    {{ edge.expand.nats_user.active ? 'Active' : 'Inactive' }}
                  </div>
                </div>
                <div class="font-mono text-sm break-all select-all">{{ edge.expand.nats_user.nats_username }}</div>
              </div>
            </div>
            <div v-else class="text-center py-8 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <p class="text-xs">No NATS user linked to this edge</p>
            </div>
          </BaseCard>
          
          <!-- Nebula Connectivity -->
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Nebula Connectivity</h3>
                <div v-if="edge.expand?.nebula_host">
                  <button @click="downloadNebulaConfig" class="btn btn-sm btn-outline h-8 min-h-0">
                    📥 Config
                  </button>
                </div>
              </div>
            </template>

            <div v-if="edge.expand?.nebula_host" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-[10px] font-bold text-base-content/50 uppercase block mb-1">Hostname</span>
                <div class="font-mono text-xs">{{ edge.expand.nebula_host.hostname }}</div>
              </div>
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <span class="text-[10px] font-bold text-base-content/50 uppercase block mb-1">Overlay IP</span>
                <div class="font-mono text-xs">{{ edge.expand.nebula_host.overlay_ip }}</div>
              </div>
            </div>
            <div v-else class="text-center py-8 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <p class="text-xs">No Nebula host linked to this edge</p>
            </div>
          </BaseCard>
        </div>
      </div>

      <!-- Bottom Section: Digital Twin (NATS KV) -->
      <div v-if="edge.code && natsStore.isConnected" class="mt-6">
        <KvDashboard 
          :key="edge.code"
          :base-key="`edge.${edge.code}`" 
        />
      </div>

      <!-- Digital Twin Warning States -->
      <div v-else class="mt-6">
        <div v-if="edge.code && !natsStore.isConnected" class="alert shadow-sm border border-base-300 bg-base-100">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          <div class="text-xs">
            <div class="font-bold">Digital Twin Offline</div> 
            <span class="opacity-70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live Edge state.</span>
          </div>
        </div>
        <div v-else-if="!edge.code" class="alert shadow-sm border border-base-300 bg-base-200/50">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-base-content/30 shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          <div class="text-xs opacity-60">
            <div class="font-bold">Digital Twin Unavailable</div> 
            <span>To enable the Twin, provide a <b>Code</b> for this Edge.</span>
          </div>
        </div>
      </div>

    </template>

    <!-- Regenerate Modal -->
    <dialog class="modal" :class="{ 'modal-open': showRegenerateModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Regenerate Credentials?</h3>
        <p class="py-4 text-sm opacity-80">
          This will invalidate the existing <code>.creds</code> file immediately. The edge gateway will lose connectivity until you update it with the new file.
        </p>
        <div class="modal-action">
          <button class="btn btn-sm" @click="showRegenerateModal = false" :disabled="regenerating">Cancel</button>
          <button class="btn btn-sm btn-error" @click="confirmRegenerate" :disabled="regenerating">
            <span v-if="regenerating" class="loading loading-spinner loading-xs"></span>
            Regenerate
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop"><button @click="showRegenerateModal = false">close</button></form>
    </dialog>
  </div>
</template>
