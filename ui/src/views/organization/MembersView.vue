<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { usePagination } from '@/composables/usePagination'
import { useServerSearch } from '@/composables/useServerSearch'
import { useAuthStore } from '@/stores/auth'
import type { Membership } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import BaseCard from '@/components/ui/BaseCard.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'
import ListPager from '@/components/ui/ListPager.vue'

const router = useRouter() // Added
const authStore = useAuthStore()

// Pagination for members list
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

// Search
// Search runs on the SERVER. The client-side pass this replaces filtered the
// twenty records already on screen, so a match on page three answered "No
// results found" -- which is not the same claim at all.
const { searchQuery, filter: searchFilter } = useServerSearch(
  ['user.name', 'user.email', 'organization.name', 'role'],
  () => {
    page.value = 1
    loadMembers()
  },
)

// The active sort, in PocketBase's own syntax ('name', '-created'). It rides in
// queryOptions rather than being passed at the call site, for the same reason the
// filter does: the pager buttons reuse that object, and a sort that is not in it
// is a sort that page two forgets.
const sort = ref('role,-created')

// One options object, passed to EVERY call into usePagination, the pager
// buttons included -- passing only expand/sort there drops the filter and page
// two comes back unfiltered.
//
// The organization clause is ANDed with the search rather than replaced by it.
// A platform operator reaches this view too, and their memberships listRule
// spans organizations, so dropping the clause would let another tenant's
// members appear in a search result here.
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
  page.value = 1 // a new order makes the old page number meaningless
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
    sortable: 'user.email',
    label: 'Email',
    mobileLabel: 'Email',
  },
  {
    key: 'role',
    sortable: 'role',
    label: 'Role',
    mobileLabel: 'Role',
    format: (value) => value.charAt(0).toUpperCase() + value.slice(1),
  },
]

async function loadMembers() {
  if (!authStore.currentOrgId) return

  await load(queryOptions.value)
}

// Navigate to Detail View
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

function handleOrgChange() {
  searchQuery.value = ''
  loadMembers()
}

onMounted(() => {
  loadMembers()
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
        <h1 class="text-3xl font-bold">Team Members</h1>
        <p class="text-base-content/70 mt-1">
          Manage members of {{ authStore.currentOrg?.name }}
        </p>
      </div>
      <router-link 
        to="/organization/invitations" 
        class="btn btn-primary w-full sm:w-auto"
      >
        <span class="text-lg">✉️</span>
        <span>Send Invitation</span>
      </router-link>
    </div>
    
    <!-- Search -->
    <div class="form-control">
      <input 
        v-model="searchQuery"
        type="text"
        placeholder="Search members by name, email, or role..."
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
        <span class="text-6xl">👥</span>
        <h3 class="text-xl font-bold mt-4">No members found</h3>
      </div>
    </BaseCard>
    
    <!-- No Search Results -->
    <BaseCard v-else-if="members.length === 0">
      <div class="text-center py-12">
        <span class="text-6xl">🔍</span>
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
        <!-- Custom cell for name -->
        <template #cell-expand.user.name="{ item }">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-10">
                <span class="text-xs">
                  {{ item.expand?.user?.name?.[0]?.toUpperCase() || '?' }}
                </span>
              </div>
            </div>
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
        
        <!-- Custom mobile card for name -->
        <template #card-expand.user.name="{ item }">
          <div class="flex items-center gap-3">
            <div class="avatar placeholder">
              <div class="bg-neutral text-neutral-content rounded-full w-12">
                <span class="text-sm">
                  {{ item.expand?.user?.name?.[0]?.toUpperCase() || '?' }}
                </span>
              </div>
            </div>
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
        
        <!-- Custom cell for email -->
        <template #cell-expand.user.email="{ item }">
          <span class="opacity-70 text-sm">{{ item.expand?.user?.email }}</span>
        </template>
        
        <!-- Custom cell for role -->
        <template #cell-role="{ item }">
          <span 
            class="badge badge-sm"
            :class="getRoleBadgeClass(item.role)"
          >
            {{ item.role.charAt(0).toUpperCase() + item.role.slice(1) }}
          </span>
        </template>
        
        <!-- Actions -->
        <template #actions="{ item }">
          <button 
            @click.stop="handleRowClick(item)"
            class="btn btn-xs flex-1 sm:flex-initial"
          >
            {{ authStore.can.manageMembers ? 'Manage' : 'View' }}
          </button>
        </template>
      </ResponsiveList>
      
      <!-- Pagination -->
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
