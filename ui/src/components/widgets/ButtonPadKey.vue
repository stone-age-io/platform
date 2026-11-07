<template>
  <button
    type="button"
    class="pad-key"
    :class="{ 'is-sent': flash === 'success', 'is-error': flash === 'error' }"
    :style="keyStyle"
    :disabled="!natsStore.isConnected"
    :title="natsStore.isConnected ? undefined : 'Offline'"
    @click="publish(cell.subject, cell.payload || '{}')"
  >
    {{ cell.label }}
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { usePublishAction } from '@/composables/usePublishAction'
import type { ButtonPadCell } from '@/types/dashboard'

const props = defineProps<{ cell: ButtonPadCell }>()

const natsStore = useNatsStore()
// Short, and the key stays enabled: a remote is pressed several times in a row,
// and ButtonWidget's 1.5s lockout would swallow the second press.
const { flash, publish } = usePublishAction({ flashMs: 250 })

const keyStyle = computed(() => {
  const bg = props.cell.color || 'var(--color-primary)'
  return {
    gridRow: props.cell.row + 1,
    gridColumn: props.cell.col + 1,
    backgroundColor: bg,
    // The Neutral option is the panel colour, which is light in the light theme.
    color: bg === 'var(--panel)' ? 'var(--text)' : 'white',
  }
})
</script>

<style scoped>
.pad-key {
  min-width: 0;
  min-height: 0;
  border: 1px solid transparent;
  border-radius: 6px;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.2;
  padding: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  user-select: none;
  touch-action: manipulation; /* no double-tap zoom when a key is pressed twice */
  transition: filter 0.15s, transform 0.05s;
}

.pad-key:hover:not(:disabled) {
  filter: brightness(1.1);
}

.pad-key:active:not(:disabled) {
  transform: scale(0.97);
}

.pad-key:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pad-key.is-sent {
  filter: brightness(1.3);
}

.pad-key.is-error {
  background-color: var(--color-error) !important;
  color: white !important;
}
</style>
