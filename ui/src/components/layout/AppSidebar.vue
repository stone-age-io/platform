<!-- ui/src/components/layout/AppSidebar.vue -->
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMediaQuery } from '@vueuse/core'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { pb } from '@/utils/pb'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()

// Responsive Check: 'lg' breakpoint is 1024px
const isLargeScreen = useMediaQuery('(min-width: 1024px)')

// Computed: Only allow compact mode if user wants it AND we are on desktop
const effectiveCompact = computed(() => uiStore.sidebarCompact && isLargeScreen.value)

// Modal State
const showNatsModal = ref(false)
const showServerInfo = ref(false)

// Org Search State
const orgSearchQuery = ref('')

// Dropdown State
const isDropdownOpen = ref(false)
const dropdownTriggerRef = ref<HTMLElement | null>(null)
const dropdownMenuRef = ref<HTMLElement | null>(null)

const filteredMemberships = computed(() => {
  const q = orgSearchQuery.value.toLowerCase().trim()
  if (!q) return authStore.memberships
  
  return authStore.memberships.filter(m => 
    m.expand?.organization?.name?.toLowerCase().includes(q)
  )
})

const menuItems = computed(() => {
  const items: any[] = [
    { label: 'Dashboard', icon: '📊', path: '/' },
    { label: 'Things', icon: '📦', path: '/things' },
    { label: 'Locations', icon: '📍', path: '/locations' },
    { 
      label: 'NATS', 
      icon: '📡', 
      path: '/nats',
      children: [
        { label: 'Account', path: '/nats/account' },
        { label: 'Users', path: '/nats/users' },
        { label: 'Roles', path: '/nats/roles' },
      ]
    },
    { 
      label: 'Nebula', 
      icon: '🌐', 
      path: '/nebula',
      children: [
        { label: 'Certificate Authority', path: '/nebula/ca' },
        { label: 'Networks', path: '/nebula/networks' },
        { label: 'Hosts', path: '/nebula/hosts' },
      ]
    },
  ]

  if (authStore.canManageUsers) {
    items.push({
      label: 'Types',
      icon: '🏷️',
      path: '/types', 
      children: [
        { label: 'Thing Types', path: '/things/types' },
        { label: 'Location Types', path: '/locations/types' },
      ]
    })
  }

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

  if (authStore.canManageOrganizations) {
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
  if (path === '/types') return route.path.includes('/types')
  if (path === '/' && route.path === '/') return true
  if (path === '/overview' && route.path === '/overview') return true
  
  if (path !== '/' && route.path.startsWith(path)) {
    if (route.path.includes('/types') && !path.includes('/types')) return false
    return true
  }
  return false
}

function closeUserDropdown() {
  isDropdownOpen.value = false
  setTimeout(() => orgSearchQuery.value = '', 200)
}

function toggleDropdown() {
  isDropdownOpen.value = !isDropdownOpen.value
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

function handleClickOutside(e: Event) {
  if (!isDropdownOpen.value) return
  const target = e.target as Node
  
  // Allow clicks inside the trigger or the menu (even if teleported)
  if (dropdownTriggerRef.value?.contains(target)) return
  if (dropdownMenuRef.value?.contains(target)) return
  
  closeUserDropdown()
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
    case 'connected': return 'Connected'
    case 'connecting': return 'Connecting...'
    case 'reconnecting': return 'Reconnecting...'
    default: return 'Disconnected'
  }
})

const serverInfoJson = computed(() => {
  if (!natsStore.nc || !natsStore.nc.info) return 'No server info available.'
  return JSON.stringify(natsStore.nc.info, null, 2)
})

// NATS Connection Controls
const canConnect = computed(() => {
  return authStore.isAuthenticated
    && authStore.currentMembership?.nats_user
    && natsStore.serverUrls.length > 0
})

async function handleConnect() {
  await natsStore.connect()
}

async function handleDisconnect() {
  await natsStore.disconnect()
}
</script>

<template>
  <aside 
    class="bg-base-100 h-screen flex flex-col border-r border-base-300 transition-all duration-300 ease-in-out z-20"
    :class="effectiveCompact ? 'w-20 min-w-[5rem]' : 'w-72 min-w-[18rem]'"
  >
    
    <!-- ====================================================================== -->
    <!-- SECTION 1: IDENTITY HEADER & TOGGLE -->
    <!-- ====================================================================== -->
    <div class="flex-none p-3 pb-0 flex flex-col gap-2">
      
      <!-- Top Row: Brand + Toggle -->
      <!-- Expanded: Row (Space Between), Compact: Column (Gap) -->
      <div class="flex transition-all duration-300" :class="effectiveCompact ? 'flex-col items-center gap-4 py-2' : 'flex-row items-center justify-between px-2 py-2'">
        
        <!-- Brand Link -->
        <router-link to="/" class="flex items-center gap-3 hover:opacity-80 transition-opacity overflow-hidden" @click="closeDrawer">
          <div class="w-8 h-8 flex items-center justify-center flex-shrink-0 text-primary">
             <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 256 256" fill="currentColor">
              <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z"/>
            </svg>
          </div>
          <span v-show="!effectiveCompact" class="font-bold text-lg tracking-tight text-base-content whitespace-nowrap overflow-hidden">Stone-Age.io</span>
        </router-link>

        <!-- Toggle Button (Desktop Only) -->
        <!-- In Compact mode, this sits below the logo. In Expanded, to the right. -->
        <button 
          v-if="isLargeScreen"
          @click="uiStore.toggleCompact" 
          class="btn btn-ghost btn-sm btn-square opacity-60 hover:opacity-100 transition-opacity"
          :title="uiStore.sidebarCompact ? 'Expand Sidebar' : 'Collapse Sidebar'"
        >
          <span v-if="uiStore.sidebarCompact">»</span>
          <span v-else>«</span>
        </button>

      </div>

      <!-- User & Org Card (DROPDOWN) -->
      <div 
        class="dropdown"
        :class="effectiveCompact ? '' : 'dropdown-bottom w-full'"
      >
        <!-- Trigger -->
        <div 
          ref="dropdownTriggerRef"
          @click="toggleDropdown"
          class="flex items-center gap-3 w-full p-2 rounded-lg bg-base-200/50 hover:bg-base-200 border border-transparent hover:border-base-300 transition-all cursor-pointer select-none"
          :class="{ 'justify-center': effectiveCompact }"
          role="button"
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
          <div v-show="!effectiveCompact" class="flex flex-col truncate flex-1 text-left min-w-0">
            <span class="font-semibold text-sm truncate">{{ authStore.user?.name || 'User' }}</span>
            <span class="text-xs text-base-content/60 truncate flex items-center gap-1">
              <span class="w-1.5 h-1.5 rounded-full bg-base-content/40"></span>
              {{ authStore.currentOrg?.name || 'Select Org' }}
            </span>
          </div>
          
          <!-- Chevron -->
          <svg v-show="!effectiveCompact" xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 256 256" class="opacity-50 flex-shrink-0"><path d="M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z" fill="currentColor"/></svg>
        </div>
        
        <!-- Dropdown Menu -->
        <!-- Use Teleport when compact to escape sidebar clipping -->
        <Teleport to="body" :disabled="!effectiveCompact">
          <ul 
            v-if="isDropdownOpen"
            ref="dropdownMenuRef"
            class="z-[9999] menu p-0 shadow-xl bg-base-100 rounded-box border border-base-300 gap-0 overflow-hidden"
            :class="effectiveCompact ? 'fixed left-[5.5rem] top-[4.5rem] w-72' : 'absolute w-full mt-1 left-0'"
          >
            
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

            <!-- SCROLLABLE LIST -->
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
              
              <li v-if="filteredMemberships.length === 0" class="disabled opacity-50 px-4 py-3 text-center text-xs">
                No matching organizations
              </li>
            </div>
            
            <!-- Footer Actions -->
            <div class="border-t border-base-300 bg-base-100 p-1">
              <li v-if="authStore.canManageOrganizations">
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
        </Teleport>
      </div>

      <!-- Connectivity Status Bar (Clickable) -->
      <div 
        @click="showNatsModal = true" 
        class="flex items-center px-3 py-1.5 bg-base-100 border border-base-200 rounded text-xs cursor-pointer hover:border-base-300 hover:shadow-sm transition-all group select-none"
        :class="effectiveCompact ? 'justify-center' : 'justify-between'"
      >
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full shadow-sm animate-pulse" :class="natsStatusColor"></span>
          <span v-show="!effectiveCompact" class="font-medium text-base-content/70 group-hover:text-base-content transition-colors">
            {{ natsStatusText }}
          </span>
        </div>
        <span v-show="!effectiveCompact && natsStore.isConnected && natsStore.rtt" class="font-mono opacity-50 group-hover:opacity-100">
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
          <details v-if="item.children" :open="isActive(item.path) && !effectiveCompact">
            <summary 
              :class="{ 'active': isActive(item.path) }"
              class="group"
            >
              <!-- Tooltip for Compact Mode -->
              <div 
                v-if="effectiveCompact" 
                class="tooltip tooltip-right absolute left-0 w-full h-full" 
                :data-tip="item.label"
              ></div>
              
              <span class="text-lg opacity-80 w-6 text-center">{{ item.icon }}</span>
              <span v-show="!effectiveCompact" class="font-medium truncate">{{ item.label }}</span>
            </summary>
            
            <ul v-show="!effectiveCompact">
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
            class="group relative"
          >
            <!-- Tooltip for Compact Mode -->
            <div 
              v-if="effectiveCompact" 
              class="tooltip tooltip-right absolute left-0 w-full h-full" 
              :data-tip="item.label"
            ></div>

            <span class="text-lg opacity-80 w-6 text-center">{{ item.icon }}</span>
            <span v-show="!effectiveCompact" class="font-medium truncate">{{ item.label }}</span>
          </router-link>
        </li>
      </ul>
    </nav>

    <!-- ====================================================================== -->
    <!-- SECTION 4: MODAL (Teleported) -->
    <!-- ====================================================================== -->
    <Teleport to="body">
      <dialog class="modal" :class="{ 'modal-open': showNatsModal }">
        <div class="modal-box max-w-2xl">
          <!-- Header -->
          <div class="flex justify-between items-center mb-4">
            <h3 class="font-bold text-lg flex items-center gap-2">
              <span class="text-2xl">📡</span> NATS Connection
            </h3>
            <button @click="showNatsModal = false" class="btn btn-sm btn-circle btn-ghost">✕</button>
          </div>

          <!-- Connection Status & Controls -->
          <div class="flex flex-wrap items-center justify-between gap-3 mb-4 p-3 bg-base-200 rounded-box">
            <div class="flex items-center gap-2">
              <span class="w-2.5 h-2.5 rounded-full shrink-0" :class="natsStatusColor"></span>
              <span class="font-medium">{{ natsStatusText }}</span>
              <span v-if="natsStore.reconnectCount > 0" class="badge badge-ghost badge-sm">
                {{ natsStore.reconnectCount }}x
              </span>
            </div>
            <div class="flex gap-2">
              <button
                v-if="!natsStore.isConnected && natsStore.status !== 'connecting'"
                @click="handleConnect"
                :disabled="!canConnect"
                class="btn btn-success btn-sm"
              >
                Connect
              </button>
              <button
                v-if="natsStore.status === 'connecting'"
                disabled
                class="btn btn-sm"
              >
                <span class="loading loading-spinner loading-xs"></span>
                Connecting
              </button>
              <button
                v-if="natsStore.isConnected || natsStore.status === 'reconnecting'"
                @click="handleDisconnect"
                class="btn btn-error btn-sm"
              >
                Disconnect
              </button>
            </div>
          </div>

          <!-- Error Message -->
          <div v-if="natsStore.lastError" class="alert alert-error mb-4 text-sm">
            <span>{{ natsStore.lastError }}</span>
          </div>

          <!-- No Identity Warning -->
          <div v-if="!authStore.currentMembership?.nats_user" class="alert alert-warning mb-4 text-sm">
            <span>No NATS identity linked to this organization. Configure one in Settings.</span>
          </div>

          <!-- Connected State: Show Server Info -->
          <div v-if="natsStore.isConnected" class="space-y-3">
            <!-- Stats -->
            <div class="grid grid-cols-2 gap-3">
              <div class="bg-base-200 rounded-box p-3 text-center">
                <div class="text-xs opacity-60 mb-1">RTT</div>
                <div class="text-xl font-mono font-semibold">{{ natsStore.rtt ?? '—' }}<span class="text-sm opacity-60">ms</span></div>
              </div>
              <div class="bg-base-200 rounded-box p-3 text-center">
                <div class="text-xs opacity-60 mb-1">Server</div>
                <div class="text-sm font-mono text-primary break-all">{{ natsStore.nc?.getServer() || 'Unknown' }}</div>
              </div>
            </div>

            <!-- Server Info JSON (collapsible) -->
            <div class="bg-base-200 rounded-box overflow-hidden">
              <button
                @click="showServerInfo = !showServerInfo"
                class="w-full flex items-center justify-between p-3 text-sm font-medium hover:bg-base-300 transition-colors"
              >
                <span>Server Info</span>
                <span class="text-xs opacity-60">{{ showServerInfo ? '▲' : '▼' }}</span>
              </button>
              <div v-show="showServerInfo" class="border-t border-base-300">
                <pre class="p-3 text-xs overflow-x-auto"><code>{{ serverInfoJson }}</code></pre>
              </div>
            </div>
          </div>

          <!-- Disconnected State -->
          <div v-else-if="natsStore.status === 'disconnected'" class="text-center py-8">
            <span class="text-4xl opacity-40">🔌</span>
            <p class="text-sm opacity-60 mt-2">
              {{ authStore.currentMembership?.nats_user
                ? 'Click Connect to establish a connection.'
                : 'Link a NATS identity in Settings to enable connections.' }}
            </p>
          </div>

          <!-- Footer -->
          <div class="modal-action mt-6">
            <router-link to="/settings" class="btn btn-ghost" @click="showNatsModal = false">
              Settings
            </router-link>
            <button class="btn" @click="showNatsModal = false">Close</button>
          </div>
        </div>
        <div class="modal-backdrop" @click="showNatsModal = false"></div>
      </dialog>
    </Teleport>

  </aside>
</template>

<style scoped>
/* Hide summary marker in compact mode to avoid weird arrows */
details[open] > summary {
  margin-bottom: 0;
}
</style>

