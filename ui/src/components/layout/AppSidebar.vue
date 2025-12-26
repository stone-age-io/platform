<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

/**
 * Menu items configuration
 * Conditionally shows Team menu based on user role
 */
const menuItems = computed(() => {
  const items = [
    { label: 'Dashboard', icon: '📊', path: '/' },
    { label: 'Things', icon: '📦', path: '/things' },
    { label: 'Edges', icon: '🔌', path: '/edges' },
    { label: 'Locations', icon: '📍', path: '/locations' },
    { label: 'Map', icon: '🗺️', path: '/map' },
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
  
  // Add Team menu for admins/owners
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

/**
 * Check if a path is currently active
 */
const isActive = (path: string) => {
  return route.path === path || route.path.startsWith(path + '/')
}
</script>

<template>
  <aside class="bg-base-100 w-64 min-h-screen flex flex-col">
    <!-- Logo/Title -->
    <div class="p-4 border-b border-base-300">
      <div class="flex items-center gap-4">
        <svg 
          xmlns="http://www.w3.org/2000/svg" 
          width="28" 
          height="28" 
          viewBox="0 0 256 256" 
          class="flex-shrink-0"
          aria-label="Stone-Age.io Logo"
        >
          <path 
            d="M 128,0 C 57.343,0 0,57.343 0,128 C 0,198.657 57.343,256 128,256 C 198.657,256 256,198.657 256,128 C 256,57.343 198.657,0 128,0 z M 128,28 C 181.423,28 224.757,71.334 224.757,124.757 C 224.757,139.486 221.04,153.32 214.356,165.42 C 198.756,148.231 178.567,138.124 162.876,124.331 C 155.723,124.214 128.543,124.043 113.254,124.043 C 113.254,147.334 113.254,172.064 113.254,190.513 C 100.456,179.347 94.543,156.243 94.543,156.243 C 83.432,147.065 31.243,124.757 31.243,124.757 C 31.243,71.334 74.577,28 128,28 z" 
            fill="currentColor"
          />
        </svg>
        <h1 class="text-xl font-semibold whitespace-nowrap">Stone-Age.io</h1>
      </div>
    </div>
    
    <!-- Navigation Menu -->
    <ul class="menu p-4 gap-1 flex-1">
      <li v-for="item in menuItems" :key="item.path">
        <!-- Menu item with children (collapsible) -->
        <details v-if="item.children" :open="isActive(item.path)">
          <summary :class="{ 'active': isActive(item.path) }">
            <span class="text-xl">{{ item.icon }}</span>
            <span>{{ item.label }}</span>
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
          <span class="text-xl">{{ item.icon }}</span>
          <span>{{ item.label }}</span>
        </router-link>
      </li>
    </ul>
    
    <!-- Bottom Section - Settings -->
    <div class="p-4 border-t border-base-300">
      <router-link to="/settings" class="btn btn-ghost w-full justify-start gap-4">
        <span class="text-xl">⚙️</span>
        <span>Settings</span>
      </router-link>
    </div>
  </aside>
</template>
