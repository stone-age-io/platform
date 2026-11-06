<script setup lang="ts">
import { SearchX, UserIcon } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import type { NatsUser } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import ExpiryBadge from '@/components/common/ExpiryBadge.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'

const router = useRouter()

const {
  items: users,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsUser>('nats_users', 20)

// Server-side, so a match beyond the current page is found.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['nats_username', 'description', 'account_id.name', 'role_id.name'],
  () => {
    page.value = 1
    loadUsers()
  },
)

const sort = ref('')

// Pass this to every usePagination call, the pager included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
  expand: 'account_id,role_id',
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadUsers()
}

// Any non-empty allow/deny list is an override merged with the role, so flag it.
function hasOverrides(user: NatsUser): boolean {
  const lists = [
    user.publish_permissions,
    user.subscribe_permissions,
    user.publish_deny_permissions,
    user.subscribe_deny_permissions,
  ]
  return lists.some((v) => {
    if (!v) return false
    if (Array.isArray(v)) return v.length > 0
    const t = String(v).trim()
    return t !== '' && t !== '[]'
  })
}

const columns: Column<NatsUser>[] = [
  {
    key: 'nats_username',
    sortable: 'nats_username',
    label: 'Username',
    mobileLabel: 'Username',
  },
  {
    key: 'expand.account_id.name',
    cardWide: true,
    sortable: 'account_id.name',
    label: 'Account',
    mobileLabel: 'Account',
  },
  {
    key: 'expand.role_id.name',
    cardWide: true,
    sortable: 'role_id.name',
    label: 'Role',
    mobileLabel: 'Role',
  },
  {
    key: 'active',
    label: 'Status',
    mobileLabel: 'Status',
    format: (value) => value ? 'Active' : 'Inactive',
  },
  {
    key: 'created',
    sortable: '-created',
    width: '8rem',
    label: 'Created',
    mobileLabel: 'Created',
    format: (value) => formatDate(value, 'PP'),
  },
]

async function loadUsers() {
  await load(queryOptions.value)
}

function handleRowClick(user: NatsUser) {
  router.push(`/nats/users/${user.id}`)
}

onMounted(() => {
  loadUsers()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">NATS Users</h1>
        <p class="text-base-content/70 mt-1">
          Manage NATS user credentials and access
        </p>
      </div>
      <router-link to="/nats/users/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New NATS User</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by username, account, role, or description..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && users.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && users.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load NATS users</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadUsers" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="users.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <UserIcon :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No NATS users found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first NATS user to get started
        </p>
        <router-link to="/nats/users/new" class="btn btn-primary mt-4">
          Create NATS User
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="users.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching users</h3>
        <button @click="searchQuery = ''" class="btn btn-ghost mt-4">
          Clear Search
        </button>
      </div>
    </BaseCard>
    
    <!-- Responsive List -->
    <BaseCard v-else :no-padding="true">
      <ResponsiveList 
        :sort="sort"
        @update:sort="onSort"
        :items="users" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-nats_username="{ item }">
          <div>
            <div class="font-medium font-mono flex items-center gap-2">
              {{ item.nats_username }}
              <span
                v-if="hasOverrides(item)"
                class="badge badge-xs badge-warning badge-outline"
                title="Has per-user permission overrides (merged with role)"
              >overrides</span>
            </div>
            <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <template #card-nats_username="{ item }">
          <div>
            <div class="font-semibold font-mono text-base flex items-center gap-2">
              {{ item.nats_username }}
              <span
                v-if="hasOverrides(item)"
                class="badge badge-xs badge-warning badge-outline"
                title="Has per-user permission overrides (merged with role)"
              >overrides</span>
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <!-- JWT expiry beside the badge: a credential about to expire fails like a revoked one. -->
        <template #cell-active="{ item }">
          <div class="flex flex-wrap items-center gap-1">
            <StatusBadge :active="item.active" />
            <ExpiryBadge :value="item.jwt_expires_at" size="sm" />
          </div>
        </template>

        <template #card-active="{ item }">
          <div class="flex flex-wrap items-center gap-1">
            <StatusBadge :active="item.active" />
            <ExpiryBadge :value="item.jwt_expires_at" size="sm" />
          </div>
        </template>

        <template #actions="{ item }">
          <router-link 
            :to="`/nats/users/${item.id}/edit`" 
            class="btn btn-xs"
          >
            Edit
          </router-link>
        </template>
      </ResponsiveList>
      
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="users.length"
        :total="totalItems"
        noun="users"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
