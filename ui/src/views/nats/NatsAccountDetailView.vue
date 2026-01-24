<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate, formatBytes } from '@/utils/format'
import { useAuthStore } from '@/stores/auth'
import type { NatsAccount } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const toast = useToast()
const authStore = useAuthStore()

const account = ref<NatsAccount | null>(null)
const loading = ref(true)
const rotating = ref(false)
const showRotateModal = ref(false)

/**
 * Load account by Organization ID (Singleton)
 */
async function loadAccount() {
  if (!authStore.currentOrgId) return
  
  loading.value = true
  account.value = null
  
  try {
    // Fetch the FIRST account for this org
    account.value = await pb.collection('nats_accounts').getFirstListItem<NatsAccount>(
      `organization = "${authStore.currentOrgId}"`
    )
  } catch (err: any) {
    if (err.status !== 404) {
      toast.error(err.message || 'Failed to load NATS account')
    }
    // 404 is acceptable here (handled in template)
  } finally {
    loading.value = false
  }
}

/**
 * Self-Healing: Provision Account manually if missing
 */
async function provisionAccount() {
  if (!authStore.currentOrgId || !authStore.currentOrg) return
  loading.value = true
  try {
    await pb.collection('nats_accounts').create({
      name: authStore.currentOrg.name,
      organization: authStore.currentOrgId,
      active: true,
      max_connections: 10,
      max_subscriptions: 50,
      max_payload: 1048576
    })
    toast.success('NATS Account provisioned')
    await loadAccount()
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    loading.value = false
  }
}

function formatLimit(value?: number, isBytes = false) {
  if (value === undefined || value === null) return 'Not set'
  if (value === -1) return 'Unlimited'
  return isBytes ? formatBytes(value) : value.toLocaleString()
}

async function copyToClipboard(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(`${label} copied`)
  } catch (err) {
    toast.error('Failed to copy')
  }
}

async function confirmRotateKeys() {
  if (!account.value) return
  
  rotating.value = true
  try {
    await pb.collection('nats_accounts').update(account.value.id, {
      rotate_keys: true
    })
    toast.success('Keys rotated successfully')
    showRotateModal.value = false
    await loadAccount()
  } catch (err: any) {
    toast.error(err.message || 'Failed to rotate keys')
  } finally {
    rotating.value = false
  }
}

// Watch for org changes
function handleOrgChange() {
  loadAccount()
}

onMounted(() => {
  loadAccount()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State / Provisioning -->
    <div v-else-if="!account" class="text-center py-12">
      <span class="text-6xl">📡</span>
      <h3 class="text-xl font-bold mt-4">No NATS Account Found</h3>
      <p class="text-base-content/70 mt-2 max-w-md mx-auto">
        Your organization does not have a NATS account provisioned yet.
      </p>
      <button @click="provisionAccount" class="btn btn-primary mt-6">
        Provision NATS Account
      </button>
    </div>

    <!-- Details -->
    <template v-else>
      <div class="flex flex-col gap-4">
        <!-- Simplified Breadcrumbs -->
        <div class="breadcrumbs text-sm">
          <ul>
            <li>NATS</li>
            <li>Account Settings</li>
          </ul>
        </div>
        
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ account.name }}</h1>
            <div class="flex items-center gap-1.5 px-3 py-1 bg-base-200 rounded-full">
              <span class="w-2 h-2 rounded-full" :class="account.active ? 'bg-success' : 'bg-error'"></span>
              <span class="text-xs font-medium">{{ account.active ? 'Active' : 'Inactive' }}</span>
            </div>
          </div>
        </div>
      </div>
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
        
        <!-- Left Column -->
        <div class="space-y-6">
          <BaseCard title="Basic Information">
            <dl class="space-y-4">
              <div>
                <dt class="text-sm font-medium text-base-content/70">Description</dt>
                <dd class="mt-1 text-sm">{{ account.description || '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(account.created) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
                <dd class="mt-1 text-sm">{{ formatDate(account.updated) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Resource Limits">
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-6">
              <div class="space-y-3">
                <h4 class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Connectivity</h4>
                <div>
                  <dt class="text-xs text-base-content/70">Connections</dt>
                  <dd class="font-mono text-sm font-medium">{{ formatLimit(account.max_connections) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-base-content/70">Subscriptions</dt>
                  <dd class="font-mono text-sm font-medium">{{ formatLimit(account.max_subscriptions) }}</dd>
                </div>
              </div>

              <div class="space-y-3">
                <h4 class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Data</h4>
                <div>
                  <dt class="text-xs text-base-content/70">Max Payload</dt>
                  <dd class="font-mono text-sm font-medium">{{ formatLimit(account.max_payload, true) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-base-content/70">Total Data</dt>
                  <dd class="font-mono text-sm font-medium">{{ formatLimit(account.max_data, true) }}</dd>
                </div>
              </div>

              <div class="space-y-3 sm:col-span-2 border-t border-base-200 pt-3">
                <h4 class="text-xs font-bold text-base-content/50 uppercase tracking-wider">JetStream Storage</h4>
                <div class="grid grid-cols-2 gap-6">
                  <div>
                    <dt class="text-xs text-base-content/70">Memory</dt>
                    <dd class="font-mono text-sm font-medium">{{ formatLimit(account.max_jetstream_memory_storage, true) }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs text-base-content/70">Disk</dt>
                    <dd class="font-mono text-sm font-medium">{{ formatLimit(account.max_jetstream_disk_storage, true) }}</dd>
                  </div>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
        
        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard>
            <template #header>
              <div class="flex justify-between items-center mb-4">
                <h3 class="card-title text-base">Security & Keys</h3>
                <button 
                  @click="showRotateModal = true" 
                  class="btn btn-sm btn-outline btn-warning"
                  title="Rotate Account Keys"
                >
                  <span class="text-lg">🔄</span>
                  Rotate Keys
                </button>
              </div>
            </template>

            <div class="space-y-6">
              <div>
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Account Public Key</div>
                  <button @click="copyToClipboard(account.public_key!, 'Public Key')" class="btn btn-ghost btn-xs">Copy</button>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all border border-base-300">
                  {{ account.public_key || 'Not Generated' }}
                </div>
              </div>

              <div>
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Signing Public Key</div>
                  <button @click="copyToClipboard(account.signing_public_key!, 'Signing Key')" class="btn btn-ghost btn-xs">Copy</button>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all border border-base-300">
                  {{ account.signing_public_key || 'Not Generated' }}
                </div>
              </div>

              <div>
                <div class="flex justify-between items-center mb-1">
                  <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider">Account JWT</div>
                  <button @click="copyToClipboard(account.jwt!, 'JWT')" class="btn btn-ghost btn-xs">Copy</button>
                </div>
                <div class="bg-base-200 p-3 rounded-lg font-mono text-xs break-all max-h-32 overflow-y-auto border border-base-300">
                  {{ account.jwt || 'Not Generated' }}
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>

    <!-- Rotate Modal -->
    <dialog class="modal" :class="{ 'modal-open': showRotateModal }">
      <div class="modal-box">
        <h3 class="font-bold text-lg text-warning">Rotate Account Keys?</h3>
        <p class="py-4">
          This will generate new signing keys and update the Account JWT. 
          <br><br>
          <strong>Warning:</strong> Existing users may need to have their credentials updated if they rely on the old signing key chain.
        </p>
        <div class="modal-action">
          <button class="btn" @click="showRotateModal = false" :disabled="rotating">Cancel</button>
          <button class="btn btn-warning" @click="confirmRotateKeys" :disabled="rotating">
            <span v-if="rotating" class="loading loading-spinner"></span> Rotate Keys
          </button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="showRotateModal = false"><button>close</button></form>
    </dialog>
  </div>
</template>
