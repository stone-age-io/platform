<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Thing, Location } from '@/types/pocketbase'

const props = defineProps<{
  things: Thing[]
  isMobile: boolean
}>()

const emit = defineEmits<{
  close: []
  select: [thingId: string]
}>()

const router = useRouter()

const isList = computed(() => props.things.length > 1)
const single = computed(() => props.things[0])

// If every thing in the cluster shares one parent location, surface it
// in the header — common case for the warehouse-with-many-sensors pattern.
const sharedLocation = computed<Location | null>(() => {
  if (!isList.value) return null
  const first = props.things[0].expand?.location as Location | undefined
  if (!first?.id) return null
  const allSame = props.things.every(t =>
    (t.expand?.location as Location | undefined)?.id === first.id
  )
  return allSame ? first : null
})

function goToLocation(loc: Location) {
  router.push(`/locations/${loc.id}`)
}
</script>

<template>
  <div
    class="absolute top-0 bottom-0 right-0 z-[500] flex flex-col bg-base-100 border-l border-base-300 shadow-xl"
    :class="isMobile ? 'left-0 w-full border-l-0' : 'w-80'"
  >
    <!-- Header (cluster / list mode) -->
    <div v-if="isList" class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/30 shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-lg shrink-0">📦</span>
        <div class="min-w-0">
          <h3 class="font-bold text-sm truncate">{{ things.length }} things</h3>
          <span v-if="sharedLocation" class="text-xs text-base-content/60 truncate block">
            at {{ sharedLocation.name }}
          </span>
          <span v-else class="text-xs text-base-content/60">in this cluster</span>
        </div>
      </div>
      <button class="btn btn-sm btn-circle btn-ghost shrink-0" @click="emit('close')">✕</button>
    </div>

    <!-- Header (single / detail mode) -->
    <div v-else class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/30 shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-lg shrink-0">📦</span>
        <div class="min-w-0">
          <h3 class="font-bold text-sm truncate">{{ single.name || 'Unnamed' }}</h3>
          <span v-if="single.expand?.type" class="text-xs text-base-content/60">{{ single.expand.type.name }}</span>
        </div>
      </div>
      <button class="btn btn-sm btn-circle btn-ghost shrink-0" @click="emit('close')">✕</button>
    </div>

    <!-- Body (cluster / list mode) -->
    <div v-if="isList" class="flex-1 overflow-y-auto">
      <button
        v-for="t in things"
        :key="t.id"
        class="w-full text-left p-3 border-b border-base-300 hover:bg-base-200 transition-colors flex items-center justify-between gap-2 min-w-0"
        @click="emit('select', t.id)"
      >
        <div class="min-w-0 flex-1">
          <div class="font-medium text-sm truncate">{{ t.name || 'Unnamed' }}</div>
          <div class="flex items-center gap-2 mt-0.5">
            <span v-if="t.expand?.type" class="badge badge-ghost badge-xs">{{ t.expand.type.name }}</span>
            <code v-if="t.code" class="text-[10px] text-base-content/60">{{ t.code }}</code>
          </div>
        </div>
        <span class="text-base-content/40 text-xs shrink-0">›</span>
      </button>
    </div>

    <!-- Body (single / detail mode) -->
    <div v-else class="flex-1 overflow-y-auto p-4 space-y-3">
      <div v-if="single.description">
        <div class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold">Description</div>
        <p class="text-sm mt-1">{{ single.description }}</p>
      </div>

      <div v-if="single.code">
        <div class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold">Code</div>
        <code class="text-xs bg-base-200 px-1.5 py-0.5 rounded mt-1 inline-block">{{ single.code }}</code>
      </div>

      <div v-if="single.expand?.location">
        <div class="text-[10px] uppercase tracking-wider text-base-content/50 font-semibold">Location</div>
        <button
          class="text-sm mt-1 link link-primary text-left"
          @click="goToLocation(single.expand.location as Location)"
        >
          📍 {{ (single.expand.location as Location).name }}
        </button>
      </div>
    </div>

    <!-- Footer (detail mode only) -->
    <div v-if="!isList" class="p-3 border-t border-base-300 bg-base-200/30 shrink-0">
      <router-link :to="`/things/${single.id}`" class="btn btn-sm btn-ghost w-full">
        View Details
      </router-link>
    </div>
  </div>
</template>
