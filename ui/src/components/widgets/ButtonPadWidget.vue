<template>
  <div v-if="cells.length" class="button-pad" :style="gridStyle">
    <ButtonPadKey v-for="cell in cells" :key="`${cell.row}:${cell.col}`" :cell="cell" />
  </div>
  <div v-else class="button-pad-empty">
    <Grid3x3 :size="20" aria-hidden="true" />
    <span>No buttons. Configure the widget to add some.</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Grid3x3 } from '@lucide/vue'
import ButtonPadKey from './ButtonPadKey.vue'
import type { WidgetConfigOf } from '@/types/dashboard'
import { padSize, padCells } from '@/utils/buttonPad'

// Same layout on desktop and phone, so there is no card mode to switch on.
const props = defineProps<{ config: WidgetConfigOf<'buttonpad'> }>()

const size = computed(() => padSize(props.config.buttonpadConfig))
const cells = computed(() => padCells(props.config.buttonpadConfig))

const gridStyle = computed(() => ({
  gridTemplateRows: `repeat(${size.value.rows}, minmax(0, 1fr))`,
  gridTemplateColumns: `repeat(${size.value.cols}, minmax(0, 1fr))`,
}))
</script>

<style scoped>
.button-pad {
  display: grid;
  gap: 8px;
  padding: 8px;
  width: 100%;
  height: 100%;
}

.button-pad-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 100%;
  padding: 8px;
  text-align: center;
  font-size: 13px;
  color: var(--muted);
}
</style>
