<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { pb } from '@/utils/pb'
import type { Location, Thing } from '@/types/pocketbase'

/**
 * A location folded onto a site's pin, with its depth below that site.
 *
 * Depth is relative to the site rather than absolute, because the site is the
 * root of what this pin represents -- an intermediate ancestor with no
 * coordinates is skipped on the map, so indenting to an absolute depth would
 * start the list at an arbitrary offset with nothing above it to explain it.
 */
export interface SiteDescendant {
  location: Location
  depth: number
}

const props = defineProps<{
  /** One site in detail mode; the cluster's sites in list mode. */
  locations: Location[]
  /** The selected site's subtree, in tree order. Empty in list mode. */
  descendants: SiteDescendant[]
  isMobile: boolean
}>()

const emit = defineEmits<{
  close: []
  select: [locationId: string]
}>()

const router = useRouter()

const isList = computed(() => props.locations.length > 1)
const single = computed(() => props.locations[0])

const activeTab = ref<'sub-locations' | 'things'>('sub-locations')
const things = ref<Thing[]>([])
const loadingThings = ref(false)

/**
 * Things anywhere in the site's subtree, not just on the site record itself.
 *
 * The pin represents the whole subtree, so "what is at this building" has to
 * include the rooms -- a sensor is almost never attached to the building
 * record. Chunked because the filter rides in a query string: a site with a few
 * hundred rooms would otherwise build a URL long enough for a proxy to reject,
 * and it would present as an empty Things tab rather than as an error.
 */
const THINGS_FILTER_CHUNK = 50

async function loadThings() {
  const site = single.value
  if (isList.value || !site) {
    things.value = []
    return
  }

  loadingThings.value = true
  try {
    const ids = [site.id, ...props.descendants.map((d) => d.location.id)]
    const chunks: string[][] = []
    for (let i = 0; i < ids.length; i += THINGS_FILTER_CHUNK) {
      chunks.push(ids.slice(i, i + THINGS_FILTER_CHUNK))
    }

    const pages = await Promise.all(
      chunks.map((chunk) => {
        const params: Record<string, string> = {}
        const expr = chunk
          .map((id, i) => {
            params[`l${i}`] = id
            return `location = {:l${i}}`
          })
          .join(' || ')
        return pb.collection('things').getFullList<Thing>({
          filter: pb.filter(expr, params),
          expand: 'type,location',
          sort: 'name',
        })
      }),
    )

    things.value = pages
      .flat()
      .sort((a, b) => (a.name || '').localeCompare(b.name || ''))
  } catch (err) {
    console.error('[LocationMapDrawer] Failed to load things:', err)
    things.value = []
  } finally {
    loadingThings.value = false
  }
}

function goToLocation(id: string) {
  router.push(`/locations/${id}`)
}

function goToThing(id: string) {
  router.push(`/things/${id}`)
}

/**
 * Keyed on the site id in detail mode and on null in list mode -- NOT on the id
 * alone, which silently skipped the fetch in the one path that matters most.
 * `single` is `locations[0]`, so picking the first site out of the cluster it
 * already headed leaves the id unchanged while the mode flips from list to
 * detail: the watcher never fired and the Things tab kept the empty array that
 * list mode had just set. It failed as "No things" on a site that has several.
 *
 * `immediate` covers the mount, so the two entry paths cannot drift apart.
 */
watch(
  () => (isList.value ? null : single.value?.id),
  () => {
    activeTab.value = 'sub-locations'
    loadThings()
  },
  { immediate: true },
)
</script>

<template>
  <div
    class="absolute top-0 bottom-0 right-0 z-[500] flex flex-col bg-base-100 border-l border-base-300 shadow-xl"
    :class="isMobile ? 'left-0 w-full border-l-0' : 'w-80'"
  >
    <!-- Header (cluster / list mode) -->
    <div v-if="isList" class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/30 shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-lg shrink-0">📍</span>
        <div class="min-w-0">
          <h3 class="font-bold text-sm truncate">{{ locations.length }} sites</h3>
          <span class="text-xs text-base-content/60">in this cluster</span>
        </div>
      </div>
      <button class="btn btn-sm btn-circle btn-ghost shrink-0" aria-label="Close" @click="emit('close')">✕</button>
    </div>

    <!-- Header (single / detail mode) -->
    <div v-else class="flex items-center justify-between p-4 border-b border-base-300 bg-base-200/30 shrink-0">
      <div class="flex items-center gap-3 min-w-0">
        <span class="text-lg shrink-0">📍</span>
        <div class="min-w-0">
          <h3 class="font-bold text-sm truncate">{{ single.name || 'Unnamed' }}</h3>
          <span v-if="single.expand?.type" class="text-xs text-base-content/60">{{ single.expand.type.name }}</span>
        </div>
      </div>
      <button class="btn btn-sm btn-circle btn-ghost shrink-0" aria-label="Close" @click="emit('close')">✕</button>
    </div>

    <!-- Body (cluster / list mode) -->
    <div v-if="isList" class="flex-1 overflow-y-auto">
      <button
        v-for="l in locations"
        :key="l.id"
        class="w-full text-left p-3 border-b border-base-300 hover:bg-base-200 transition-colors flex items-center justify-between gap-2 min-w-0"
        @click="emit('select', l.id)"
      >
        <div class="min-w-0 flex-1">
          <div class="font-medium text-sm truncate">{{ l.name || 'Unnamed' }}</div>
          <div class="flex items-center gap-2 mt-0.5">
            <span v-if="l.expand?.type" class="badge badge-ghost badge-xs">{{ l.expand.type.name }}</span>
            <code v-if="l.code" class="text-[10px] text-base-content/60">{{ l.code }}</code>
          </div>
        </div>
        <span class="text-base-content/40 text-xs shrink-0">›</span>
      </button>
    </div>

    <!-- Body (single / detail mode) -->
    <template v-else>
      <!-- Tabs -->
      <div class="tabs tabs-bordered shrink-0 px-2">
        <button
          class="tab tab-sm"
          :class="{ 'tab-active': activeTab === 'sub-locations' }"
          @click="activeTab = 'sub-locations'"
        >
          Sub-Locations
          <span v-if="descendants.length" class="badge badge-xs ml-1">{{ descendants.length }}</span>
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

      <div class="flex-1 overflow-y-auto">
        <!-- Sub-Locations Tab: the whole subtree, indented -->
        <div v-if="activeTab === 'sub-locations'">
          <button
            v-for="d in descendants"
            :key="d.location.id"
            class="w-full text-left p-3 border-b border-base-300 hover:bg-base-200 transition-colors flex items-center justify-between gap-2 min-w-0"
            :style="{ paddingLeft: `${0.75 + d.depth * 0.875}rem` }"
            @click="goToLocation(d.location.id)"
          >
            <div class="min-w-0 flex-1">
              <div class="font-medium text-sm truncate">
                <span v-if="d.depth > 0" class="text-base-content/30 mr-1">└</span>{{ d.location.name || 'Unnamed' }}
              </div>
              <div class="flex items-center gap-2 mt-0.5">
                <span v-if="d.location.expand?.type" class="badge badge-ghost badge-xs">{{ d.location.expand.type.name }}</span>
                <code v-if="d.location.code" class="text-[10px] text-base-content/60">{{ d.location.code }}</code>
              </div>
            </div>
            <span class="text-base-content/40 text-xs shrink-0">›</span>
          </button>

          <div v-if="descendants.length === 0" class="flex flex-col items-center gap-2 opacity-40 py-10">
            <span class="text-3xl">📭</span>
            <span class="text-xs font-bold uppercase tracking-widest">No sub-locations</span>
          </div>
        </div>

        <!-- Things Tab: everything in the subtree -->
        <div v-if="activeTab === 'things'">
          <div v-if="loadingThings" class="flex justify-center py-10">
            <span class="loading loading-spinner loading-md text-primary"></span>
          </div>

          <template v-else>
            <button
              v-for="t in things"
              :key="t.id"
              class="w-full text-left p-3 border-b border-base-300 hover:bg-base-200 transition-colors flex items-center justify-between gap-2 min-w-0"
              @click="goToThing(t.id)"
            >
              <div class="min-w-0 flex-1">
                <div class="font-medium text-sm truncate">{{ t.name || 'Unnamed' }}</div>
                <!-- Where it actually is, when that is not the site itself. This
                     tab now spans the subtree, so a bare name would leave "which
                     room" unanswerable from the drawer. -->
                <div
                  v-if="t.location && t.location !== single.id && t.expand?.location"
                  class="text-[11px] text-base-content/60 truncate"
                >
                  📍 {{ (t.expand.location as Location).name }}
                </div>
                <div class="flex items-center gap-2 mt-0.5">
                  <span v-if="t.expand?.type" class="badge badge-ghost badge-xs">{{ t.expand.type.name }}</span>
                  <code v-if="t.code" class="text-[10px] text-base-content/60">{{ t.code }}</code>
                </div>
              </div>
              <span class="text-base-content/40 text-xs shrink-0">›</span>
            </button>

            <div v-if="things.length === 0" class="flex flex-col items-center gap-2 opacity-40 py-10">
              <span class="text-3xl">📭</span>
              <span class="text-xs font-bold uppercase tracking-widest">No things</span>
            </div>
          </template>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-3 border-t border-base-300 bg-base-200/30 shrink-0">
        <router-link :to="`/locations/${single.id}`" class="btn btn-sm btn-ghost w-full">
          View Details
        </router-link>
      </div>
    </template>
  </div>
</template>
