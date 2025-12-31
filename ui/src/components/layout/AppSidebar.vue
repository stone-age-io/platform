<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { pb } from '@/utils/pb'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()

// Modal State
const showNatsModal = ref(false)

// Org Search State
const orgSearchQuery = ref('')

/**
 * Filtered Memberships for the Switcher
 */
const filteredMemberships = computed(() => {
  const q = orgSearchQuery.value.toLowerCase().trim()
  if (!q) return authStore.memberships
  
  return authStore.memberships.filter(m => 
    m.expand?.organization?.name?.toLowerCase().includes(q)
  )
})

/**
 * Navigation Menu Configuration
 */
const menuItems = computed(() => {
  // 1. Core Functional Items
  const items: any[] = [
    { label: 'Dashboard', icon: '📊', path: '/' },
    { label: 'Map', icon: '🗺️', path: '/map' },
    { label: 'Things', icon: '📦', path: '/things' },
    { label: 'Edges', icon: '🔌', path: '/edges' },
    { label: 'Locations', icon: '📍', path: '/locations' },
    { 
      label: 'NATS', 
      icon: '📡', 
      path: '/nats',
      children: [
        { label: 'Accounts', path: '/nats/accounts' },
        { label: 'Users', path: '/nats/users' },
        { label: 'Roles', path: '/nats/roles' },
      ]
    },
    { 
      label: 'Nebula', 
      icon: '🌐', 
      path: '/nebula',
      children: [
        { label: 'CAs', path: '/nebula/cas' },
        { label: 'Networks', path: '/nebula/networks' },
        { label: 'Hosts', path: '/nebula/hosts' },
      ]
    },
  ]

  // 2. Types Group (Admin/Owner Only)
  if (authStore.canManageUsers) {
    items.push({
      label: 'Types',
      icon: '🏷️',
      path: '/types', 
      children: [
        { label: 'Thing Types', path: '/things/types' },
        { label: 'Edge Types', path: '/edges/types' },
        { label: 'Location Types', path: '/locations/types' },
      ]
    })
  }

  // 3. Admin & Audit
  if (authStore.canManageUsers) {
    items.push({ 
      label: 'Team', 
      icon: '👥', 
      path: '/organization',
      children: [
        { label: 'Members', path: '/organization/members' },
        { label: 'Invitations', path: '/organization/invitations' },
      ]
    })
  }

  if (authStore.isSuperAdmin) {
    items.push({ 
      label: 'Organizations', 
      icon: '🏢', 
      path: '/organizations' 
    })
  }

  items.push({ 
    label: 'Audit Logs', 
    icon: '📋', 
    path: '/audit' 
  })
  
  return items
})

const isActive = (path: string) => {
  if (path === '/types') {
    return route.path.includes('/types')
  }
  if (route.path === path) return true
  if (route.path.startsWith(path + '/')) {
    if (route.path.includes('/types') && !path.includes('/types')) {
      return false
    }
    return true
  }
  return false
}

// --- Logic to Close Dropdown ---
function closeUserDropdown() {
  const details = document.getElementById('user-dropdown')
  if (details) details.removeAttribute('open')
  // Clear search on close for fresh state next time
  setTimeout(() => orgSearchQuery.value = '', 200)
}

async function handleOrgChange(orgId: string) {
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  await authStore.switchOrganization(orgId)
  closeUserDropdown() 
  closeDrawer() 
}

async function handleLogout() {
  await authStore.logout()
  closeUserDropdown()
  closeDrawer()
  router.push('/login')
}

// --- Click Outside Handler ---
function handleClickOutside(e: Event) {
  const details = document.getElementById('user-dropdown')
  if (details && details.hasAttribute('open')) {
    if (!details.contains(e.target as Node)) {
      details.removeAttribute('open')
      // Clear search on close
      setTimeout(() => orgSearchQuery.value = '', 200)
    }
  }
}

onMounted(() => {
  window.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  window.removeEventListener('click', handleClickOutside)
})

function getAvatarUrl() {
  if (!authStore.user?.avatar) return null
  return pb.files.getUrl(authStore.user, (authStore.user as any).avatar, { 
    thumb: '100x100',
    token: pb.authStore.token 
  })
}

function closeDrawer() {
  const drawer = document.getElementById('sidebar-drawer') as HTMLInputElement
  if (drawer) drawer.checked = false
}

// NATS Status Helpers
const natsStatusColor = computed(() => {
  switch (natsStore.status) {
    case 'connected': return 'bg-success'
    case 'connecting': 
    case 'reconnecting': return 'bg-warning'
    default: return 'bg-error'
  }
})

const natsStatusText = computed(() => {
  switch (natsStore.status) {
    case 'connected': return 'NATS Connected'
    case 'connecting': return 'Connecting...'
    case 'reconnecting': return 'Reconnecting...'
    default: return 'NATS Disconnected'
  }
})

const serverInfoJson = computed(() => {
  if (!natsStore.nc || !natsStore.nc.info) return 'No server info available.'
  return JSON.stringify(natsStore.nc.info, null, 2)
})
</script>

<template>
  <aside class="bg-base-100 w-72 min-w-[18rem] max-w-[18rem] h-screen flex flex-col border-r border-base-300 overflow-hidden">
    
    <!-- ====================================================================== -->
    <!-- SECTION 1: IDENTITY HEADER (Static) -->
    <!-- ====================================================================== -->
    <div class="flex-none p-3 pb-0 flex flex-col gap-2">
      
      <!-- Brand -->
      <router-link to="/" class="flex items-center gap-3 px-2 py-2 hover:opacity-80 transition-opacity" @click="closeDrawer">
        <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 256 256" class="flex-shrink-0 text-primary">
          <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z" fill="currentColor"/>
        </svg>
        <span class="font-bold text-lg tracking-tight text-base-content">Stone-Age.io</span>
      </router-link>

      <!-- User & Org Card (DETAILS DROPDOWN) -->
      <details id="user-dropdown" class="dropdown w-full">
        <summary 
          class="flex items-center gap-3 w-full p-2 rounded-lg bg-base-200/50 hover:bg-base-200 border border-transparent hover:border-base-300 transition-all cursor-pointer select-none list-none"
        >
          <!-- User Avatar -->
          <div class="avatar placeholder">
            <div v-if="getAvatarUrl()" class="w-8 rounded-full">
              <img :src="getAvatarUrl()!" alt="User avatar" />
            </div>
            <div v-else class="bg-neutral text-neutral-content rounded-full w-8">
              <span class="text-xs font-bold">{{ authStore.user?.name?.[0]?.toUpperCase() || 'U' }}</span>
            </div>
          </div>
          
          <!-- Context Info -->
          <div class="flex flex-col truncate flex-1 text-left min-w-0">
            <span class="font-semibold text-sm truncate">{{ authStore.user?.name || 'User' }}</span>
            <span class="text-xs text-base-content/60 truncate flex items-center gap-1">
              <span class="w-1.5 h-1.5 rounded-full bg-base-content/40"></span>
              {{ authStore.currentOrg?.name || 'Select Org' }}
            </span>
          </div>
          
          <!-- Chevron -->
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 256 256" class="opacity-50 flex-shrink-0"><path d="M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z" fill="currentColor"/></svg>
        </summary>
        
        <!-- Dropdown Menu -->
        <ul class="dropdown-content z-[20] menu p-0 shadow-xl bg-base-100 rounded-box w-full border border-base-300 gap-0 mt-1 overflow-hidden">
          
          <!-- Header -->
          <li class="menu-title px-3 py-2 bg-base-200/30 text-xs font-bold uppercase tracking-wider border-b border-base-200">
            Switch Organization
          </li>

          <!-- Search Input -->
          <div class="px-2 py-2 border-b border-base-200">
            <input 
              v-model="orgSearchQuery"
              type="text" 
              placeholder="Filter organizations..." 
              class="input input-xs input-bordered w-full"
              @click.stop
            />
          </div>

          <!-- SCROLLABLE LIST: Handles 10+ Organizations -->
          <div class="max-h-64 overflow-y-auto scrollbar-thin">
            <li v-for="membership in filteredMemberships" :key="membership.id">
              <a 
                @click="handleOrgChange(membership.organization)"
                :class="{ 'active': authStore.currentOrgId === membership.organization }"
                class="justify-between rounded-none px-4 py-2 border-b border-base-200/50"
              >
                <div class="flex items-center gap-2 truncate">
                  <span class="w-2 h-2 rounded-full flex-shrink-0" :class="authStore.currentOrgId === membership.organization ? 'bg-current' : 'bg-transparent border border-base-content/30'"></span>
                  <span class="truncate">{{ membership.expand?.organization?.name }}</span>
                </div>
              </a>
            </li>
            
            <!-- Empty State -->
            <li v-if="filteredMemberships.length === 0" class="disabled opacity-50 px-4 py-3 text-center text-xs">
              No matching organizations
            </li>
          </div>
          
          <!-- Footer Actions (Always Visible) -->
          <div class="border-t border-base-300 bg-base-100 p-1">
            <li v-if="authStore.isSuperAdmin">
              <router-link to="/organizations/new" class="text-xs" @click="closeUserDropdown">
                + New Organization
              </router-link>
            </li>
            
            <li>
              <router-link to="/settings" @click="closeUserDropdown">
                ⚙️ Settings
              </router-link>
            </li>
            <li>
              <a @click="uiStore.toggleTheme">
                {{ uiStore.theme === 'dark' ? '☀️ Light Mode' : '🌙 Dark Mode' }}
              </a>
            </li>
            <li>
              <a @click="handleLogout" class="text-error hover:bg-error/10">
                🚪 Log out
              </a>
            </li>
          </div>
        </ul>
      </details>

      <!-- Connectivity Status Bar (Clickable) -->
      <div 
        @click="showNatsModal = true" 
        class="flex items-center justify-between px-3 py-1.5 bg-base-100 border border-base-200 rounded text-xs cursor-pointer hover:border-base-300 hover:shadow-sm transition-all group select-none"
      >
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full shadow-sm animate-pulse" :class="natsStatusColor"></span>
          <span class="font-medium text-base-content/70 group-hover:text-base-content transition-colors">
            {{ natsStatusText }}
          </span>
        </div>
        <span v-if="natsStore.isConnected && natsStore.rtt" class="font-mono opacity-50 group-hover:opacity-100">
          {{ natsStore.rtt }}ms
        </span>
      </div>

      <div class="divider my-0"></div>
    </div>
    
    <!-- ====================================================================== -->
    <!-- SECTION 2: NAVIGATION (Scrollable) -->
    <!-- ====================================================================== -->
    <nav class="flex-1 overflow-y-auto overflow-x-hidden px-3 pb-4">
      <ul class="menu p-0 gap-1 w-full">
        <li v-for="item in menuItems" :key="item.label">
          
          <!-- Parent Item -->
          <details v-if="item.children" :open="isActive(item.path)">
            <summary :class="{ 'active': isActive(item.path) }">
              <span class="text-lg opacity-80 w-6 text-center">{{ item.icon }}</span>
              <span class="font-medium truncate">{{ item.label }}</span>
            </summary>
            <ul>
              <li v-for="child in item.children" :key="child.path">
                <router-link 
                  :to="child.path"
                  :class="{ 'active': isActive(child.path) }"
                  @click="closeDrawer"
                  class="truncate"
                >
                  {{ child.label }}
                </router-link>
              </li>
            </ul>
          </details>
          
          <!-- Single Item -->
          <router-link 
            v-else
            :to="item.path"
            :class="{ 'active': isActive(item.path) }"
            @click="closeDrawer"
          >
            <span class="text-lg opacity-80 w-6 text-center">{{ item.icon }}</span>
            <span class="font-medium truncate">{{ item.label }}</span>
          </router-link>
        </li>
      </ul>
    </nav>

    <!-- ====================================================================== -->
    <!-- SECTION 3: MODAL (Teleported) -->
    <!-- ====================================================================== -->
    <Teleport to="body">
      <dialog class="modal" :class="{ 'modal-open': showNatsModal }">
        <div class="modal-box max-w-2xl">
          <div class="flex justify-between items-center mb-4">
            <h3 class="font-bold text-lg flex items-center gap-2">
              <span class="text-2xl">📡</span> Server Information
            </h3>
            <button @click="showNatsModal = false" class="btn btn-sm btn-circle btn-ghost">✕</button>
          </div>
          
          <div v-if="natsStore.isConnected" class="space-y-4">
            <!-- Stats Bar -->
            <div class="stats shadow w-full bg-base-200">
              <div class="stat place-items-center py-2">
                <div class="stat-title text-xs">RTT</div>
                <div class="stat-value text-lg font-mono">{{ natsStore.rtt }}ms</div>
              </div>
              <div class="stat place-items-center py-2">
                <div class="stat-title text-xs">Connected To</div>
                <!-- 
                   FIX: Removed 'truncate max-w-[150px]'. 
                   Added 'break-all text-center leading-tight' to wrap long URLs. 
                -->
                <div class="stat-value text-lg font-mono text-primary break-all text-center leading-tight">
                  {{ natsStore.nc?.getServer() || 'Unknown' }}
                </div>
              </div>
            </div>

            <!-- Raw Info JSON -->
            <!-- 
               FIX: Added 'text-base-content' to override DaisyUI's default white text 
               in mockup-code, ensuring readability in Light Mode. 
            -->
            <div class="mockup-code bg-base-300 text-base-content text-xs">
              <pre class="px-4 py-2"><code>{{ serverInfoJson }}</code></pre>
            </div>
          </div>

          <div v-else class="text-center py-8">
            <span class="text-4xl opacity-50">🔌</span>
            <h3 class="font-bold mt-2">Not Connected</h3>
            <p class="text-sm opacity-70 mb-4">Connect to a NATS server in settings to view diagnostics.</p>
            <router-link to="/settings" class="btn btn-primary btn-sm" @click="showNatsModal = false">
              Go to Settings
            </router-link>
          </div>

          <div class="modal-action">
            <button class="btn" @click="showNatsModal = false">Close</button>
          </div>
        </div>
        <div class="modal-backdrop" @click="showNatsModal = false"></div>
      </dialog>
    </Teleport>

  </aside>
</template>
