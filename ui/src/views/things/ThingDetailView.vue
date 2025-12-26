<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { Thing } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const thing = ref<Thing | null>(null)
const loading = ref(true)

const thingId = route.params.id as string

/**
 * Load thing details
 */
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

/**
 * Handle delete
 */
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
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="thing">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/things">Things</router-link></li>
            <li class="truncate max-w-[200px]">{{ thing.name || 'Unnamed' }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <h1 class="text-3xl font-bold break-words">{{ thing.name || 'Unnamed Thing' }}</h1>
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
      
      <!-- Details Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Basic Info -->
        <BaseCard title="Basic Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">Name</dt>
              <dd class="mt-1 text-sm">{{ thing.name || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Description</dt>
              <dd class="mt-1 text-sm">{{ thing.description || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Code</dt>
              <dd class="mt-1">
                <code v-if="thing.code" class="text-sm">{{ thing.code }}</code>
                <span v-else class="text-sm">-</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Type</dt>
              <dd class="mt-1">
                <span v-if="thing.expand?.type" class="badge badge-ghost">
                  {{ thing.expand.type.name }}
                </span>
                <span v-else class="text-sm">-</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Location</dt>
              <dd class="mt-1">
                <router-link 
                  v-if="thing.expand?.location" 
                  :to="`/locations/${thing.location}`"
                  class="link link-hover"
                >
                  {{ thing.expand.location.name }}
                </router-link>
                <span v-else class="text-sm">-</span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Infrastructure -->
        <BaseCard title="Infrastructure">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">NATS User</dt>
              <dd class="mt-1">
                <span v-if="thing.expand?.nats_user">
                  {{ thing.expand.nats_user.nats_username }}
                </span>
                <span v-else class="text-sm text-base-content/40">Not configured</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Nebula Host</dt>
              <dd class="mt-1">
                <span v-if="thing.expand?.nebula_host">
                  {{ thing.expand.nebula_host.hostname }}
                </span>
                <span v-else class="text-sm text-base-content/40">Not configured</span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Metadata -->
        <BaseCard v-if="thing.metadata" title="Metadata">
          <pre class="text-xs bg-base-200 p-4 rounded overflow-x-auto">{{ JSON.stringify(thing.metadata, null, 2) }}</pre>
        </BaseCard>
        
        <!-- System Info -->
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ thing.id }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Created</dt>
              <dd class="mt-1 text-sm">{{ formatDate(thing.created) }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
              <dd class="mt-1 text-sm">{{ formatDate(thing.updated) }}</dd>
            </div>
          </dl>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
