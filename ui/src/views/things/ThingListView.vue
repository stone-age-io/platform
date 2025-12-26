<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

// Pagination
const {
  items: things,
  page,
  totalPages,
  totalItems,
  loading,
  load,
  nextPage,
  prevPage,
} = usePagination<Thing>('things', 20)

// Search
const searchQuery = ref('')

// Computed filter
const filter = computed(() => {
  if (!searchQuery.value.trim()) return undefined
  return `name ~ "${searchQuery.value}"`
})

// Column configuration for responsive list
const columns: Column<Thing>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'expand.type.name',
    label: 'Type',
    mobileLabel: 'Type',
  },
  {
    key: 'expand.location.name',
    label: 'Location',
    mobileLabel: 'Location',
  },
  {
    key: 'code',
    label: 'Code',
    mobileLabel: 'Code',
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

/**
 * Load things from API
 * Backend automatically filters by current organization
 */
async function loadThings() {
  await load(filter.value, '-created', 'type,location')
}

/**
 * Handle search input
 */
function handleSearch() {
  loadThings()
}

/**
 * Handle delete
 */
async function handleDelete(thing: Thing) {
  if (!confirm(`Delete "${thing.name}"? This cannot be undone.`)) return
  
  try {
    await pb.collection('things').delete(thing.id)
    toast.success('Thing deleted')
    loadThings()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete thing')
  }
}

/**
 * Handle organization change
 */
function handleOrgChange() {
  loadThings()
}

onMounted(() => {
  loadThings()
  window.addEventListener('organization-changed', handleOrgChange)
})

onUnmounted(() => {
  window.removeEventListener('organization-changed', handleOrgChange)
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Things</h1>
        <p class="text-base-content/70 mt-1">
          Manage IoT devices and sensors
        </p>
      </div>
      <router-link to="/things/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Thing</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        @input="handleSearch"
        type="text"
        placeholder="Search things..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && things.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>
    
    <!-- Empty State -->
    <BaseCard v-else-if="things.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📦</span>
        <h3 class="text-xl font-bold mt-4">No things found</h3>
        <p class="text-base-content/70 mt-2">
          {{ searchQuery ? 'Try a different search term' : 'Create your first thing to get started' }}
        </p>
        <router-link v-if="!searchQuery" to="/things/new" class="btn btn-primary mt-4">
          Create Thing
        </router-link>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList :items="things" :columns="columns" :loading="loading">
        <!-- Custom cell for name (with description) -->
        <template #cell-name="{ item }">
          <router-link 
            :to="`/things/${item.id}`" 
            class="link link-hover font-medium"
          >
            {{ item.name || 'Unnamed' }}
          </router-link>
          <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
            {{ item.description }}
          </div>
        </template>
        
        <!-- Custom mobile card for name (make it prominent) -->
        <template #card-name="{ item }">
          <div>
            <router-link 
              :to="`/things/${item.id}`" 
              class="link link-hover font-semibold text-base"
            >
              {{ item.name || 'Unnamed' }}
            </router-link>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- Custom cell for type (badge) -->
        <template #cell-expand.type.name="{ item }">
          <span v-if="item.expand?.type" class="badge badge-ghost">
            {{ item.expand.type.name }}
          </span>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Custom card for type (badge) -->
        <template #card-expand.type.name="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Type</span>
            <div class="mt-1">
              <span v-if="item.expand?.type" class="badge badge-ghost badge-sm">
                {{ item.expand.type.name }}
              </span>
              <span v-else class="text-sm text-base-content/40">-</span>
            </div>
          </div>
        </template>
        
        <!-- Custom cell for code (mono font) -->
        <template #cell-code="{ item }">
          <code v-if="item.code" class="text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <router-link 
            :to="`/things/${item.id}/edit`" 
            class="btn btn-ghost btn-sm flex-1 sm:flex-initial"
          >
            Edit
          </router-link>
          <button 
            @click="handleDelete(item)" 
            class="btn btn-ghost btn-sm text-error flex-1 sm:flex-initial"
          >
            Delete
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
      <div class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ things.length }} of {{ totalItems }} things
        </span>
        <div class="join">
          <button 
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage(filter, '-created', 'type,location')"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button 
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage(filter, '-created', 'type,location')"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
