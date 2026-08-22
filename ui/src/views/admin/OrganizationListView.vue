<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { pb } from '@/utils/pb'
import { formatDate } from '@/utils/format'
import type { Organization, User } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

interface OrganizationWithExpand extends Organization {
  expand?: {
    owner?: User
  }
}

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
} = usePagination<OrganizationWithExpand>('organizations', 20)

// Search runs on the SERVER. The client-side pass this replaces filtered the
// twenty records already on screen, so a match on page three answered "No
// results found" -- which is not the same claim at all.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'description', 'owner.email', 'owner.name'],
  () => {
    page.value = 1
    loadData()
  },
)

// One options object, passed to EVERY call into usePagination, the pager
// buttons included -- passing only expand/sort there drops the filter and
// page two comes back unfiltered.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: 'name',
  expand: 'owner',
}))

const deleting = ref(false)

const columns: Column<OrganizationWithExpand>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.owner.email', label: 'Owner', mobileLabel: 'Owner' },
  { key: 'active', label: 'Status', mobileLabel: 'Status' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (v) => formatDate(v, 'PP') },
]

async function loadData() {
  await load(queryOptions.value)
}

function handleRowClick(item: OrganizationWithExpand) {
  router.push(`/organizations/${item.id}`)
}

async function handleDelete(item: OrganizationWithExpand) {
  const confirmed = await confirm({
    title: 'Delete Organization',
    message: `Are you sure you want to delete "${item.name}"?`,
    details: 'This will delete ALL data associated with this organization including users, things, locations, and configurations. This action cannot be undone.',
    confirmText: 'Delete Organization',
    variant: 'danger'
  })
  if (!confirmed) return

  deleting.value = true
  try {
    await pb.collection('organizations').delete(item.id)
    toast.success('Organization deleted')
    loadData()
  } catch (err: any) {
    toast.error(err.message)
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Organizations</h1>
        <p class="text-base-content/70 mt-1">Manage platform organizations</p>
      </div>
      <router-link to="/organizations/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Organization</span>
      </router-link>
    </div>

    <!-- Search -->
    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search by name or owner..." class="input input-bordered w-full" />
    </div>

    <!-- Loading State -->
    <div v-if="loading && items.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load organizations</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadData" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="items.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <span class="text-6xl">🏢</span>
        <h3 class="text-xl font-bold mt-4">No organizations found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first organization to get started
        </p>
        <router-link to="/organizations/new" class="btn btn-primary mt-4">
          Create Organization
        </router-link>
      </div>
    </BaseCard>

    <!-- No Search Results -->
    <BaseCard v-else-if="items.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
        <h3 class="text-xl font-bold mt-4">No matching organizations</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>

    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList :items="items" :columns="columns" :loading="loading" @row-click="handleRowClick">
        <!-- Owner cell -->
        <template #cell-expand.owner.email="{ item }">
          <span class="text-sm">
            {{ item.expand?.owner?.email || '—' }}
          </span>
        </template>

        <!-- Owner mobile card -->
        <template #card-expand.owner.email="{ item }">
          <div class="flex flex-col">
            <span class="text-xs font-medium text-base-content/70">Owner</span>
            <span>{{ item.expand?.owner?.email || '—' }}</span>
          </div>
        </template>

        <!-- Status cell -->
        <template #cell-active="{ item }">
          <span class="badge" :class="item.active ? 'badge-success' : 'badge-error'">
            {{ item.active ? 'Active' : 'Inactive' }}
          </span>
        </template>

        <!-- Actions -->
        <template #actions="{ item }">
          <router-link :to="`/organizations/${item.id}/edit`" class="btn btn-xs flex-1 sm:flex-initial">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-xs text-error flex-1 sm:flex-initial" :disabled="deleting">Delete</button>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <div class="flex flex-col sm:flex-row justify-between items-center gap-4 p-4 border-t border-base-300">
        <span class="text-sm text-base-content/70 text-center sm:text-left">
          Showing {{ items.length }} of {{ totalItems }} organizations
        </span>
        <div class="join">
          <button
            class="join-item btn btn-sm"
            :disabled="page === 1 || loading"
            @click="prevPage(queryOptions)"
          >
            «
          </button>
          <button class="join-item btn btn-sm">
            {{ page }} / {{ totalPages }}
          </button>
          <button
            class="join-item btn btn-sm"
            :disabled="page === totalPages || loading"
            @click="nextPage(queryOptions)"
          >
            »
          </button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
