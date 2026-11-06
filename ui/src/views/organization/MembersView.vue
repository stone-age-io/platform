<script setup lang="ts">
import { Mail, SearchX, Users } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { useAuthStore } from '@/stores/auth'
import type { Membership } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'
import UserAvatar from '@/components/common/UserAvatar.vue'

const router = useRouter()
const authStore = useAuthStore()

const {
  items: members,
  page,
  totalPages,
  totalItems,
  loading,
  error,
  load,
  nextPage,
  prevPage,
} = usePagination<Membership>('memberships', 20)

// Server-side, so a match beyond the current page is found.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['user.name', 'user.email', 'role'],
  () => {
    page.value = 1
    loadMembers()
  },
)

const sort = ref('role,-created')

// Pass this to every usePagination call, the pager included, or page two
// loses the filter and sort. The org clause is ANDed, never replaced: an
// operator's memberships listRule spans organizations.
const queryOptions = computed(() => {
  const org = `organization = "${authStore.currentOrgId}"`
  return {
    filter: searchFilter.value ? `(${org}) && (${searchFilter.value})` : org,
    expand: 'user,invited_by,organization',
    sort: sort.value,
  }
})

function onSort(next: string) {
  sort.value = next
  page.value = 1
  loadMembers()
}

const columns: Column<Membership>[] = [
  {
    key: 'expand.user.name',
    sortable: 'user.name',
    label: 'Name',
    mobileLabel: 'Name',
  },
  {
    key: 'expand.user.email',
    cardWide: true,
    sortable: 'user.email',
    label: 'Email',
    mobileLabel: 'Email',
  },
  {
    key: 'role',
    sortable: 'role',
    width: '7rem',
    label: 'Role',
    mobileLabel: 'Role',
    format: (value) => value.charAt(0).toUpperCase() + value.slice(1),
  },
]

async function loadMembers() {
  if (!authStore.currentOrgId) return

  await load(queryOptions.value)
}

function handleRowClick(member: Membership) {
  router.push(`/organization/members/${member.id}`)
}

function getRoleBadgeClass(role: string): string {
  switch (role) {
    case 'owner': return 'badge-primary'
    case 'admin': return 'badge-secondary'
    case 'member': return 'badge-ghost'
    case 'viewer': return 'badge-outline'
    default: return 'badge-ghost'
  }
}

onMounted(() => {
  loadMembers()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
      <div>
        <h1 class="text-3xl font-bold">Team Members</h1>
        <p class="text-base-content/70 mt-1">
          Manage members of {{ authStore.currentOrg?.name }}
        </p>
      </div>
      <router-link 
        to="/organization/invitations" 
        class="btn btn-primary w-full sm:w-auto"
      >
        <Mail :size="18" class="inline-block shrink-0" />
        <span>Send Invitation</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search by name, email, or role..."
        class="input input-bordered w-full"
      />
    </div>
    
    <!-- Loading State -->
    <div v-if="loading && members.length === 0" class="flex justify-center p-12">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <!-- Error State -->
    <BaseCard v-else-if="error && members.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">&#9888;</span>
        <h3 class="text-xl font-bold mt-4">Failed to load members</h3>
        <p class="text-base-content/70 mt-2">{{ error }}</p>
        <button @click="loadMembers" class="btn btn-primary mt-4">Retry</button>
      </div>
    </BaseCard>

    <!-- Empty State -->
    <BaseCard v-else-if="members.length === 0 && !searchQuery">
      <div class="text-center py-12">
        <Users :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No members found</h3>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="members.length === 0">
      <div class="text-center py-12">
        <SearchX :size="48" :stroke-width="1.5" class="mx-auto text-base-content/30" />
        <h3 class="text-xl font-bold mt-4">No matching members</h3>
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
        :items="members" 
        :columns="columns" 
        :loading="loading"
        :clickable="true"
        @row-click="handleRowClick"
      >
        <template #cell-expand.user.name="{ item }">
          <div class="flex items-center gap-3">
            <UserAvatar :user="item.expand?.user" :size="40" />
            <div>
              <div class="font-medium">
                {{ item.expand?.user?.name || 'Unknown User' }}
              </div>
              <div v-if="item.user === authStore.user?.id" class="text-xs text-base-content/60">
                (You)
              </div>
            </div>
          </div>
        </template>
        
        <template #card-expand.user.name="{ item }">
          <div class="flex items-center gap-3">
            <UserAvatar :user="item.expand?.user" :size="48" />
            <div>
              <div class="font-semibold text-base">
                {{ item.expand?.user?.name || 'Unknown User' }}
              </div>
              <div v-if="item.user === authStore.user?.id" class="text-xs text-base-content/60">
                (You)
              </div>
            </div>
          </div>
        </template>
        
        <template #cell-expand.user.email="{ item }">
          <span class="text-base-content/70 text-sm">{{ item.expand?.user?.email }}</span>
        </template>
        
        <template #cell-role="{ item }">
          <span 
            class="badge badge-sm"
            :class="getRoleBadgeClass(item.role)"
          >
            {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
          </span>
        </template>
        
        <template #actions="{ item }">
          <button 
            @click.stop="handleRowClick(item)"
            class="btn btn-xs"
          >
            {{ authStore.can.manageMembers ? 'Manage' : 'View' }}
          </button>
        </template>
      </ResponsiveList>
      
      <ListPager
        :page="page"
        :total-pages="totalPages"
        :shown="members.length"
        :total="totalItems"
        noun="members"
        :loading="loading"
        @prev="prevPage(queryOptions)"
        @next="nextPage(queryOptions)"
      />
    </BaseCard>
  </div>
</template>
