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
      <h1 class="text-2xl font-bold">Stone Age 🪨</h1>
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
