<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { LocationType } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

const {
  items,
  page,
  totalPages,
  totalItems,
  loading,
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
  if (!confirm(`Delete type "${item.name}"?`)) return
  try {
    await pb.collection('location_types').delete(item.id)
    toast.success('Deleted')
    loadData()
  } catch (err: any) {
    toast.error(err.message)
  }
}

function handleOrgChange() {
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
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Location Types</h1>
        <p class="text-base-content/70 mt-1">Classify your physical sites</p>
      </div>
      <router-link to="/locations/types/new" class="btn btn-primary">
        + New Type
      </router-link>
    </div>

    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search..." class="input input-bordered" />
    </div>

    <BaseCard :no-padding="true">
      <ResponsiveList 
        :items="filteredItems" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-code="{ item }">
          <code v-if="item.code" class="bg-base-200 px-1 rounded text-xs">{{ item.code }}</code>
          <span v-else class="opacity-50">-</span>
        </template>
        
        <template #actions="{ item }">
          <router-link :to="`/locations/types/${item.id}/edit`" class="btn btn-xs flex-1">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1">Delete</button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination Controls (Generic) -->
      <div v-if="!searchQuery" class="flex justify-between items-center p-4 border-t border-base-300">
        <span class="text-sm opacity-70">Showing {{ items.length }} of {{ totalItems }}</span>
        <div class="join">
          <button class="join-item btn btn-sm" :disabled="page === 1" @click="prevPage()">«</button>
          <button class="join-item btn btn-sm">Page {{ page }}</button>
          <button class="join-item btn btn-sm" :disabled="page === totalPages" @click="nextPage()">»</button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
