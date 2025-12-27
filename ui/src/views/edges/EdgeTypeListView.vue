<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { EdgeType } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()

const { items, page, totalPages, totalItems, loading, load, nextPage, prevPage } = usePagination<EdgeType>('edge_types', 20)
const searchQuery = ref('')

const filteredItems = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return items.value
  return items.value.filter(item => 
    item.name?.toLowerCase().includes(query) || 
    item.code?.toLowerCase().includes(query)
  )
})

const columns: Column<EdgeType>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (val) => formatDate(val, 'PP') },
]

async function loadData() { await load({ sort: 'name' }) }
function handleRowClick(item: EdgeType) { router.push(`/edges/types/${item.id}/edit`) }
async function handleDelete(item: EdgeType) {
  if (!confirm(`Delete type "${item.name}"?`)) return
  try {
    await pb.collection('edge_types').delete(item.id)
    toast.success('Deleted')
    loadData()
  } catch (err: any) { toast.error(err.message) }
}
function handleOrgChange() { loadData() }

onMounted(() => {
  loadData()
  window.addEventListener('organization-changed', handleOrgChange)
})
onUnmounted(() => window.removeEventListener('organization-changed', handleOrgChange))
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Edge Types</h1>
        <p class="text-base-content/70 mt-1">Classify your edge gateways</p>
      </div>
      <router-link to="/edges/types/new" class="btn btn-primary">+ New Type</router-link>
    </div>

    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search..." class="input input-bordered" />
    </div>

    <BaseCard :no-padding="true">
      <ResponsiveList :items="filteredItems" :columns="columns" :loading="loading" @row-click="handleRowClick">
        <template #cell-code="{ item }">
          <code v-if="item.code" class="bg-base-200 px-1 rounded text-xs">{{ item.code }}</code>
          <span v-else class="opacity-50">-</span>
        </template>
        <template #actions="{ item }">
          <router-link :to="`/edges/types/${item.id}/edit`" class="btn btn-ghost btn-sm">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-ghost btn-sm text-error">Delete</button>
        </template>
      </ResponsiveList>
      
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
