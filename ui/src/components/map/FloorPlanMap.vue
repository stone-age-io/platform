<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useFloorPlan } from '@/composables/useFloorPlan'
import { pb } from '@/utils/pb'

const props = defineProps<{
  location: any,
  things: any[],
  editable: boolean
}>()

const emit = defineEmits(['thing-moved'])
const { initFloorPlan, renderMarkers } = useFloorPlan()
const loading = ref(false)

const loadMap = () => {
  if (!props.location?.floorplan) return
  loading.value = true

  const imageUrl = pb.files.getURL(props.location, props.location.floorplan)
  const img = new Image()
  img.onload = () => {
    initFloorPlan('floorplan-container', imageUrl, img.width, img.height)
    updateMarkers()
    loading.value = false
  }
  img.src = imageUrl
}

const updateMarkers = () => {
  renderMarkers(props.things, props.editable, (id, x, y) => {
    emit('thing-moved', { id, x, y })
  })
}

onMounted(loadMap)
watch(() => props.things, updateMarkers, { deep: true })
watch(() => props.editable, updateMarkers)
watch(() => props.location?.floorplan, loadMap)
</script>

<template>
  <div class="relative w-full h-full min-h-[500px] bg-base-300 rounded-xl overflow-hidden border border-base-300 shadow-inner">
    <div v-if="loading" class="absolute inset-0 z-20 bg-base-300/50 backdrop-blur-sm flex items-center justify-center">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div id="floorplan-container" class="absolute inset-0 z-10"></div>
  </div>
</template>
