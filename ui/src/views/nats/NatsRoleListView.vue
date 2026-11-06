<script setup lang="ts">
import { SearchX, Shield } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { formatDate } from '@/utils/format'
import type { NatsRole } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

const router = useRouter()

const {
  items: roles,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<NatsRole>('nats_roles', 20)

// Server-side, so a match beyond the current page is found.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['name', 'description'],
  () => {
    page.value = 1
    loadRoles()
  },
)

const sort = ref('')

// Pass this to every usePagination call, the pager included, or page two
// loses the filter and sort.
const queryOptions = computed(() => ({
  filter: searchFilter.value,
  sort: sort.value,
}))

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadRoles()
}

const columns: Column<NatsRole>[] = [
  // 'auto', not omitted: omitting it means 28% for column 0 (see Column.width).
  // The only free-text column, so it takes the slack.
  {
    key: 'name',
    sortable: 'name',
    width: 'auto',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'is_default',
    width: '7rem',
    label: 'Default',
    mobileLabel: 'Default',
    format: (value) => value ? 'Yes' : 'No',
  },
  {
    key: 'max_subscriptions',
    sortable: 'max_subscriptions',
    width: '10rem',
    label: 'Max Subscriptions',
    mobileLabel: 'Max Subs',
    format: (value) => value === -1 ? 'Unlimited' : value?.toString() || '0',
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

async function loadRoles() {
  await load(queryOptions.value)
}

function handleRowClick(role: NatsRole) {
  router.push(`/nats/roles/${role.id}`)
}

onMounted(() => {
  loadRoles()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">NATS Roles</h1>
        <p class="text-base-content/70 mt-1">
          Define permissions for NATS users
        </p>
      </div>
      <router-link to="/nats/roles/new" class="btn btn-primary w-full sm:w-auto">
        <span class="text-lg">+</span>
        <span>New Role</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by name or description..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && roles.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && roles.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load roles</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadRoles" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="roles.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <Shield :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No roles found</h3>
        <p class="text-base-content/70 mt-2">
          Create your first NATS role to get started
        </p>
        <router-link to="/nats/roles/new" class="btn btn-primary mt-4">
          Create Role
        </router-link>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="roles.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching roles</h3>
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
        :items="roles" 
        :columns="columns" 
        :loading="loading"
        @row-click="handleRowClick"
      >
        <template #cell-name="{ item }">
          <div>
            <div class="font-medium">
              {{ item.name }}
            </div>
            <div v-if="item.description" :title="item.description" class="text-sm text-base-content/60 line-clamp-1">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <template #card-name="{ item }">
          <div>
            <div class="font-semibold text-base">
              {{ item.name }}
            </div>
            <div v-if="item.description" class="text-sm text-base-content/60 mt-1 line-clamp-2">
              {{ item.description }}
            </div>
          </div>
        </template>
        
        <template #cell-is_default="{ item }">
          <span v-if="item.is_default" class="badge badge-primary badge-sm">
            Default
          </span>
          <span v-else class="text-base-content/40">—</span>
        </template>
        
        <template #actions="{ item }">
          <router-link 
            :to="`/nats/roles/${item.id}/edit`" 
            class="btn btn-xs"
          >
            Edit
          </router-link>
        </template>
      </ResponsiveList>
      
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="roles.length"
        :total="totalItems"
        noun="roles"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
