<!-- ui/src/views/things/ThingDetailView.vue -->
<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useNatsStore } from '@/stores/nats'
import { formatDate } from '@/utils/format'
import type { Thing, NatsUser, NebulaHost, Location } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import KvDashboard from '@/components/nats/KvDashboard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const natsStore = useNatsStore()

const thing = ref<Thing | null>(null)
const loading = ref(true)
const regenerating = ref(false)
const showRegenerateModal = ref(false)

const thingId = route.params.id as string

/**
 * JSON Highlighter
 */
const highlightedMetadata = computed(() => {
  if (!thing.value?.metadata) return '{}'
  const json = JSON.stringify(thing.value.metadata, null, 2)
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

/**
 * Grug utility: Copy raw JSON to clipboard
 */
async function copyMetadata() {
  if (!thing.value?.metadata) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(thing.value.metadata, null, 2))
    toast.success('Metadata copied to clipboard')
  } catch (err) {
    toast.error('Failed to copy')
  }
}

/**
 * Computed helpers for Digital Twin logic
 */
const locationCode = computed(() => (thing.value?.expand?.location as Location)?.code)
const thingCode = computed(() => thing.value?.code)
const hasDigitalTwinConfig = computed(() => !!locationCode.value && !!thingCode.value)

async function loadThing() {
  loading.value = true
  try {
    thing.value = await pb.collection('things').getOne<Thing>(thingId, {
      expand: 'type,location,nats_user,nebula_host',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load thing')
    router.push('/things')
  } finally {
    loading.value = false
  }
}

function downloadFile(filename: string, content: string, contentType: string) {
  if (!content) {
    toast.error('No content to download')
    return
  }
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
  const natsUser = thing.value?.expand?.nats_user as NatsUser
  if (!natsUser?.creds_file) {
    toast.error('No credentials file available')
    return
  }
  downloadFile(`${natsUser.nats_username}.creds`, natsUser.creds_file, 'text/plain')
  toast.success('Credentials downloaded')
}

async function confirmRegenerate() {
  const natsUser = thing.value?.expand?.nats_user as NatsUser
  if (!natsUser) return

  regenerating.value = true
  try {
    await pb.collection('nats_users').update(natsUser.id, { regenerate: true })
    toast.success('Credentials regenerated')
    showRegenerateModal.value = false
    await loadThing()
  } catch (err: any) {
    toast.error(err.message || 'Failed to regenerate credentials')
  } finally {
    regenerating.value = false
  }
}

function downloadNebulaConfig() {
  const host = thing.value?.expand?.nebula_host as NebulaHost
  if (!host?.config_yaml) {
    toast.error('No configuration available')
    return
  }
  downloadFile(`${host.hostname}.yaml`, host.config_yaml, 'text/yaml')
  toast.success('Config downloaded')
}

async function handleDelete() {
  if (!thing.value) return
  if (!confirm(`Delete "${thing.value.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('things').delete(thing.value.id)
    toast.success('Thing deleted')
    router.push('/things')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete thing')
  }
}

onMounted(() => {
  loadThing()
})
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="thing">
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/things">Things</router-link></li>
            <li class="truncate max-w-[200px]">{{ thing.name || 'Unnamed' }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ thing.name || 'Unnamed Thing' }}</h1>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/things/${thing.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">
              Delete
            </button>
          </div>
        </div>
      </div>
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ thing.description || '-' }}</dd>
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Type</dt>
                  <dd class="mt-1">
                    <span v-if="thing.expand?.type" class="badge badge-neutral">{{ thing.expand.type.name }}</span>
                    <span v-else class="text-sm text-base-content/40">-</span>
                  </dd>
                </div>
                <div>
                  <dt class="text-sm font-medium text-base-content/70">Code</dt>
                  <dd class="mt-1">
                    <code v-if="thing.code" class="text-sm bg-base-200 px-1 py-0.5 rounded font-mono">{{ thing.code }}</code>
                    <span v-else class="text-sm">-</span>
                  </dd>
                </div>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Location</dt>
                <dd class="mt-1">
                  <router-link v-if="thing.expand?.location" :to="`/locations/${thing.location}`" class="link link-primary hover:no-underline flex items-center gap-1">
                    📍 {{ thing.expand.location.name }}
                  </router-link>
                  <span v-else class="text-sm text-base-content/40">No location assigned</span>
                </dd>
              </div>
            </dl>
          </BaseCard>

          <!-- UPDATED: Metadata Card -->
          <BaseCard v-if="thing.metadata && Object.keys(thing.metadata).length > 0">
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Metadata</h3>
                <button @click="copyMetadata" class="btn btn-xs btn-ghost gap-1 opacity-70 hover:opacity-100" title="Copy raw JSON">
                  📋 Copy
                </button>
              </div>
            </template>

            <div class="bg-base-200 rounded-lg p-4 border border-base-300 overflow-hidden">
              <div class="max-h-[500px] overflow-y-auto overflow-x-auto custom-scrollbar">
                <pre class="text-sm font-mono leading-relaxed" v-html="highlightedMetadata"></pre>
              </div>
            </div>
          </BaseCard>
        </div>
        
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">NATS Connectivity</h3>
                <div class="flex gap-2" v-if="thing.expand?.nats_user">
                  <button @click="downloadNatsCreds" class="btn btn-sm btn-outline h-8 min-h-0" title="Download .creds file">
                    <span class="text-lg">📥</span>
                    <span class="hidden sm:inline">.creds</span>
                  </button>
                  <button @click="showRegenerateModal = true" class="btn btn-sm btn-outline btn-error h-8 min-h-0" title="Regenerate credentials">🔄</button>
                </div>
              </div>
            </template>

            <div v-if="thing.expand?.nats_user" class="flex flex-col gap-4">
              <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                <div class="flex justify-between items-start mb-1">
                  <span class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Username</span>
                  <div class="flex items-center gap-1.5" v-if="thing.expand.nats_user.active">
                    <span class="w-2 h-2 rounded-full bg-success"></span>
                    <span class="text-xs font-medium text-base-content/70">Active</span>
                  </div>
                  <div class="flex items-center gap-1.5" v-else>
                    <span class="w-2 h-2 rounded-full bg-error"></span>
                    <span class="text-xs font-medium text-base-content/70">Inactive</span>
                  </div>
                </div>
                <div class="font-mono text-base break-all select-all">{{ thing.expand.nats_user.nats_username }}</div>
              </div>
            </div>
            <div v-else class="text-center py-8 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">📡</span>
              <p class="text-sm">No NATS user linked</p>
            </div>
          </BaseCard>
          
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-2">
                <h3 class="card-title text-base">Nebula Connectivity</h3>
                <div v-if="thing.expand?.nebula_host">
                  <button @click="downloadNebulaConfig" class="btn btn-sm btn-outline h-8 min-h-0">📥 Config</button>
                </div>
              </div>
            </template>

            <div v-if="thing.expand?.nebula_host" class="flex flex-col gap-4">
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                  <span class="text-xs font-bold text-base-content/50 uppercase block mb-1">Hostname</span>
                  <div class="font-mono text-sm break-all">{{ thing.expand.nebula_host.hostname }}</div>
                </div>
                <div class="bg-base-200 rounded-lg p-3 border border-base-300">
                  <span class="text-xs font-bold text-base-content/50 uppercase block mb-1">Overlay IP</span>
                  <div class="font-mono text-sm">{{ thing.expand.nebula_host.overlay_ip }}</div>
                </div>
              </div>
            </div>
            <div v-else class="text-center py-8 text-base-content/50 bg-base-200/50 rounded-lg border border-dashed border-base-300">
              <span class="text-2xl block mb-2">🌐</span>
              <p class="text-sm">No Nebula host linked</p>
            </div>
          </BaseCard>
        </div>
      </div>

      <div v-if="thing.code && natsStore.isConnected" class="mt-6">
        <KvDashboard :key="thing.code" :base-key="`thing.${thing.code}`" />
      </div>
      <div v-else class="mt-6">
        <div v-if="hasDigitalTwinConfig && !natsStore.isConnected" class="alert shadow-sm border border-base-300 bg-base-100">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-info shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          <div class="text-xs">
            <div class="font-bold">Digital Twin Offline</div> 
            <span class="opacity-70">Connect to NATS in <router-link to="/settings" class="link">Settings</router-link> to view live data.</span>
          </div>
        </div>
      </div>
    </template>

    <dialog class="modal" :class="{ 'modal-open': showRegenerateModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Regenerate Credentials?</h3>
        <p class="py-4">This will invalidate the existing credentials immediately.</p>
        <div class="modal-action">
          <button class="btn" @click="showRegenerateModal = false" :disabled="regenerating">Cancel</button>
          <button class="btn btn-error" @click="confirmRegenerate" :disabled="regenerating">
            <span v-if="regenerating" class="loading loading-spinner"></span> Regenerate
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop"><button @click="showRegenerateModal = false">close</button></form>
    </dialog>
  </div>
</template>

<style scoped>
.custom-scrollbar::-webkit-scrollbar {
  width: 4px;
  height: 4px;
}
.custom-scrollbar::-webkit-scrollbar-thumb {
  background: oklch(var(--bc) / 0.2);
  border-radius: 10px;
}
.custom-scrollbar::-webkit-scrollbar-track {
  background: transparent;
}
</style>
