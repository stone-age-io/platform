<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { NatsRole } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

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
  if (!role.value || !confirm(`Delete "${role.value.name}"?`)) return
  try {
    await pb.collection('nats_roles').delete(role.value.id)
    toast.success('Role deleted')
    router.push('/nats/roles')
  } catch (err: any) {
    toast.error(err.message)
  }
}

function formatLimit(value?: number) {
  if (value === undefined || value === null) return 'Not set'
  return value === -1 ? 'Unlimited' : value.toString()
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
      
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
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
          </dl>
        </BaseCard>
        
        <BaseCard title="Limits">
          <dl class="grid grid-cols-2 gap-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Subscriptions</dt>
              <dd class="mt-1 font-mono text-sm">{{ formatLimit(role.max_subscriptions) }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Payload</dt>
              <dd class="mt-1 font-mono text-sm">{{ formatLimit(role.max_payload) }}</dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- PUBLISHING -->
        <div class="lg:col-span-2 space-y-4">
          <h3 class="text-lg font-bold flex items-center gap-2">
            <span>📤</span> Publishing Permissions
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <BaseCard title="Allow" class="border-t-4 border-t-success">
              <div v-if="getSubjectArray(role.publish_permissions).length" class="flex flex-wrap gap-2">
                <!-- Use s directly, avoid quotes -->
                <code v-for="s in getSubjectArray(role.publish_permissions)" :key="s" class="badge badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
              </div>
              <div v-else class="text-xs opacity-40 italic">No allow rules</div>
            </BaseCard>

            <BaseCard title="Deny" class="border-t-4 border-t-error">
              <div v-if="getSubjectArray(role.publish_deny_permissions).length" class="flex flex-wrap gap-2">
                <code v-for="s in getSubjectArray(role.publish_deny_permissions)" :key="s" class="badge badge-error badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
              </div>
              <div v-else class="text-xs opacity-40 italic">No deny rules</div>
            </BaseCard>
          </div>
        </div>

        <!-- SUBSCRIBING -->
        <div class="lg:col-span-2 space-y-4">
          <h3 class="text-lg font-bold flex items-center gap-2">
            <span>📥</span> Subscribing Permissions
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <BaseCard title="Allow" class="border-t-4 border-t-success">
              <div v-if="getSubjectArray(role.subscribe_permissions).length" class="flex flex-wrap gap-2">
                <code v-for="s in getSubjectArray(role.subscribe_permissions)" :key="s" class="badge badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
              </div>
              <div v-else class="text-xs opacity-40 italic">No allow rules</div>
            </BaseCard>

            <BaseCard title="Deny" class="border-t-4 border-t-error">
              <div v-if="getSubjectArray(role.subscribe_deny_permissions).length" class="flex flex-wrap gap-2">
                <code v-for="s in getSubjectArray(role.subscribe_deny_permissions)" :key="s" class="badge badge-error badge-outline font-mono text-xs h-auto py-1.5 px-2">{{ s }}</code>
              </div>
              <div v-else class="text-xs opacity-40 italic">No deny rules</div>
            </BaseCard>
          </div>
        </div>
        
        <BaseCard title="System Information" class="lg:col-span-2">
          <dl class="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
            <div>
              <dt class="font-bold opacity-50 uppercase">ID</dt>
              <dd class="mt-1 font-mono">{{ role.id }}</dd>
            </div>
            <div>
              <dt class="font-bold opacity-50 uppercase">Created</dt>
              <dd class="mt-1">{{ formatDate(role.created) }}</dd>
            </div>
            <div>
              <dt class="font-bold opacity-50 uppercase">Updated</dt>
              <dd class="mt-1">{{ formatDate(role.updated) }}</dd>
            </div>
          </dl>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
