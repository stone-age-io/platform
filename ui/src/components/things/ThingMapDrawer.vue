<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { Thing, Location } from '@/types/pocketbase'

defineProps<{
  thing: Thing
  isMobile: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const router = useRouter()

function goToLocation(loc: Location) {
  router.push(`/locations/${loc.id}`)
}
</script>

<template>
  <div
    class="absolute top-0 bottom-0 right-0 z-[500] flex flex-col bg-base-100 border-l border-base-300 shadow-xl"
    :class="isMobile ? 'left-0 w-full border-l-0' : 'w-80'"
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/30 shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-lg shrink-0">📦</span>
        <div class="min-w-0">
          <h3 class="font-bold text-sm truncate">{{ thing.name || 'Unnamed' }}</h3>
          <span v-if="thing.expand?.type" class="text-xs text-base-content/60">{{ thing.expand.type.name }}</span>
        </div>
      </div>
      <button class="btn btn-sm btn-circle btn-ghost shrink-0" @click="emit('close')">✕</button>
    </div>

    <!-- Body -->
    <div class="flex-1 overflow-y-auto p-4 space-y-3">
      <div v-if="thing.description">
        <div class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold">Description</div>
        <p class="text-sm mt-1">{{ thing.description }}</p>
      </div>

      <div v-if="thing.code">
        <div class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold">Code</div>
        <code class="text-xs bg-base-200 px-1.5 py-0.5 rounded mt-1 inline-block">{{ thing.code }}</code>
      </div>

      <div v-if="thing.expand?.location">
        <div class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold">Location</div>
        <button
          class="text-sm mt-1 link link-primary text-left"
          @click="goToLocation(thing.expand.location as Location)"
        >
          📍 {{ (thing.expand.location as Location).name }}
        </button>
      </div>
    </div>

    <!-- Footer -->
    <div class="p-3 border-t border-base-300 bg-base-200/30 shrink-0">
      <router-link :to="`/things/${thing.id}`" class="btn btn-sm btn-ghost w-full">
        View Details
      </router-link>
    </div>
  </div>
</template>
