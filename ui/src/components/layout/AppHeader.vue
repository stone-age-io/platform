<script setup lang="ts">
import { useUIStore } from '@/stores/ui'

const uiStore = useUIStore()
</script>

<template>
  <!-- 
    Sticky Header for Mobile
    lg:hidden: Hides on desktop
    sticky top-0 z-30: Keeps header fixed while scrolling
  -->
  <header class="navbar bg-base-100 border-b border-base-300 min-h-[4rem] lg:hidden sticky top-0 z-30">
    
    <!-- Left: Sidebar Toggle -->
    <div class="flex-none">
      <label for="sidebar-drawer" class="btn btn-square btn-ghost">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" class="inline-block w-6 h-6 stroke-current">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
        </svg>
      </label>
    </div>
    
    <!-- Center: Logo -->
    <div class="flex-1 flex justify-center">
      <router-link to="/" class="btn btn-ghost btn-circle p-1">
        <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 256 256" class="text-primary">
          <path d="M128,0 C57.343,0 0,57.343 0,128 C0,198.657 57.343,256 128,256 C198.657,256 256,198.657 256,128 C256,57.343 198.657,0 128,0 z M128,28 C181.423,28 224.757,71.334 224.757,124.757 C224.757,139.486 221.04,153.32 214.356,165.42 C198.756,148.231 178.567,138.124 162.876,124.331 C155.723,124.214 128.543,124.043 113.254,124.043 C113.254,147.334 113.254,172.064 113.254,190.513 C100.456,179.347 94.543,156.243 94.543,156.243 C83.432,147.065 31.243,124.757 31.243,124.757 C31.243,71.334 74.577,28 128,28 z" fill="currentColor"/>
        </svg>
      </router-link>
    </div>

    <!-- Right: Theme Toggle -->
    <div class="flex-none">
      <button class="btn btn-square btn-ghost" @click="uiStore.toggleTheme">
        <span v-if="uiStore.theme === 'dark'" class="text-xl">☀️</span>
        <span v-else class="text-xl">🌙</span>
      </button>
    </div>

  </header>
</template>
