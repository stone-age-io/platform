<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { NatsUser } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const user = ref<NatsUser | null>(null)
const loading = ref(true)

const userId = route.params.id as string

/**
 * Load user details
 */
async function loadUser() {
  loading.value = true
  
  try {
    user.value = await pb.collection('nats_users').getOne<NatsUser>(userId, {
      expand: 'account_id,role_id',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load NATS user')
    router.push('/nats/users')
  } finally {
    loading.value = false
  }
}

/**
 * Handle delete
 */
async function handleDelete() {
  if (!user.value) return
  if (!confirm(`Delete NATS user "${user.value.nats_username}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nats_users').delete(user.value.id)
    toast.success('NATS user deleted')
    router.push('/nats/users')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete NATS user')
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
 * Download credentials file
 */
function downloadCredsFile() {
  if (!user.value?.creds_file) return
  
  const blob = new Blob([user.value.creds_file], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${user.value.nats_username}.creds`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  
  toast.success('Credentials file downloaded')
}

onMounted(() => {
  loadUser()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="user">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/users">NATS Users</router-link></li>
            <li class="truncate max-w-[200px] font-mono">{{ user.nats_username }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold font-mono break-words">{{ user.nats_username }}</h1>
            <span 
              class="badge"
              :class="user.active ? 'badge-success' : 'badge-error'"
            >
              {{ user.active ? 'Active' : 'Inactive' }}
            </span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nats/users/${user.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
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
              <dt class="text-sm font-medium text-base-content/70">Username</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ user.nats_username }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Description</dt>
              <dd class="mt-1 text-sm">{{ user.description || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Account</dt>
              <dd class="mt-1">
                <router-link 
                  v-if="user.expand?.account_id"
                  :to="`/nats/accounts/${user.account_id}`"
                  class="link link-hover"
                >
                  {{ user.expand.account_id.name }}
                </router-link>
                <span v-else>-</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Role</dt>
              <dd class="mt-1">
                <router-link 
                  v-if="user.expand?.role_id"
                  :to="`/nats/roles/${user.role_id}`"
                  class="link link-hover"
                >
                  {{ user.expand.role_id.name }}
                </router-link>
                <span v-else>-</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Status</dt>
              <dd class="mt-1">
                <span 
                  class="badge"
                  :class="user.active ? 'badge-success' : 'badge-error'"
                >
                  {{ user.active ? 'Active' : 'Inactive' }}
                </span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Authentication -->
        <BaseCard title="Authentication">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Bearer Token</dt>
              <dd class="mt-1">
                <span class="badge" :class="user.bearer_token ? 'badge-success' : 'badge-ghost'">
                  {{ user.bearer_token ? 'Enabled' : 'Disabled' }}
                </span>
              </dd>
            </div>
            <div v-if="user.jwt_expires_at">
              <dt class="text-sm font-medium text-base-content/70">JWT Expires</dt>
              <dd class="mt-1 text-sm">{{ formatDate(user.jwt_expires_at) }}</dd>
            </div>
            <div v-if="user.public_key">
              <dt class="text-sm font-medium text-base-content/70">Public Key</dt>
              <dd class="mt-1 flex items-center gap-2">
                <code class="text-xs break-all flex-1">{{ user.public_key }}</code>
                <button 
                  @click="copyToClipboard(user.public_key!, 'Public key')"
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
        <BaseCard v-if="user.jwt" title="User JWT" class="lg:col-span-2">
          <div class="flex items-start gap-2">
            <div class="flex-1 bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto break-all">
              {{ user.jwt }}
            </div>
            <button 
              @click="copyToClipboard(user.jwt!, 'JWT')"
              class="btn btn-ghost btn-sm"
              title="Copy to clipboard"
            >
              📋
            </button>
          </div>
        </BaseCard>
        
        <!-- Credentials File -->
        <BaseCard v-if="user.creds_file" title="Credentials File" class="lg:col-span-2">
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm">
                Download the credentials file to authenticate with NATS
              </p>
              <p class="text-xs text-base-content/60 mt-1">
                File: <code>{{ user.nats_username }}.creds</code>
              </p>
            </div>
            <button 
              @click="downloadCredsFile"
              class="btn btn-primary btn-sm"
            >
              📥 Download
            </button>
          </div>
        </BaseCard>
        
        <!-- System Info -->
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ user.id }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Created</dt>
              <dd class="mt-1 text-sm">{{ formatDate(user.created) }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
              <dd class="mt-1 text-sm">{{ formatDate(user.updated) }}</dd>
            </div>
          </dl>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
