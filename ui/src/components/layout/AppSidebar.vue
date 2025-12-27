<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { pb } from '@/utils/pb'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const uiStore = useUIStore()

const menuItems = computed(() => {
  const items = [
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
    { label: 'Audit Logs', icon: '📋', path: '/audit' },
  ]
  
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
  
  return items
})

const isActive = (path: string) => {
  return route.path === path || route.path.startsWith(path + '/')
}

async function handleOrgChange(orgId: string) {
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
  await authStore.switchOrganization(orgId)
}

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}

function getAvatarUrl() {
  if (!authStore.user?.avatar) return null
  return pb.files.getUrl(authStore.user, authStore.user.avatar, { thumb: '100x100' })
}
</script>

<template>
  <aside class="bg-base-100 w-72 min-h-screen flex flex-col border-r border-base-300">
    <!-- SECTION 1: Logo (Sticky Top) -->
    <div class="p-4 flex items-center gap-3">
      <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 256 256" class="flex-shrink-0 text-primary">
        <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z" fill="currentColor"/>
      </svg>
      <span class="font-bold text-lg tracking-tight">Stone-Age.io</span>
    </div>

    <!-- SECTION 2: Organization Switcher -->
    <div class="px-3 pb-2">
      <div class="dropdown w-full">
        <!-- Removed 'transition-colors' to fix theme switching lag -->
        <div 
          tabindex="0" 
          role="button" 
          class="flex items-center justify-between w-full p-2 text-sm font-medium border rounded-md border-base-300 bg-base-100 text-base-content hover:bg-base-200 hover:border-base-400 cursor-pointer select-none"
        >
          <div class="flex items-center gap-2 overflow-hidden">
            <div class="w-5 h-5 rounded-full bg-primary/20 text-primary flex items-center justify-center text-xs font-bold shrink-0">
              {{ authStore.currentOrg?.name?.[0]?.toUpperCase() || '?' }}
            </div>
            <span class="truncate">{{ authStore.currentOrg?.name || 'Select Org' }}</span>
          </div>
          <!-- Chevron -->
          <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 256 256" class="opacity-50"><path d="M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z" fill="currentColor"/></svg>
        </div>
        
        <ul tabindex="0" class="dropdown-content z-[1] menu p-2 shadow-lg bg-base-100 rounded-box w-full border border-base-300 gap-1 mt-1">
          <li class="menu-title px-2">Switch Organization</li>
          <li v-for="membership in authStore.memberships" :key="membership.id">
            <a 
              @click="handleOrgChange(membership.organization)"
              :class="{ 'active': authStore.currentOrgId === membership.organization }"
              class="justify-between"
            >
              <div class="flex items-center gap-2">
                <span class="w-2 h-2 rounded-full" :class="authStore.currentOrgId === membership.organization ? 'bg-current' : 'bg-transparent border border-base-content/30'"></span>
                {{ membership.expand?.organization?.name }}
              </div>
            </a>
          </li>
          
          <!-- Replaced DaisyUI 'divider' class with simple border to remove gray block -->
          <li class="border-t border-base-200 my-1"></li>
          
          <li>
            <router-link to="/organization/invitations" class="text-xs">
              + New Organization
            </router-link>
          </li>
        </ul>
      </div>
    </div>
    
    <!-- SECTION 3: Main Navigation -->
    <ul class="menu p-3 gap-1 flex-1 overflow-y-auto">
      <li v-for="item in menuItems" :key="item.path">
        <!-- Menu item with children -->
        <details v-if="item.children" :open="isActive(item.path)">
          <summary :class="{ 'active': isActive(item.path) }">
            <span class="text-lg opacity-80 w-6 text-center">{{ item.icon }}</span>
            <span class="font-medium">{{ item.label }}</span>
          </summary>
          <ul>
            <li v-for="child in item.children" :key="child.path">
              <router-link 
                :to="child.path"
                :class="{ 'active': isActive(child.path) }"
              >
                {{ child.label }}
              </router-link>
            </li>
          </ul>
        </details>
        
        <!-- Simple menu item -->
        <router-link 
          v-else
          :to="item.path"
          :class="{ 'active': isActive(item.path) }"
        >
          <span class="text-lg opacity-80 w-6 text-center">{{ item.icon }}</span>
          <span class="font-medium">{{ item.label }}</span>
        </router-link>
      </li>
    </ul>
    
    <!-- SECTION 4: User Profile & Actions -->
    <div class="p-3 border-t border-base-300">
      <div class="dropdown dropdown-top w-full">
        <!-- Removed 'transition-colors' -->
        <div 
          tabindex="0" 
          role="button" 
          class="flex items-center gap-3 w-full p-2 rounded-lg hover:bg-base-200 cursor-pointer select-none"
        >
          <div class="avatar placeholder">
            <div v-if="getAvatarUrl()" class="w-9 rounded-full">
              <img :src="getAvatarUrl()!" alt="User avatar" />
            </div>
            <div v-else class="bg-neutral text-neutral-content rounded-full w-9">
              <span class="text-xs">{{ authStore.user?.name?.[0]?.toUpperCase() || 'U' }}</span>
            </div>
          </div>
          <div class="flex flex-col truncate flex-1">
            <span class="font-semibold text-sm truncate">{{ authStore.user?.name || 'User' }}</span>
            <span class="text-xs text-base-content/60 truncate">{{ authStore.user?.email }}</span>
          </div>
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 256 256" class="flex-shrink-0 opacity-50"><path d="M112,60a16,16,0,1,1,16,16A16,16,0,0,1,112,60Zm16,52a16,16,0,1,0,16,16A16,16,0,0,0,128,112Zm0,68a16,16,0,1,0,16,16A16,16,0,0,0,128,180Z" fill="currentColor"/></svg>
        </div>
        
        <ul tabindex="0" class="dropdown-content z-[1] menu p-2 shadow-xl bg-base-100 rounded-box w-full border border-base-300 mb-2">
          <li>
            <router-link to="/settings">
              <span class="w-5 text-center">⚙️</span> Settings
            </router-link>
          </li>
          <li>
            <a @click="uiStore.toggleTheme">
              <span class="w-5 text-center">{{ uiStore.theme === 'dark' ? '☀️' : '🌙' }}</span> 
              {{ uiStore.theme === 'dark' ? 'Light Mode' : 'Dark Mode' }}
            </a>
          </li>
          
          <!-- Replaced divider here too -->
          <li class="border-t border-base-200 my-1"></li>
          
          <li>
            <a @click="handleLogout" class="text-error">
              <span class="w-5 text-center">🚪</span> Log out
            </a>
          </li>
        </ul>
      </div>
    </div>
  </aside>
</template>
