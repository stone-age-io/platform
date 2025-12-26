<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { pb } from '@/utils/pb'
import { useToast } from '@/composables/useToast'
import { formatDate } from '@/utils/format'
import type { Location } from '@/types/pocketbase'
import BaseCard from '@/components/ui/BaseCard.vue'

const router = useRouter()
const route = useRoute()
const toast = useToast()

const location = ref<Location | null>(null)
const loading = ref(true)

const locationId = route.params.id as string

/**
 * Load location details
 */
async function loadLocation() {
  loading.value = true
  
  try {
    location.value = await pb.collection('locations').getOne<Location>(locationId, {
      expand: 'type,parent',
    })
  } catch (err: any) {
    toast.error(err.message || 'Failed to load location')
    router.push('/locations')
  } finally {
    loading.value = false
  }
}

/**
 * Handle delete
 */
async function handleDelete() {
  if (!location.value) return
  if (!confirm(`Delete "${location.value.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('locations').delete(location.value.id)
    toast.success('Location deleted')
    router.push('/locations')
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete location')
  }
}

/**
 * Get floorplan URL
 */
function getFloorplanUrl() {
  if (!location.value?.floorplan) return null
  return pb.files.getUrl(location.value, location.value.floorplan)
}

onMounted(() => {
  loadLocation()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <template v-else-if="location">
      <!-- Header -->
      <div class="flex flex-col gap-4">
        <div class="breadcrumbs text-sm">
          <ul>
            <li><router-link to="/locations">Locations</router-link></li>
            <li class="truncate max-w-[200px]">{{ location.name || 'Unnamed' }}</li>
          </ul>
        </div>
        <div class="flex flex-col sm:flex-row justify-between items-start gap-4">
          <h1 class="text-3xl font-bold break-words">{{ location.name || 'Unnamed Location' }}</h1>
          <div class="flex gap-2 w-full sm:w-auto">
            <router-link :to="`/locations/${location.id}/edit`" class="btn btn-primary flex-1 sm:flex-initial">
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
              <dd class="mt-1 text-sm">{{ location.name || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Description</dt>
              <dd class="mt-1 text-sm">{{ location.description || '-' }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Code</dt>
              <dd class="mt-1">
                <code v-if="location.code" class="text-sm">{{ location.code }}</code>
                <span v-else class="text-sm">-</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Type</dt>
              <dd class="mt-1">
                <span v-if="location.expand?.type" class="badge badge-ghost">
                  {{ location.expand.type.name }}
                </span>
                <span v-else class="text-sm">-</span>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Parent Location</dt>
              <dd class="mt-1">
                <router-link 
                  v-if="location.expand?.parent" 
                  :to="`/locations/${location.parent}`"
                  class="link link-hover"
                >
                  {{ location.expand.parent.name }}
                </router-link>
                <span v-else class="text-sm">-</span>
              </dd>
            </div>
          </dl>
        </BaseCard>
        
        <!-- Coordinates -->
        <BaseCard title="Geo Coordinates">
          <dl class="space-y-4">
            <div v-if="location.coordinates">
              <dt class="text-sm font-medium text-base-content/70">Latitude</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ location.coordinates.lat }}</code>
              </dd>
            </div>
            <div v-if="location.coordinates">
              <dt class="text-sm font-medium text-base-content/70">Longitude</dt>
              <dd class="mt-1">
                <code class="text-sm">{{ location.coordinates.lon }}</code>
              </dd>
            </div>
            <div v-if="!location.coordinates" class="text-sm text-base-content/40">
              No coordinates set
            </div>
          </dl>
        </BaseCard>
        
        <!-- Floorplan -->
        <BaseCard v-if="getFloorplanUrl()" title="Floorplan" class="lg:col-span-2">
          <div class="overflow-hidden rounded-lg border border-base-300">
            <img 
              :src="getFloorplanUrl()!" 
              :alt="`Floorplan for ${location.name}`"
              class="w-full h-auto"
            />
          </div>
        </BaseCard>
        
        <!-- Metadata -->
        <BaseCard v-if="location.metadata" title="Metadata">
          <pre class="text-xs bg-base-200 p-4 rounded overflow-x-auto">{{ JSON.stringify(location.metadata, null, 2) }}</pre>
        </BaseCard>
        
        <!-- System Info -->
        <BaseCard title="System Information">
          <dl class="space-y-4">
            <div>
              <dt class="text-sm font-medium text-base-content/70">ID</dt>
              <dd class="mt-1">
                <code class="text-xs">{{ location.id }}</code>
              </dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Created</dt>
              <dd class="mt-1 text-sm">{{ formatDate(location.created) }}</dd>
            </div>
            <div>
              <dt class="text-sm font-medium text-base-content/70">Last Updated</dt>
              <dd class="mt-1 text-sm">{{ formatDate(location.updated) }}</dd>
            </div>
          </dl>
        </BaseCard>
      </div>
    </template>
  </div>
</template>
