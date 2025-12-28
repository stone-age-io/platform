<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useFloorPlan } from '@/composables/useFloorPlan'
import { pb } from '@/utils/pb'

const props = defineProps<{
  location: any,
  things: any[],
  editable: boolean
}>()

const emit = defineEmits(['thing-moved', 'uploaded'])
const { initFloorPlan, renderMarkers } = useFloorPlan()
const loading = ref(false)

const loadMap = () => {
  if (!props.location.floorplan) return
  loading.value = true
  
  const imageUrl = pb.files.getUrl(props.location, props.location.floorplan)
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

const handleUpload = async (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  loading.value = true
  const fd = new FormData()
  fd.append('floorplan', file)
  try {
    const updated = await pb.collection('locations').update(props.location.id, fd)
    emit('uploaded', updated)
    loadMap()
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

onMounted(loadMap)
watch(() => props.things, updateMarkers, { deep: true })
watch(() => props.editable, updateMarkers)
</script>

<template>
  <div class="relative w-full h-[500px] bg-base-300 rounded-xl overflow-hidden border border-base-300 shadow-inner">
    <div v-if="!location.floorplan" class="absolute inset-0 flex flex-col items-center justify-center p-6 text-center">
      <span class="text-6xl mb-4">🖼️</span>
      <h3 class="font-bold">No Floor Plan Available</h3>
      <p class="text-sm opacity-60 mb-4">Upload an image to start mapping devices.</p>
      <input type="file" class="file-input file-input-bordered file-input-sm w-full max-w-xs" @change="handleUpload" />
    </div>

    <div v-if="loading" class="absolute inset-0 z-20 bg-base-300/50 backdrop-blur-sm flex items-center justify-center">
      <span class="loading loading-spinner loading-lg"></span>
    </div>

    <div id="floorplan-container" class="w-full h-full z-10"></div>
  </div>
</template>
