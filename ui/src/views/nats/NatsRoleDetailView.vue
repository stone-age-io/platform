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
 * Load role details
 */
async function loadRole() {
  loading.value = true
  
  try {
    role.value = await pb.collection('nats_roles').getOne<NatsRole>(roleId)
  } catch (err: any) {
    toast.error(err.message || 'Failed to load role')
    router.push('/nats/roles')
  } finally {
    loading.value = false
  }
}

/**
 * Handle delete
 */
async function handleDelete() {
  if (!role.value) return
  if (!confirm(`Delete "${role.value.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('nats_roles').delete(role.value.id)
    toast.success('Role deleted')
    router.push('/nats/roles')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete role')
  }
}

/**
 * Format limit value
 */
function formatLimit(value?: number) {
  if (value === undefined || value === null) return 'Not set'
  if (value === -1) return 'Unlimited'
  return value.toString()
}

onMounted(() => {
  loadRole()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="role">
      <!-- Header -->
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
            <router-link :to="`/nats/roles/${role.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
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
              <dt class="text-sm font-medium text-base-content/70">Name</dt>
              <dd class="mt-1 text-sm">{{ role.name }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Description</dt>
              <dd class="mt-1 text-sm">{{ role.description || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Default Role</dt>
              <dd class="mt-1">
                <span class="badge" :class="role.is_default ? 'badge-primary' : 'badge-ghost'">
                  {{ role.is_default ? 'Yes' : 'No' }}
                </span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Limits -->
        <BaseCard title="Limits">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Subscriptions</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(role.max_subscriptions) }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Data</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(role.max_data) }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Max Payload</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ formatLimit(role.max_payload) }}</code>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Permissions -->
        <BaseCard title="Publish Permissions" class="lg:col-span-2">
          <div v-if="role.publish_permissions" class="bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all">
            {{ role.publish_permissions }}
          </div>
          <div v-else class="text-sm text-base-content/40">
            No publish permissions set
          </div>
        </BaseCard>
        
        <BaseCard title="Subscribe Permissions" class="lg:col-span-2">
          <div v-if="role.subscribe_permissions" class="bg-base-200 p-4 rounded font-mono text-xs overflow-x-auto whitespace-pre-wrap break-all">
            {{ role.subscribe_permissions }}
          </div>
          <div v-else class="text-sm text-base-content/40">
            No subscribe permissions set
          </div>
        </BaseCard>
        
        <!-- System Info -->
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ role.id }}</code>
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
      </div>
    </template>
  </div>
</template>
