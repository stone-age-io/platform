<template>
  <div class="widget-state" :class="state">
    <template v-if="state === 'loading'">
      <LoadingState size="small" inline />
      <div v-if="message && !compact" class="state-message">{{ message }}</div>
    </template>

    <template v-else-if="state === 'error'">
      <div class="state-icon">⚠️</div>
      <div v-if="!compact" class="state-message">{{ message || 'Error' }}</div>
      <button
        v-if="$attrs.onRetry"
        class="state-retry"
        @click="$emit('retry')"
      >Retry</button>
    </template>

    <template v-else-if="state === 'empty'">
      <div class="state-icon">{{ icon || '∅' }}</div>
      <div v-if="message && !compact" class="state-message">{{ message }}</div>
    </template>
  </div>
</template>

<script setup lang="ts">
import LoadingState from '@/components/common/LoadingState.vue'

defineProps<{
  state: 'loading' | 'error' | 'empty'
  message?: string
  icon?: string
  compact?: boolean
}>()

defineEmits<{
  retry: []
}>()

defineOptions({ inheritAttrs: false })
</script>

<style scoped>
.widget-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  padding: 16px;
}

.widget-state.error { color: var(--color-error, oklch(var(--er))); }
.widget-state.empty { color: var(--muted, oklch(var(--bc) / 0.4)); }

.state-icon {
  font-size: 28px;
  line-height: 1;
}

.state-message {
  font-size: 13px;
  line-height: 1.4;
  max-width: 80%;
  word-wrap: break-word;
}

.state-retry {
  margin-top: 4px;
  padding: 4px 10px;
  border: 1px solid currentColor;
  background: transparent;
  color: inherit;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  opacity: 0.8;
}

.state-retry:hover {
  opacity: 1;
  background: rgba(255, 255, 255, 0.05);
}
</style>
