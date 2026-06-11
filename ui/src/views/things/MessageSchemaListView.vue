<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { MessageSchema } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()

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
} = usePagination<MessageSchema>('message_schemas', 20)

const searchQuery = ref('')

const deleting = ref(false)

const filteredItems = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return items.value
  return items.value.filter(s =>
    s.namespace?.toLowerCase().includes(query) ||
    s.name?.toLowerCase().includes(query) ||
    s.version?.toLowerCase().includes(query) ||
    s.description?.toLowerCase().includes(query)
  )
})

const columns: Column<MessageSchema>[] = [
  { key: 'identity', label: 'Schema', mobileLabel: 'Schema' },
  { key: 'version', label: 'Version', mobileLabel: 'Version' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (val) => formatDate(val, 'PP') },
]

async function loadData() {
  await load({ sort: 'namespace,name,-version' })
}

function handleRowClick(item: MessageSchema) {
  router.push(`/things/schemas/${item.id}/edit`)
}

async function handleDelete(item: MessageSchema) {
  const confirmed = await confirm({
    title: 'Delete Message Schema',
    message: `Are you sure you want to delete "${item.namespace}/${item.name}@${item.version}"?`,
    details: 'Operations still linking this schema will be left with a dangling reference.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return
  deleting.value = true
  try {
    await pb.collection('message_schemas').delete(item.id)
    toast.success('Deleted')
    loadData()
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    deleting.value = false
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
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Message Schemas</h1>
        <p class="text-base-content/70 mt-1">JSON Schemas that describe operation payloads</p>
      </div>
      <router-link to="/things/schemas/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Schema</span>
      </router-link>
    </div>

    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search schemas by namespace, name, version..." class="input input-bordered w-full" />
    </div>

    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load schemas</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📐</span>
        <h3 class="text-xl font-bold mt-4">No schemas found</h3>
        <p class="text-base-content/70 mt-2">Create your first message schema to describe operation payloads</p>
        <router-link to="/things/schemas/new" class="btn btn-primary mt-4">Create Schema</router-link>
      </div>
    </BaseCard>

    <BaseCard v-else-if="filteredItems.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching schemas</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
      </div>
    </BaseCard>

    <BaseCard v-else :no-padding="true">
      <ResponsiveList :items="filteredItems" :columns="columns" :loading="loading" @row-click="handleRowClick">
        <template #cell-identity="{ item }">
          <div>
            <code class="font-mono text-xs">{{ item.namespace }}/{{ item.name }}</code>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1 mt-1">{{ item.description }}</div>
          </div>
        </template>

        <template #cell-version="{ item }">
          <span class="badge badge-sm badge-outline">{{ item.version }}</span>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/things/schemas/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1 sm:flex-initial" :disabled="deleting">Delete</button>
        </template>
      </ResponsiveList>

      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ items.length }} of {{ totalItems }} schemas
        </span>
        <div class="join">
          <button class="join-item btn btn-sm" :disabled="page === 1 || loading" @click="prevPage({ sort: 'namespace,name,-version' })">«</button>
          <button class="join-item btn btn-sm">{{ page }} / {{ totalPages }}</button>
          <button class="join-item btn btn-sm" :disabled="page === totalPages || loading" @click="nextPage({ sort: 'namespace,name,-version' })">»</button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
