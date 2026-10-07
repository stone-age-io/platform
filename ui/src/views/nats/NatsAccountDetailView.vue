<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate, formatBytes } from '@/utils/format'
import { useAuthStore } from '@/stores/auth'
import type { NatsAccount } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const { confirm } = useConfirm()
const authStore = useAuthStore()

const account = ref<NatsAccount | null>(null)
const loading = ref(true)
const rotating = ref(false)
const addingKey = ref(false)
const removingKey = ref('')

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

/**
 * Signing-key operations go through a dedicated route, not a record update.
 * `nats_accounts.updateRule` is operator-only, because a rule cannot permit these
 * three trigger fields while forbidding the account limits and the signed `jwt`.
 * The route takes no record id — it always acts on the caller's active
 * organization's account. See hooks/nats_account_routes.go.
 */
type KeyAction = 'rotate' | 'add_signing' | 'remove_signing'

async function applyKeyAction(action: KeyAction, publicKey?: string) {
  await pb.send('/api/org/nats-account/keys', {
    method: 'POST',
    body: { action, ...(publicKey ? { public_key: publicKey } : {}) },
  })
  await loadAccount()
}

// What happens after a key change, from pb-nats (internal/sync/manager.go):
// NATS rejects a user JWT whose issuer is no longer one of the account's signing
// keys, so every deployed .creds file signed with a removed key dies at once.
// pb-nats then reissues a file for every ACTIVE user in the account by itself
// (regenerateUsersInAccount), and skips suspended ones, so a rotation is not a
// way back for a revoked device. The work left to a person is DELIVERING those
// files, not regenerating them -- the old dialog said the opposite.
async function rotateKeys() {
  if (!account.value) return

  const confirmed = await confirm({
    title: 'Rotate All Signing Keys',
    message: 'Replace every signing key on this account with one new key?',
    details: 'Every .creds file in use in this account stops working immediately, on every device and app. New files are issued automatically for every active user; suspended users stay suspended. Each device then needs its new file. Use this when a signing key has leaked.',
    confirmText: 'Rotate All Keys',
    variant: 'danger',
  })
  if (!confirmed) return

  rotating.value = true
  try {
    await applyKeyAction('rotate')
    toast.success('Keys rotated. Deliver the new .creds files.')
  } catch (err: any) {
    toast.error(err.message || 'Failed to rotate keys')
  } finally {
    rotating.value = false
  }
}

async function addSigningKey() {
  if (!account.value) return

  addingKey.value = true
  try {
    await applyKeyAction('add_signing')
    toast.success('Signing key added')
  } catch (err: any) {
    toast.error(err.message || 'Failed to add signing key')
  } finally {
    addingKey.value = false
  }
}

// See rotateKeys for what follows a removal.
async function removeKey(publicKey: string) {
  if (!account.value) return

  const confirmed = await confirm({
    title: 'Remove Signing Key',
    message: `Remove signing key ${publicKey.slice(0, 12)}…?`,
    details: 'Any .creds file signed with this key stops working immediately. New files are issued automatically for every active user in the account, and devices holding a file signed with this key need theirs.',
    confirmText: 'Remove Key',
    variant: 'danger',
  })
  if (!confirmed) return

  removingKey.value = publicKey
  try {
    await applyKeyAction('remove_signing', publicKey)
    toast.success('Signing key removed')
  } catch (err: any) {
    toast.error(err.message || 'Failed to remove signing key')
  } finally {
    removingKey.value = ''
  }
}

function getSigningKeyPublicKey(key: any): string {
  if (typeof key === 'string') return key
  return key?.public_key || key?.publicKey || JSON.stringify(key)
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
      <!--
        There used to be a "Provision NATS Account" button here that created the
        record from the browser. It could never work for the roles that reach
        this view -- nats_accounts.createRule is null -- and for a superuser it
        wrote stale hard-coded limits instead of nats.default_limits. The server
        provisions the account (hooks/org_provisioning.go), create-if-missing on
        every organization save, so re-saving the organization IS the retry.
      -->
      <p class="text-base-content/70 mt-2 max-w-md mx-auto">
        Your organization does not have a NATS account yet. It is provisioned by the
        server when the organization is saved; if that failed, saving the organization
        again retries it.
      </p>
      <router-link
        v-if="authStore.isOperator && authStore.currentOrgId"
        :to="{ name: 'AdminOrgEdit', params: { id: authStore.currentOrgId } }"
        class="btn btn-primary mt-6"
      >
        Edit organization to retry
      </router-link>
      <p v-else class="text-sm text-base-content/60 mt-4 max-w-md mx-auto">
        Ask a Platform Operator to re-save this organization.
      </p>
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
                <div class="flex gap-2">
                  <button
                    @click="addSigningKey"
                    class="btn btn-sm btn-outline btn-success"
                    :disabled="addingKey"
                    title="Add Signing Key"
                  >
                    <span v-if="addingKey" class="loading loading-spinner loading-xs"></span>
                    <span v-else class="text-lg">+</span>
                    Add Key
                  </button>
                  <button
                    @click="rotateKeys"
                    class="btn btn-sm btn-outline btn-warning"
                    title="Emergency Key Rotation"
                    :disabled="rotating"
                  >
                    <span class="text-lg">🔄</span>
                    Rotate All
                  </button>
                </div>
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

              <!-- Signing Keys List -->
              <div>
                <div class="text-xs font-bold text-base-content/50 uppercase tracking-wider mb-2">Signing Keys</div>
                <div v-if="account.signing_keys && account.signing_keys.length" class="space-y-2">
                  <div
                    v-for="(key, index) in account.signing_keys"
                    :key="index"
                    class="bg-base-200 p-3 rounded-lg border border-base-300 flex items-start justify-between gap-2"
                  >
                    <div class="font-mono text-xs break-all flex-1 pt-0.5">
                      {{ getSigningKeyPublicKey(key) }}
                    </div>
                    <div class="flex gap-1 shrink-0">
                      <button
                        @click="copyToClipboard(getSigningKeyPublicKey(key), 'Signing Key')"
                        class="btn btn-ghost btn-xs"
                      >Copy</button>
                      <button
                        @click="removeKey(getSigningKeyPublicKey(key))"
                        class="btn btn-ghost btn-xs text-error"
                        :disabled="removingKey === getSigningKeyPublicKey(key) || account.signing_keys!.length <= 1"
                        :title="account.signing_keys!.length <= 1 ? 'Cannot remove the only signing key' : 'Remove this signing key'"
                      >
                        <span v-if="removingKey === getSigningKeyPublicKey(key)" class="loading loading-spinner loading-xs"></span>
                        <span v-else>Remove</span>
                      </button>
                    </div>
                  </div>
                </div>
                <div v-else class="text-xs text-base-content/40 italic">No signing keys</div>
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
  </div>
</template>
