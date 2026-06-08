<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { LeafNode } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()

const {
  items: leafNodes,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<LeafNode>('leaf_nodes', 20)

const searchQuery = ref('')

const filteredLeafNodes = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return leafNodes.value
  return leafNodes.value.filter(node =>
    node.name?.toLowerCase().includes(query) ||
    node.code?.toLowerCase().includes(query) ||
    node.domain?.toLowerCase().includes(query)
  )
})

const columns: Column<LeafNode>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
  { key: 'domain', label: 'Domain', mobileLabel: 'Domain' },
  { key: 'synced_collections', label: 'Synced', mobileLabel: 'Synced' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (value) => formatDate(value, 'PP') },
]

async function loadLeafNodes() {
  // Backend filters by current organization via API rules.
  await load({ expand: 'location,nats_user' })
}

function handleRowClick(node: LeafNode) {
  router.push(`/leaf-nodes/${node.id}`)
}

async function handleDelete(node: LeafNode) {
  const confirmed = await confirm({
    title: 'Delete Leaf Node',
    message: `Are you sure you want to delete "${node.name || node.code}"?`,
    details: 'This removes the edge node record. Its NATS user is not deleted automatically.',
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return

  try {
    await pb.collection('leaf_nodes').delete(node.id)
    toast.success('Leaf node deleted')
    loadLeafNodes()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete leaf node')
  }
}

function handleOrgChange() {
  searchQuery.value = ''
  loadLeafNodes()
}

onMounted(() => {
  loadLeafNodes()
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
        <h1 class="text-3xl font-bold">Leaf Nodes</h1>
        <p class="text-base-content/70 mt-1">
          Edge nodes that mirror this organization's config into local NATS
        </p>
      </div>
      <router-link to="/leaf-nodes/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Leaf Node</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search leaf nodes by name, code, or domain..."
        class="input input-bordered w-full"
      />
    </div>

    <!-- Loading State -->
    <div v-if="loading && leafNodes.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && leafNodes.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load leaf nodes</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadLeafNodes" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="leafNodes.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🍃</span>
        <h3 class="text-xl font-bold mt-4">No leaf nodes yet</h3>
        <p class="text-base-content/70 mt-2">
          Create your first edge node to get started
        </p>
        <router-link to="/leaf-nodes/new" class="btn btn-primary mt-4">
          Create Leaf Node
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="filteredLeafNodes.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching leaf nodes</h3>
        <p class="text-base-content/70 mt-2">Try a different search term</p>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">Clear Search</button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="filteredLeafNodes"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium">{{ item.name || 'Unnamed' }}</div>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #cell-code="{ item }">
          <code v-if="item.code" class="text-xs">{{ item.code }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>

        <template #cell-domain="{ item }">
          <code v-if="item.domain" class="text-xs">{{ item.domain }}</code>
          <span v-else class="text-base-content/40">-</span>
        </template>

        <template #cell-synced_collections="{ item }">
          <span class="badge badge-ghost badge-sm">
            {{ item.synced_collections?.length || 0 }}
          </span>
        </template>

        <template #actions="{ item }">
          <router-link :to="`/leaf-nodes/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial">
            Edit
          </router-link>
          <button @click="handleDelete(item)" class="btn btn-xs text-error flex-1 sm:flex-initial">
            Delete
          </button>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ leafNodes.length }} of {{ totalItems }} leaf nodes
        </span>
        <div class="join">
          <button class="join-item btn btn-sm" :disabled="page === 1 || loading" @click="prevPage({ expand: 'location,nats_user' })">«</button>
          <button class="join-item btn btn-sm">{{ page }} / {{ totalPages }}</button>
          <button class="join-item btn btn-sm" :disabled="page === totalPages || loading" @click="nextPage({ expand: 'location,nats_user' })">»</button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
