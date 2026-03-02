<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { pb } from '@/utils/pb'
import type { Location, Thing } from '@/types/pocketbase'
import type { Column } from '@/components/ui/ResponsiveList.vue'
import ResponsiveList from '@/components/ui/ResponsiveList.vue'

const props = defineProps<{
  location: Location
  isMobile: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const router = useRouter()

const activeTab = ref<'sub-locations' | 'things'>('sub-locations')
const subLocations = ref<Location[]>([])
const things = ref<Thing[]>([])
const loadingSubs = ref(false)
const loadingThings = ref(false)

const subLocColumns: Column<Location>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
]

const thingColumns: Column<Thing>[] = [
  { key: 'name', label: 'Name', mobileLabel: 'Name' },
  { key: 'expand.type.name', label: 'Type', mobileLabel: 'Type' },
  { key: 'code', label: 'Code', mobileLabel: 'Code' },
]

async function loadData() {
  loadingSubs.value = true
  loadingThings.value = true

  try {
    const [subs, t] = await Promise.all([
      pb.collection('locations').getFullList<Location>({
        filter: `parent = "${props.location.id}"`,
        expand: 'type',
        sort: 'name',
        perPage: 50,
      }),
      pb.collection('things').getFullList<Thing>({
        filter: `location = "${props.location.id}"`,
        expand: 'type',
        sort: 'name',
        perPage: 50,
      })
    ])
    subLocations.value = subs
    things.value = t
  } catch (err) {
    console.error('[LocationMapDrawer] Failed to load data:', err)
  } finally {
    loadingSubs.value = false
    loadingThings.value = false
  }
}

function handleSubLocClick(item: Location) {
  router.push(`/locations/${item.id}`)
}

function handleThingClick(item: Thing) {
  router.push(`/things/${item.id}`)
}

watch(() => props.location.id, () => {
  loadData()
})

onMounted(() => {
  loadData()
})
</script>

<template>
  <div
    class="absolute top-0 bottom-0 right-0 z-[500] flex flex-col bg-base-100 border-l border-base-300 shadow-xl"
    :class="isMobile ? 'left-0 w-full border-l-0' : 'w-80'"
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/30 shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-lg shrink-0">📍</span>
        <div class="min-w-0">
          <h3 class="font-bold text-sm truncate">{{ location.name }}</h3>
          <span v-if="location.expand?.type" class="text-xs text-base-content/60">{{ location.expand.type.name }}</span>
        </div>
      </div>
      <button class="btn btn-sm btn-circle btn-ghost shrink-0" @click="emit('close')">✕</button>
    </div>

    <!-- Tabs -->
    <div class="tabs tabs-bordered shrink-0 px-2">
      <button
        class="tab tab-sm"
        :class="{ 'tab-active': activeTab === 'sub-locations' }"
        @click="activeTab = 'sub-locations'"
      >
        Sub-Locations
        <span v-if="subLocations.length" class="badge badge-xs ml-1">{{ subLocations.length }}</span>
      </button>
      <button
        class="tab tab-sm"
        :class="{ 'tab-active': activeTab === 'things' }"
        @click="activeTab = 'things'"
      >
        Things
        <span v-if="things.length" class="badge badge-xs ml-1">{{ things.length }}</span>
      </button>
    </div>

    <!-- Tab Content -->
    <div class="flex-1 overflow-y-auto">
      <!-- Sub-Locations Tab -->
      <div v-if="activeTab === 'sub-locations'">
        <ResponsiveList
          :items="subLocations"
          :columns="subLocColumns"
          :loading="loadingSubs"
          @row-click="handleSubLocClick"
        >
          <template #cell-code="{ item }">
            <code class="text-xs font-mono">{{ item.code || '-' }}</code>
          </template>
          <template #empty>
            <div class="flex flex-col items-center gap-2 opacity-40">
              <span class="text-3xl">📭</span>
              <span class="text-xs font-bold uppercase tracking-widest">No sub-locations</span>
            </div>
          </template>
        </ResponsiveList>
      </div>

      <!-- Things Tab -->
      <div v-if="activeTab === 'things'">
        <ResponsiveList
          :items="things"
          :columns="thingColumns"
          :loading="loadingThings"
          @row-click="handleThingClick"
        >
          <template #cell-code="{ item }">
            <code class="text-xs font-mono">{{ item.code || '-' }}</code>
          </template>
          <template #empty>
            <div class="flex flex-col items-center gap-2 opacity-40">
              <span class="text-3xl">📭</span>
              <span class="text-xs font-bold uppercase tracking-widest">No things</span>
            </div>
          </template>
        </ResponsiveList>
      </div>
    </div>

    <!-- Footer -->
    <div class="p-3 border-t border-base-300 bg-base-200/30 shrink-0">
      <router-link
        :to="`/locations/${location.id}`"
        class="btn btn-sm btn-ghost w-full"
      >
        View Details
      </router-link>
    </div>
  </div>
</template>
