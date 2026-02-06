<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { LocationType } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()

// Pagination
const {
  items,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<LocationType>('location_types', 20)

const searchQuery = ref('')

const filteredItems = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return items.value

  return items.value.filter(item => {
    if (item.name?.toLowerCase().includes(query)) return true
    if (item.description?.toLowerCase().includes(query)) return true
    if (item.code?.toLowerCase().includes(query)) return true
    return false
  })
})

const columns: Column<LocationType>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'description', label: 'Description', mobileLabel: 'Desc', class: 'hidden md:table-cell' },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (val) => formatDate(val, 'PP')
  },
]

async function loadData() {
  await load({ sort: 'name' })
}

function handleRowClick(item: LocationType) {
  router.push(`/locations/types/${item.id}/edit`)
}

async function handleDelete(item: LocationType) {
  const confirmed = await confirm({
    title: 'Delete Location Type',
    message: `Are you sure you want to delete "${item.name}"?`,
    details: 'Locations using this type will not be deleted but will lose their type reference.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return
  try {
    await pb.collection('location_types').delete(item.id)
    toast.success('Deleted')
    loadData()
  } catch (err: any) {
    toast.error(err.message)
  }
}

function handleOrgChange() {
  searchQuery.value = ''
  loadData()
}

onMounted(() => {
  loadData()
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
        <h1 class="text-3xl font-bold">Location Types</h1>
        <p class="text-base-content/70 mt-1">Classify your physical sites</p>
      </div>
      <router-link to="/locations/types/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Type</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search location types by name, code, or description..." class="input input-bordered w-full" />
    </div>

    <!-- Loading State -->
    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load location types</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📍</span>
        <h3 class="text-xl font-bold mt-4">No location types found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first location type to classify sites
        </p>
        <router-link to="/locations/types/new" class="btn btn-primary mt-4">
          Create Location Type
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="filteredItems.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching location types</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="filteredItems"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-code="{ item }">
          <code v-if="item.code" class="bg-base-200 px-1 rounded text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/locations/types/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1 sm:flex-initial">Delete</button>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ items.length }} of {{ totalItems }} location types
        </span>
        <div class="join">
          <button
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage({ sort: 'name' })"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage({ sort: 'name' })"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
