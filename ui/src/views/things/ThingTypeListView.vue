<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { ThingType } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()

const { items, page, totalPages, totalItems, loading, load, nextPage, prevPage } = usePagination<ThingType>('thing_types', 20)
const searchQuery = ref('')

const filteredItems = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return items.value
  return items.value.filter(item => 
    item.name?.toLowerCase().includes(query) || 
    item.code?.toLowerCase().includes(query)
  )
})

const columns: Column<ThingType>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'capabilities', label: 'Capabilities', mobileLabel: 'Caps' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (val) => formatDate(val, 'PP') },
]

async function loadData() { await load({ sort: 'name' }) }
function handleRowClick(item: ThingType) { router.push(`/things/types/${item.id}/edit`) }
async function handleDelete(item: ThingType) {
  const confirmed = await confirm({
    title: 'Delete Thing Type',
    message: `Are you sure you want to delete "${item.name}"?`,
    details: 'Things using this type will not be deleted but will lose their type reference.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return
  try {
    await pb.collection('thing_types').delete(item.id)
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
        <h1 class="text-3xl font-bold">Thing Types</h1>
        <p class="text-base-content/70 mt-1">Classify your devices</p>
      </div>
      <router-link to="/things/types/new" class="btn btn-primary">+ New Type</router-link>
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
        
        <template #cell-capabilities="{ item }">
          <div class="flex flex-wrap gap-1">
            <span v-for="cap in item.capabilities" :key="cap" class="badge badge-sm badge-outline">
              {{ cap }}
            </span>
            <span v-if="!item.capabilities?.length" class="opacity-50 text-xs">-</span>
          </div>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/things/types/${item.id}/edit`" class="btn btn-xs flex-1">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1">Delete</button>
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
