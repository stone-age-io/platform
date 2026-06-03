<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useFloorPlan } from '@/composables/useFloorPlan'
import { pb } from '@/utils/pb'
import type { Thing, Location } from '@/types/pocketbase'
import ThingMapDrawer from '@/components/things/ThingMapDrawer.vue'
import FloorPlanPositionDrawer from '@/components/map/FloorPlanPositionDrawer.vue'

const props = defineProps<{
  location: Location
  things: Thing[]
}>()

const emit = defineEmits<{
  'update-position': [payload: { id: string; position: { x: number; y: number } | null }]
}>()

const { initFloorPlan, renderMarkers, setSelected, getViewCenter } = useFloorPlan()

const loading = ref(false)
const positionMode = ref(false)        // markers draggable; persists across drawer open/close
const showPositionDrawer = ref(false)  // the positioning panel
const selectedThingId = ref<string | null>(null) // the detail panel (view mode)
const isMobile = ref(false)

function isPlaced(t: Thing): boolean {
  const p = t.floorplan_position
  return !!p && typeof p.x === 'number' && typeof p.y === 'number'
}

const placedThings = computed(() => props.things.filter(isPlaced))
const unmappedThings = computed(() => props.things.filter(t => !isPlaced(t)))
const selectedThing = computed(() => props.things.find(t => t.id === selectedThingId.value) || null)

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

const loadMap = () => {
  // Reset interaction state when the floor plan changes (e.g. navigating locations).
  selectedThingId.value = null
  showPositionDrawer.value = false
  positionMode.value = false

  if (!props.location?.floorplan) return
  loading.value = true

  const imageUrl = pb.files.getURL(props.location, props.location.floorplan)
  const img = new Image()
  img.onload = () => {
    initFloorPlan('floorplan-container', imageUrl, img.width, img.height)
    renderAll()
    loading.value = false
  }
  img.src = imageUrl
}

function renderAll() {
  renderMarkers(placedThings.value, {
    draggable: positionMode.value,
    onMove: (id, x, y) => emit('update-position', { id, position: { x, y } }),
    onClick: handleMarkerClick,
  })
  setSelected(selectedThingId.value)
}

// In view mode (not arranging, no panel open), clicking a marker shows its detail.
function handleMarkerClick(id: string) {
  if (positionMode.value || showPositionDrawer.value) return
  if (selectedThingId.value === id) {
    closeDetail()
    return
  }
  selectedThingId.value = id
  setSelected(id)
}

function closeDetail() {
  selectedThingId.value = null
  setSelected(null)
}

function togglePositionDrawer() {
  if (showPositionDrawer.value) {
    showPositionDrawer.value = false
    return
  }
  closeDetail()
  showPositionDrawer.value = true
}

function place(id: string) {
  emit('update-position', { id, position: getViewCenter() })
  if (isMobile.value) showPositionDrawer.value = false // free the map to drag on mobile
}

function unmap(id: string) {
  if (selectedThingId.value === id) closeDetail()
  emit('update-position', { id, position: null })
}

// Click on the map background (not a marker or drawer) closes the detail panel.
function handleMapBgClick(event: MouseEvent) {
  if (!selectedThingId.value) return
  const target = event.target as HTMLElement
  if (target.closest('.leaflet-marker-icon') || target.closest('.floorplan-drawer')) return
  closeDetail()
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  loadMap()
})
onUnmounted(() => window.removeEventListener('resize', checkMobile))

watch(() => props.things, renderAll, { deep: true })
watch(positionMode, renderAll)
watch(() => props.location?.floorplan, loadMap)
</script>

<template>
  <div class="relative w-full h-full min-h-[500px] bg-base-300 rounded-xl overflow-hidden border border-base-300 shadow-inner">
    <div v-if="loading" class="absolute inset-0 z-20 bg-base-300/50 backdrop-blur-sm flex items-center justify-center">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div id="floorplan-container" class="absolute inset-0 z-10" @click="handleMapBgClick"></div>

    <!-- Overlay control (top-right), matching the map-viz pattern -->
    <div class="absolute top-[10px] right-[10px] z-[400] flex flex-col gap-2">
      <button
        class="btn btn-sm shadow-sm gap-1"
        :class="(showPositionDrawer || positionMode) ? 'btn-primary' : 'bg-base-100 border-base-300 hover:bg-base-200'"
        @click="togglePositionDrawer"
        title="Position things"
      >
        🛠️ <span class="hidden sm:inline">Position</span>
      </button>
    </div>

    <!-- Detail drawer (view mode) -->
    <ThingMapDrawer
      v-if="selectedThing"
      :things="selectedThing ? [selectedThing] : []"
      :is-mobile="isMobile"
      class="floorplan-drawer"
      @close="closeDetail"
    />

    <!-- Positioning drawer -->
    <FloorPlanPositionDrawer
      v-if="showPositionDrawer"
      :unmapped="unmappedThings"
      :placed="placedThings"
      :position-mode="positionMode"
      :is-mobile="isMobile"
      class="floorplan-drawer"
      @close="showPositionDrawer = false"
      @update:position-mode="positionMode = $event"
      @place="place"
      @unmap="unmap"
    />
  </div>
</template>
