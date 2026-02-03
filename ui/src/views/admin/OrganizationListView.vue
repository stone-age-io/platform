<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
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

const { items, loading, load, nextPage, prevPage, page, totalPages, totalItems } = usePagination<OrganizationWithExpand>('organizations', 20)
const searchQuery = ref('')

const filteredItems = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return items.value
  return items.value.filter(i =>
    i.name.toLowerCase().includes(q) ||
    i.expand?.owner?.email?.toLowerCase().includes(q)
  )
})

const columns: Column<OrganizationWithExpand>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.owner.email', label: 'Owner', mobileLabel: 'Owner' },
  { key: 'active', label: 'Status', mobileLabel: 'Status' },
  { key: 'created', label: 'Created', mobileLabel: 'Created', format: (v) => formatDate(v, 'PP') },
]

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

  try {
    await pb.collection('organizations').delete(item.id)
    toast.success('Organization deleted')
    load({ sort: 'name', expand: 'owner' })
  } catch (err: any) {
    toast.error(err.message)
  }
}

onMounted(() => load({ sort: 'name', expand: 'owner' }))
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold">Organizations</h1>
        <p class="text-base-content/70">Manage platform organizations</p>
      </div>
      <router-link to="/organizations/new" class="btn btn-primary">+ New Organization</router-link>
    </div>

    <div class="form-control">
      <input v-model="searchQuery" type="text" placeholder="Search by name or owner..." class="input input-bordered" />
    </div>

    <BaseCard :no-padding="true">
      <ResponsiveList :items="filteredItems" :columns="columns" :loading="loading" @row-click="handleRowClick">
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
          <router-link :to="`/organizations/${item.id}/edit`" class="btn btn-ghost btn-sm">Edit</router-link>
          <button @click.stop="handleDelete(item)" class="btn btn-ghost btn-sm text-error">Delete</button>
        </template>
      </ResponsiveList>

      <!-- Pagination -->
      <div v-if="!searchQuery" class="flex justify-between items-center p-4 border-t border-base-300">
        <span class="text-sm opacity-70">Showing {{ items.length }} of {{ totalItems }}</span>
        <div class="join">
          <button class="join-item btn btn-sm" :disabled="page === 1" @click="prevPage({ sort: 'name', expand: 'owner' })">«</button>
          <button class="join-item btn btn-sm">Page {{ page }}</button>
          <button class="join-item btn btn-sm" :disabled="page === totalPages" @click="nextPage({ sort: 'name', expand: 'owner' })">»</button>
        </div>
      </div>
    </BaseCard>
  </div>
</template>
