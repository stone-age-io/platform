<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { NatsAccountImport } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const router = useRouter()
const toast = useToast()
const { confirm } = useConfirm()

const {
  items: imports,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsAccountImport>('nats_account_imports', 20)

const searchQuery = ref('')

const filteredImports = computed(() => {
  const query = searchQuery.value.toLowerCase().trim()
  if (!query) return imports.value
  return imports.value.filter(imp => {
    if (imp.name?.toLowerCase().includes(query)) return true
    if (imp.subject?.toLowerCase().includes(query)) return true
    if (imp.account?.toLowerCase().includes(query)) return true
    if (imp.description?.toLowerCase().includes(query)) return true
    return false
  })
})

function truncateKey(key: string): string {
  if (!key || key.length <= 16) return key
  return key.substring(0, 8) + '...' + key.substring(key.length - 8)
}

const columns: Column<NatsAccountImport>[] = [
  {
    key: 'name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'subject',
    label: 'Subject',
    mobileLabel: 'Subject',
  },
  {
    key: 'type',
    label: 'Type',
    mobileLabel: 'Type',
  },
  {
    key: 'account',
    label: 'Source Account',
    mobileLabel: 'Source',
    format: (value) => truncateKey(value),
  },
  {
    key: 'created',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

async function loadImports() {
  await load()
}

function handleRowClick(imp: NatsAccountImport) {
  router.push(`/nats/imports/${imp.id}`)
}

async function handleDelete(imp: NatsAccountImport) {
  const confirmed = await confirm({
    title: 'Delete Import',
    message: `Are you sure you want to delete "${imp.name}"?`,
    details: 'This account will lose access to the imported subject.',
    confirmText: 'Delete',
    variant: 'danger'
  })
  if (!confirmed) return

  try {
    await pb.collection('nats_account_imports').delete(imp.id)
    toast.success('Import deleted')
    loadImports()
  } catch (err: any) {
    toast.error(err.message || 'Failed to delete import')
  }
}

function handleOrgChange() {
  searchQuery.value = ''
  loadImports()
}

onMounted(() => {
  loadImports()
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
        <h1 class="text-3xl font-bold">NATS Imports</h1>
        <p class="text-base-content/70 mt-1">
          Subscribe to subjects from other accounts
        </p>
      </div>
      <router-link to="/nats/imports/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Import</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="Search imports by name, subject, account, or description..."
        class="input input-bordered w-full"
      />
    </div>

    <!-- Loading State -->
    <div v-if="loading && imports.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && imports.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load imports</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadImports" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="imports.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">📥</span>
        <h3 class="text-xl font-bold mt-4">No imports found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first import to subscribe to subjects from other accounts
        </p>
        <router-link to="/nats/imports/new" class="btn btn-primary mt-4">
          Create Import
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="filteredImports.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching imports</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList
        :items="filteredImports"
        :columns="columns"
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium">{{ item.name }}</div>
            <div v-if="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base">{{ item.name }}</div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1">
              {{ item.description }}
            </div>
          </div>
        </template>

        <template #cell-subject="{ item }">
          <code class="text-xs">{{ item.subject }}</code>
        </template>

        <template #cell-type="{ item }">
          <span
            class="badge badge-sm"
            :class="item.type === 'stream' ? 'badge-info' : 'badge-success'"
          >
            {{ item.type }}
          </span>
        </template>

        <template #cell-account="{ item }">
          <code class="text-xs">{{ truncateKey(item.account) }}</code>
        </template>

        <!-- Actions -->
        <template #actions="{ item }">
          <router-link
            :to="`/nats/imports/${item.id}/edit`"
            class="btn btn-xs flex-1 sm:flex-initial"
          >
            Edit
          </router-link>
          <button
            @click="handleDelete(item)"
            class="btn btn-xs text-error flex-1 sm:flex-initial"
          >
            Delete
          </button>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ imports.length }} of {{ totalItems }} imports
        </span>
        <div class="join">
          <button
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage()"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage()"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
