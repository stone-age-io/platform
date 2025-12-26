<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useRouter } from 'vue-router'
import { pb } from '@/utils/pb'

const authStore = useAuthStore()
const uiStore = useUIStore()
const router = useRouter()

/**
 * Handle logout
 */
async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}

/**
 * Handle organization change
 */
async function handleOrgChange(event: Event) {
  const orgId = (event.target as HTMLSelectElement).value
  if (orgId) {
    await authStore.switchOrganization(orgId)
  }
}

/**
 * Get avatar URL for current user
 */
function getAvatarUrl() {
  if (!authStore.user?.avatar) return null
  return pb.files.getUrl(authStore.user, authStore.user.avatar, { thumb: '100x100' })
}
</script>

<template>
  <header class="navbar bg-base-100 border-b border-base-300 gap-2">
    <!-- Mobile menu button -->
    <div class="flex-1 lg:hidden">
      <label for="sidebar-drawer" class="btn btn-ghost btn-square">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-6 h-6 stroke-current">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
        </svg>
      </label>
    </div>
    
    <!-- Desktop spacer -->
    <div class="flex-1 hidden lg:block"></div>
    
    <!-- Right side controls -->
    <div class="flex-none gap-2">
      <!-- Organization Switcher -->
      <select 
        v-if="authStore.memberships.length > 0"
        class="select select-bordered select-sm"
        :value="authStore.currentOrgId || ''"
        @change="handleOrgChange"
      >
        <option disabled value="">Select Organization</option>
        <option 
          v-for="membership in authStore.memberships" 
          :key="membership.id"
          :value="membership.organization"
        >
          {{ membership.expand?.organization?.name || 'Loading...' }}
        </option>
      </select>
      
      <!-- Theme Toggle -->
      <button 
        class="btn btn-ghost btn-circle" 
        @click="uiStore.toggleTheme"
        :title="uiStore.theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
      >
        <span v-if="uiStore.theme === 'dark'" class="text-xl">☀️</span>
        <span v-else class="text-xl">🌙</span>
      </button>
      
      <!-- User Menu -->
      <div class="dropdown dropdown-end">
        <label tabindex="0" class="btn btn-ghost btn-circle avatar">
          <div class="w-10 rounded-full">
            <img 
              v-if="getAvatarUrl()"
              :src="getAvatarUrl()!"
              :alt="authStore.user?.name || 'User avatar'"
            />
            <div v-else class="bg-neutral text-neutral-content grid place-items-center">
              <span class="text-xl">
                {{ authStore.user?.name?.[0]?.toUpperCase() || '?' }}
              </span>
            </div>
          </div>
        </label>
        <ul 
          tabindex="0" 
          class="mt-3 p-2 shadow menu menu-sm dropdown-content bg-base-100 rounded-box w-52 border border-base-300"
        >
          <li class="menu-title">
            <span>{{ authStore.user?.name || authStore.user?.email }}</span>
          </li>
          <li><router-link to="/settings">⚙️ Settings</router-link></li>
          <li><a @click="handleLogout">🚪 Logout</a></li>
        </ul>
      </div>
    </div>
  </header>
</template>
