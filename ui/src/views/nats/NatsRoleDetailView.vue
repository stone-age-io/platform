<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { formatDate, formatBytes } from '@/utils/format'
import type { NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()
const { confirm } = useConfirm()

const role = ref<NatsRole | null>(null)
const loading = ref(true)
const roleId = route.params.id as string

/**
 * Grug logic for display:
 * If it's an array, use it. 
 * If it's a string, try to parse JSON. 
 * If that fails, split by comma.
 */
function getSubjectArray(val: any): string[] {
  if (!val) return []
  if (Array.isArray(val)) return val
  
  if (typeof val === 'string') {
    const trimmed = val.trim()
    if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
      try {
        return JSON.parse(trimmed)
      } catch (e) {
        return []
      }
    }
    return trimmed.split(',').map(s => s.trim()).filter(s => s !== '')
  }
  return []
}

async function loadRole() {
  loading.value = true
  try {
    role.value = await pb.collection('nats_roles').getOne<NatsRole>(roleId)
  } catch (err: any) {
    toast.error('Failed to load role')
    router.push('/nats/roles')
  } finally {
    loading.value = false
  }
}

async function handleDelete() {
  if (!role.value) return
  const confirmed = await confirm({
    title: 'Delete NATS Role',
    message: `Are you sure you want to delete "${role.value.name}"?`,
    details: 'Users with this role will not be deleted but may lose their permissions.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    await pb.collection('nats_roles').delete(role.value.id)
    toast.success('Role deleted')
    router.push('/nats/roles')
  } catch (err: any) {
    toast.error(err.message)
  }
}

function formatLimit(value?: number, isBytes = false) {
  if (value === undefined || value === null) return 'Not set'
  if (value === -1) return 'Unlimited'
  return isBytes ? formatBytes(value) : value.toLocaleString()
}

onMounted(loadRole)
</script>

<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="role">
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/nats/roles">NATS Roles</router-link></li>
            <li class="truncate max-w-[200px]">{{ role.name }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <div class="flex items-center gap-3">
            <h1 class="text-3xl font-bold break-words">{{ role.name }}</h1>
            <span v-if="role.is_default" class="badge badge-primary">Default</span>
          </div>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/nats/roles/${role.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">Edit</router-link>
            <button @click="handleDelete" class="btn btn-error flex-1 sm:flex-initial">Delete</button>
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
                <dd class="mt-1 text-sm">{{ role.description || '-' }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Default Role</dt>
                <dd class="mt-1">
                  <span class="badge" :class="role.is_default ? 'badge-primary' : 'badge-ghost'">{{ role.is_default ? 'Yes' : 'No' }}</span>
                </dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Created</dt>
                <dd class="mt-1 text-sm">{{ formatDate(role.created) }}</dd>
              </div>
              <div>
                <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
                <dd class="mt-1 text-sm">{{ formatDate(role.updated) }}</dd>
              </div>
            </dl>
          </BaseCard>

          <BaseCard title="Resource Limits">
            <dl class="space-y-3">
              <div>
                <dt class="text-xs text-base-content/70">Max Subscriptions</dt>
                <dd class="font-mono text-sm font-medium">{{ formatLimit(role.max_subscriptions) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Max Data</dt>
                <dd class="font-mono text-sm font-medium">{{ formatLimit(role.max_data, true) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-base-content/70">Max Payload</dt>
                <dd class="font-mono text-sm font-medium">{{ formatLimit(role.max_payload, true) }}</dd>
              </div>
              <p class="text-[10px] text-base-content/40 italic">Per-user limits (applied to each user with this role)</p>
            </dl>
          </BaseCard>
        </div>

        <!-- Right Column -->
        <div class="space-y-6">
          <BaseCard title="Permissions">
            <div class="space-y-6">
              <!-- Publishing -->
              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">📤</span>
                  <h4 class="text-xs font-black uppercase text-base-content/50 tracking-widest">Publishing</h4>
                </div>
                <div>
                  <span class="text-xs text-base-content/50">Allow</span>
                  <div v-if="getSubjectArray(role.publish_permissions).length" class="flex flex-wrap gap-1.5 mt-1">
                    <code v-for="s in getSubjectArray(role.publish_permissions)" :key="s" class="badge badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
                  </div>
                  <div v-else class="text-xs text-base-content/40 italic mt-1">None</div>
                </div>
                <div>
                  <span class="text-xs text-base-content/50">Deny</span>
                  <div v-if="getSubjectArray(role.publish_deny_permissions).length" class="flex flex-wrap gap-1.5 mt-1">
                    <code v-for="s in getSubjectArray(role.publish_deny_permissions)" :key="s" class="badge badge-error badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
                  </div>
                  <div v-else class="text-xs text-base-content/40 italic mt-1">None</div>
                </div>
              </div>

              <div class="divider my-0 opacity-30"></div>

              <!-- Subscribing -->
              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">📥</span>
                  <h4 class="text-xs font-black uppercase text-base-content/50 tracking-widest">Subscribing</h4>
                </div>
                <div>
                  <span class="text-xs text-base-content/50">Allow</span>
                  <div v-if="getSubjectArray(role.subscribe_permissions).length" class="flex flex-wrap gap-1.5 mt-1">
                    <code v-for="s in getSubjectArray(role.subscribe_permissions)" :key="s" class="badge badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
                  </div>
                  <div v-else class="text-xs text-base-content/40 italic mt-1">None</div>
                </div>
                <div>
                  <span class="text-xs text-base-content/50">Deny</span>
                  <div v-if="getSubjectArray(role.subscribe_deny_permissions).length" class="flex flex-wrap gap-1.5 mt-1">
                    <code v-for="s in getSubjectArray(role.subscribe_deny_permissions)" :key="s" class="badge badge-error badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
                  </div>
                  <div v-else class="text-xs text-base-content/40 italic mt-1">None</div>
                </div>
              </div>

              <div class="divider my-0 opacity-30"></div>

              <!-- Response Permissions -->
              <div class="space-y-3">
                <div class="flex items-center gap-2">
                  <span class="text-sm">🔁</span>
                  <h4 class="text-xs font-black uppercase text-base-content/50 tracking-widest">Response</h4>
                </div>
                <div class="flex flex-wrap items-center gap-x-6 gap-y-2">
                  <div>
                    <span v-if="role.allow_response" class="badge badge-success badge-sm">Enabled</span>
                    <span v-else class="badge badge-ghost badge-sm">Disabled</span>
                  </div>
                  <template v-if="role.allow_response">
                    <div>
                      <span class="text-xs text-base-content/50">Max Responses</span>
                      <div class="font-mono text-sm">{{ role.allow_response_max === -1 ? 'Unlimited' : (role.allow_response_max || 'Default (1)') }}</div>
                    </div>
                    <div>
                      <span class="text-xs text-base-content/50">TTL</span>
                      <div class="font-mono text-sm">{{ role.allow_response_ttl ? role.allow_response_ttl + 's' : 'No limit' }}</div>
                    </div>
                  </template>
                </div>
              </div>
            </div>
          </BaseCard>
        </div>
      </div>
    </template>
  </div>
</template>
