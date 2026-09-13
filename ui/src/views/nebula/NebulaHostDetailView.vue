<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate } from '@/utils/format'
import { fetchStaleHostIds, expectedCertNetwork } from '@/utils/nebula'
import type { NebulaHost } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import JsonViewer from '@/components/common/JsonViewer.vue'
import { useEscapeKey } from '@/composables/useEscapeKey'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const host = ref<NebulaHost | null>(null)
const loading = ref(true)
const deleting = ref(false)
const regenerating = ref(false)
const showRegenerateModal = ref(false)
const certIsStale = ref(false)

const hostId = route.params.id as string

/**
 * What the certificate SHOULD say, for the warning below. Presentation only --
 * the comparison itself happens on the server, against the certificate.
 */
const expectedNetwork = computed(() =>
  expectedCertNetwork(host.value?.overlay_ip, host.value?.expand?.network_id?.cidr_range),
)

async function loadHost() {
  loading.value = true
  try {
    host.value = await pb.collection('nebula_hosts').getOne<NebulaHost>(hostId, {
      expand: 'network_id',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load Nebula host')
    router.push('/nebula/hosts')
    return
  } finally {
    loading.value = false
  }

  // Advisory, and deliberately not awaited into the loading state: the page is
  // useful without it, and an unreachable audit endpoint must not blank a host
  // the operator came here to read.
  certIsStale.value = (await fetchStaleHostIds()).has(hostId)
}

async function handleDelete() {
  if (!host.value) return
  const confirmed = await confirm({
    title: 'Delete Nebula Host',
    message: `Are you sure you want to delete "${host.value.hostname}"?`,
    details: 'This will invalidate the host certificate. The host will no longer be able to connect to the overlay network.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('nebula_hosts').delete(host.value.id)
    toast.success('Nebula host deleted')
    router.push('/nebula/hosts')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete Nebula host')
  } finally {
    deleting.value = false
  }
}

/**
 * Re-issue this host's certificate.
 *
 * The field is `renew`, and it used to be `regenerate` -- which is not a field
 * on nebula_hosts and never has been. PocketBase drops unknown keys from an
 * update body without complaint, so this button returned 200, toasted
 * "Certificate regenerated" and did nothing at all, for every release up to
 * this one. The certificate on screen afterwards was the same certificate.
 *
 * `renew` is an action field: pb-nebula re-issues on the false -> true edge and
 * resets it in the same save, so it never reads back as state.
 */
async function confirmRegenerate() {
  if (!host.value) return

  regenerating.value = true
  try {
    await pb.collection('nebula_hosts').update(host.value.id, { renew: true })
    toast.success('Certificate re-issued — redeploy this host\'s config')
    showRegenerateModal.value = false
    await loadHost()
  } catch (err: any) {
    toast.error(err.message || 'Failed to re-issue certificate')
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

// Escape closes these; see useEscapeKey for why the dialogs do not get it
// from the browser and which ones are deliberately left out.
useEscapeKey(showRegenerateModal, () => { showRegenerateModal.value = false })
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
            <span v-if="host.is_relay" class="badge badge-secondary badge-outline gap-1">
              ↪️ Relay
            </span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nebula/hosts/${host.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
              Edit
            </router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial" :disabled="deleting">
              Delete
            </button>
          </div>
        </div>
      </div>
      
      <!--
        The /32 warning. Worth a banner rather than a badge because the failure
        it describes is invisible from every other angle: the certificate is
        valid, the config renders, the host starts, the handshake completes, and
        no traffic moves. Nothing else on this page looks wrong.
      -->
      <div v-if="certIsStale" class="alert alert-warning items-start">
        <span class="text-xl">⚠️</span>
        <div>
          <h3 class="font-bold">Certificate does not match this network</h3>
          <div class="text-sm mt-1">
            Nebula builds this host's overlay route from the network in its certificate,
            so a certificate issued at the wrong mask leaves the host unable to reach
            any peer — with no error anywhere.
            <template v-if="expectedNetwork">
              It should carry <code class="font-mono">{{ expectedNetwork }}</code>.
            </template>
          </div>
          <div class="text-sm mt-1">
            Re-issue it below, then redeploy this host's config.
          </div>
        </div>
        <button class="btn btn-sm" @click="showRegenerateModal = true">Re-issue</button>
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

              <!-- Signed into the certificate, unlike everything below it. -->
              <div v-if="host.unsafe_networks && host.unsafe_networks.length > 0">
                <dt class="text-sm font-medium text-base-content/70 mb-1">
                  Routes To (unsafe networks)
                </dt>
                <dd class="flex flex-wrap gap-2">
                  <code
                    v-for="net in host.unsafe_networks"
                    :key="net"
                    class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono"
                  >{{ net }}</code>
                </dd>
              </div>

              <div v-if="host.unsafe_routes && host.unsafe_routes.length > 0">
                <dt class="text-sm font-medium text-base-content/70 mb-1">
                  Routes Via (unsafe routes)
                </dt>
                <dd class="space-y-1">
                  <div v-for="(r, i) in host.unsafe_routes" :key="i" class="text-sm font-mono">
                    {{ r.route }} <span class="text-base-content/50">via</span> {{ r.via }}
                  </div>
                </dd>
              </div>

              <div v-if="host.preferred_ranges && host.preferred_ranges.length > 0">
                <dt class="text-sm font-medium text-base-content/70 mb-1">
                  Preferred Ranges (underlay)
                </dt>
                <dd class="flex flex-wrap gap-2">
                  <code
                    v-for="range in host.preferred_ranges"
                    :key="range"
                    class="text-sm bg-base-200 px-2 py-0.5 rounded font-mono"
                  >{{ range }}</code>
                </dd>
              </div>

              <div v-if="host.mtu || host.tun_device">
                <dt class="text-sm font-medium text-base-content/70">Transport Overrides</dt>
                <dd class="mt-1 text-sm font-mono">
                  <span v-if="host.mtu">MTU {{ host.mtu }}</span>
                  <span v-if="host.mtu && host.tun_device" class="text-base-content/50"> · </span>
                  <span v-if="host.tun_device">{{ host.tun_device }}</span>
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
                  <JsonViewer :data="host.firewall_inbound" class="text-xs" />
                </div>
              </div>
              <div>
                <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-2">Outbound</div>
                <div class="bg-base-200 rounded-lg p-3 overflow-x-auto border border-base-300 max-h-60">
                  <JsonViewer :data="host.firewall_outbound" class="text-xs" />
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
                    title="Re-issue certificate"
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
        <h3 class="font-bold text-lg text-warning">Re-issue Certificate?</h3>
        <p class="py-4">
          A new key pair and certificate are issued immediately, and this host's config is
          regenerated around them. The host keeps running on its old certificate until you
          download the new config and redeploy it.
        </p>
        <p class="pb-4 text-sm text-base-content/70">
          The certificate's fingerprint changes, which is what peers blocklist when a host
          is deactivated — so re-issue when you intend to redeploy, not to tidy up.
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
            Re-issue
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop">
        <button @click="showRegenerateModal = false">close</button>
      </form>
    </dialog>
  </div>
</template>
