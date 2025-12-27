<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { NatsAccount } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const account = ref<NatsAccount | null>(null)
const loading = ref(true)

const accountId = route.params.id as string

/**
 * Load account details
 */
async function loadAccount() {
  loading.value = true
  
  try {
    account.value = await pb.collection('nats_accounts').getOne<NatsAccount>(accountId)
  } catch (err: any) {
    toast.error(err.message || 'Failed to load NATS account')
    router.push('/nats/accounts')
  } finally {
    loading.value = false
  }
}

/**
 * Format limit value
 */
function formatLimit(value?: number) {
  if (value === undefined || value === null) return 'Not set'
  if (value === -1) return 'Unlimited'
  return value.toLocaleString()
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

onMounted(() => {
  loadAccount()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="account">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/accounts">NATS Accounts</router-link></li>
            <li class="truncate max-w-[200px]">{{ account.name }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ account.name }}</h1>
            <span 
              class="badge"
              :class="account.active ? 'badge-success' : 'badge-error'"
            >
              {{ account.active ? 'Active' : 'Inactive' }}
            </span>
          </div>
        </div>
      </div>
      
      <!-- Alert -->
      <div class="alert alert-info">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="stroke-current shrink-0 w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
        <span>NATS Accounts are automatically managed by the system. To use this account, create NATS Users.</span>
      </div>
      
      <!-- Details Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Basic Info -->
        <BaseCard title="Basic Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Name</dt>
              <dd class="mt-1 text-sm">{{ account.name }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Description</dt>
              <dd class="mt-1 text-sm">{{ account.description || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Status</dt>
              <dd class="mt-1">
                <span 
                  class="badge"
                  :class="account.active ? 'badge-success' : 'badge-error'"
                >
                  {{ account.active ? 'Active' : 'Inactive' }}
                </span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Connection Limits -->
        <BaseCard title="Connection Limits">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Connections</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(account.max_connections) }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Subscriptions</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(account.max_subscriptions) }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Payload</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(account.max_payload) }} bytes</code>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Data Limits -->
        <BaseCard title="Data Limits">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Data</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(account.max_data) }} bytes</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max JetStream Disk Storage</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(account.max_jetstream_disk_storage) }} bytes</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max JetStream Memory Storage</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(account.max_jetstream_memory_storage) }} bytes</code>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Keys -->
        <BaseCard title="Account Keys">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Public Key</dt>
              <dd class="mt-1 flex items-center gap-2">
                <code class="text-xs break-all">{{ account.public_key || 'Not generated' }}</code>
                <button 
                  v-if="account.public_key"
                  @click="copyToClipboard(account.public_key, 'Public key')"
                  class="btn btn-ghost btn-xs"
                  title="Copy to clipboard"
                >
                  📋
                </button>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Signing Public Key</dt>
              <dd class="mt-1 flex items-center gap-2">
                <code class="text-xs break-all">{{ account.signing_public_key || 'Not generated' }}</code>
                <button 
                  v-if="account.signing_public_key"
                  @click="copyToClipboard(account.signing_public_key, 'Signing public key')"
                  class="btn btn-ghost btn-xs"
                  title="Copy to clipboard"
                >
                  📋
                </button>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- JWT -->
        <BaseCard v-if="account.jwt" title="Account JWT" class="lg:col-span-2">
          <div class="flex items-start gap-2">
            <div class="flex-1 bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto break-all">
              {{ account.jwt }}
            </div>
            <button 
              @click="copyToClipboard(account.jwt!, 'JWT')"
              class="btn btn-ghost btn-sm"
              title="Copy to clipboard"
            >
              📋
            </button>
          </div>
        </BaseCard>
        
        <!-- System Info -->
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ account.id }}</code>
              </dd>
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
      </div>
    </template>
  </div>
</template>
