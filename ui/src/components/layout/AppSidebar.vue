<!-- ui/src/components/layout/AppSidebar.vue -->
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMediaQuery } from '@vueuse/core'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { useNatsStore } from '@/stores/nats'
import { useBrandingStore } from '@/stores/branding'
import { pb } from '@/utils/pb'
import BrandLogo from '@/components/common/BrandLogo.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const uiStore = useUIStore()
const natsStore = useNatsStore()
const brandingStore = useBrandingStore()

// Responsive Check: 'lg' breakpoint is 1024px
const isLargeScreen = useMediaQuery('(min-width: 1024px)')

// Computed: Only allow compact mode if user wants it AND we are on desktop
const effectiveCompact = computed(() => uiStore.sidebarCompact && isLargeScreen.value)

// Modal State
const showNatsModal = ref(false)
const showServerInfo = ref(false)

// Org Search State
const orgSearchQuery = ref('')

// Dropdown State (org and user are independent)
const isOrgDropdownOpen = ref(false)
const isUserDropdownOpen = ref(false)
const orgTriggerRef = ref<HTMLElement | null>(null)
const orgMenuRef = ref<HTMLElement | null>(null)
const userTriggerRef = ref<HTMLElement | null>(null)
const userMenuRef = ref<HTMLElement | null>(null)

// Compact-mode teleported dropdown positions (computed at open time)
const orgDropdownPos = ref<Record<string, string>>({})
const userDropdownPos = ref<Record<string, string>>({})

const filteredMemberships = computed(() => {
  const q = orgSearchQuery.value.toLowerCase().trim()
  if (!q) return authStore.memberships

  return authStore.memberships.filter(m =>
    m.expand?.organization?.name?.toLowerCase().includes(q)
  )
})

const orgInitial = computed(() => authStore.currentOrg?.name?.[0]?.toUpperCase() || '?')
const userInitial = computed(() => authStore.user?.name?.[0]?.toUpperCase() || 'U')

// Badge users: home is /badge, others: /
const homeRoute = computed(() => authStore.isBadgeUser ? '/badge' : '/')

const menuItems = computed(() => {
  // Badge users: minimal nav — only badge card + dashboard
  if (authStore.isBadgeUser) {
    return [
      { label: 'Badge', icon: '🪪', path: '/badge' },
      { label: 'Dashboard', icon: '📊', path: '/badge/dashboard' },
    ]
  }

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
        { label: 'Exports', path: '/nats/exports' },
        { label: 'Imports', path: '/nats/imports' },
        ...(natsStore.isConnected && authStore.canManageUsers ? [
          { label: 'Streams', path: '/nats/streams' },
          { label: 'KV Buckets', path: '/nats/kv' },
        ] : []),
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
        { label: 'Thing Operations', path: '/things/operations' },
        { label: 'Message Schemas', path: '/things/schemas' },
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

// Paths that belong to the "Types" parent menu but don't literally contain "/types".
const TYPES_CHILD_PATHS = ['/things/operations', '/things/schemas']

const isUnderTypes = (p: string) =>
  p.includes('/types') || TYPES_CHILD_PATHS.some(cp => p.startsWith(cp))

const isActive = (path: string) => {
  if (path === '/badge' && route.path === '/badge') return true
  if (path === '/types') return isUnderTypes(route.path)
  if (path === '/' && route.path === '/') return true
  if (path === '/overview' && route.path === '/overview') return true

  if (path !== '/' && route.path.startsWith(path)) {
    // Don't light up a non-Types parent when the active route actually belongs to Types.
    if (isUnderTypes(route.path) && !isUnderTypes(path)) return false
    return true
  }
  return false
}

function closeOrgDropdown() {
  isOrgDropdownOpen.value = false
  setTimeout(() => orgSearchQuery.value = '', 200)
}

function closeUserDropdown() {
  isUserDropdownOpen.value = false
}

function toggleOrgDropdown() {
  if (isUserDropdownOpen.value) closeUserDropdown()
  if (!isOrgDropdownOpen.value && effectiveCompact.value && orgTriggerRef.value) {
    const rect = orgTriggerRef.value.getBoundingClientRect()
    orgDropdownPos.value = { top: `${rect.top}px`, left: `${rect.right + 8}px` }
  }
  isOrgDropdownOpen.value = !isOrgDropdownOpen.value
}

function toggleUserDropdown() {
  if (isOrgDropdownOpen.value) closeOrgDropdown()
  if (!isUserDropdownOpen.value && effectiveCompact.value && userTriggerRef.value) {
    const rect = userTriggerRef.value.getBoundingClientRect()
    // Bottom-align with the trigger so menu grows upward from the bottom of the sidebar
    userDropdownPos.value = {
      bottom: `${window.innerHeight - rect.bottom}px`,
      left: `${rect.right + 8}px`,
    }
  }
  isUserDropdownOpen.value = !isUserDropdownOpen.value
}

async function handleOrgChange(orgId: string) {
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  await authStore.switchOrganization(orgId)
  closeOrgDropdown()
  closeDrawer()
}

async function handleLogout() {
  await authStore.logout()
  closeUserDropdown()
  closeDrawer()
  router.push('/login')
}

function handleClickOutside(e: Event) {
  const target = e.target as Node

  if (isOrgDropdownOpen.value
      && !orgTriggerRef.value?.contains(target)
      && !orgMenuRef.value?.contains(target)) {
    closeOrgDropdown()
  }

  if (isUserDropdownOpen.value
      && !userTriggerRef.value?.contains(target)
      && !userMenuRef.value?.contains(target)) {
    closeUserDropdown()
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
  return pb.files.getURL(authStore.user, (authStore.user as any).avatar, {
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
    <!-- TOP: BRAND + ORG SELECTOR -->
    <!-- ====================================================================== -->
    <div class="flex-none p-3 pb-0 flex flex-col gap-2">

      <!-- Brand + Toggle -->
      <div class="flex transition-all duration-300" :class="effectiveCompact ? 'flex-col items-center gap-4 py-2' : 'flex-row items-center justify-between px-2 py-2'">

        <router-link :to="homeRoute" class="flex items-center gap-3 hover:opacity-80 transition-opacity overflow-hidden" @click="closeDrawer">
          <div class="w-8 h-8 flex items-center justify-center flex-shrink-0 text-primary">
            <BrandLogo :size="32" />
          </div>
          <span v-show="!effectiveCompact" class="font-bold text-lg tracking-tight text-base-content whitespace-nowrap overflow-hidden">{{ brandingStore.appName }}</span>
        </router-link>

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

      <!-- Org Selector (DROPDOWN) -->
      <div
        class="dropdown"
        :class="effectiveCompact ? '' : 'dropdown-bottom w-full'"
      >
        <!-- Trigger -->
        <div
          ref="orgTriggerRef"
          @click="toggleOrgDropdown"
          class="flex items-center gap-3 w-full p-2 rounded-lg bg-base-200/50 hover:bg-base-200 border border-transparent hover:border-base-300 transition-all cursor-pointer select-none"
          :class="{ 'justify-center': effectiveCompact }"
          role="button"
        >
          <!-- Org Initial Square -->
          <div class="w-8 h-8 rounded-md bg-primary/15 text-primary border border-primary/20 flex items-center justify-center flex-shrink-0">
            <span class="text-xs font-bold">{{ orgInitial }}</span>
          </div>

          <!-- Org Info -->
          <div v-show="!effectiveCompact" class="flex flex-col truncate flex-1 text-left min-w-0">
            <span class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold leading-tight">Organization</span>
            <span class="font-semibold text-sm truncate leading-tight">{{ authStore.currentOrg?.name || 'Select Org' }}</span>
          </div>

          <!-- Chevron -->
          <svg v-show="!effectiveCompact" xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 256 256" class="opacity-50 flex-shrink-0"><path d="M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z" fill="currentColor"/></svg>
        </div>

        <!-- Dropdown Menu -->
        <Teleport to="body" :disabled="!effectiveCompact">
          <ul
            v-if="isOrgDropdownOpen"
            ref="orgMenuRef"
            class="z-[9999] menu p-0 shadow-xl bg-base-100 rounded-box border border-base-300 gap-0 overflow-hidden"
            :class="effectiveCompact ? 'fixed w-72' : 'absolute w-full mt-1 left-0'"
            :style="effectiveCompact ? orgDropdownPos : undefined"
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

            <!-- Footer Action -->
            <div v-if="authStore.canManageOrganizations" class="border-t border-base-300 bg-base-100 p-1">
              <li>
                <router-link to="/organizations/new" class="text-xs" @click="closeOrgDropdown">
                  + New Organization
                </router-link>
              </li>
            </div>
          </ul>
        </Teleport>
      </div>

      <div class="divider my-0"></div>
    </div>

    <!-- ====================================================================== -->
    <!-- NAVIGATION (Scrollable) -->
    <!-- ====================================================================== -->
    <nav class="flex-1 overflow-y-auto overflow-x-hidden px-3 pb-2">
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
    <!-- BOTTOM: NATS STATUS + USER -->
    <!-- ====================================================================== -->
    <div class="flex-none p-3 pt-0 flex flex-col gap-2">
      <div class="divider my-0"></div>

      <!-- NATS Status Pill -->
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

      <!-- User Selector (DROPDOWN, opens upward) -->
      <div
        class="dropdown"
        :class="effectiveCompact ? '' : 'dropdown-top w-full'"
      >
        <!-- Trigger -->
        <div
          ref="userTriggerRef"
          @click="toggleUserDropdown"
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
              <span class="text-xs font-bold">{{ userInitial }}</span>
            </div>
          </div>

          <!-- User Info -->
          <div v-show="!effectiveCompact" class="flex flex-col truncate flex-1 text-left min-w-0">
            <span class="font-semibold text-sm truncate leading-tight">{{ authStore.user?.name || 'User' }}</span>
            <span v-if="(authStore.user as any)?.email" class="text-xs text-base-content/60 truncate leading-tight">{{ (authStore.user as any).email }}</span>
          </div>

          <!-- Chevron (rotated since menu opens upward) -->
          <svg v-show="!effectiveCompact" xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 256 256" class="opacity-50 flex-shrink-0 rotate-180"><path d="M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z" fill="currentColor"/></svg>
        </div>

        <!-- Dropdown Menu -->
        <Teleport to="body" :disabled="!effectiveCompact">
          <ul
            v-if="isUserDropdownOpen"
            ref="userMenuRef"
            class="z-[9999] menu p-1 shadow-xl bg-base-100 rounded-box border border-base-300 gap-0 overflow-hidden"
            :class="effectiveCompact ? 'fixed w-72' : 'absolute w-full mb-1 left-0 bottom-full'"
            :style="effectiveCompact ? userDropdownPos : undefined"
          >
            <li v-if="!authStore.isBadgeUser">
              <router-link to="/my-badge" @click="closeUserDropdown(); closeDrawer()">
                🪪 My Badge
              </router-link>
            </li>
            <li>
              <router-link :to="authStore.isBadgeUser ? '/badge/settings' : '/settings'" @click="closeUserDropdown">
                ⚙️ Settings
              </router-link>
            </li>
            <li>
              <a @click="uiStore.toggleTheme">
                {{ uiStore.theme === 'dark' ? '☀️ Light Mode' : '🌙 Dark Mode' }}
              </a>
            </li>
            <div class="divider my-1"></div>
            <li>
              <a @click="handleLogout" class="text-error hover:bg-error/10">
                🚪 Log out
              </a>
            </li>
          </ul>
        </Teleport>
      </div>
    </div>

    <!-- ====================================================================== -->
    <!-- NATS MODAL (Teleported) -->
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
