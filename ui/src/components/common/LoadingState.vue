<!-- src/components/common/LoadingState.vue -->
<template>
  <div class="loading-state" :class="[size, { inline }]">
    <div class="spinner" :class="spinnerVariant">
      <div class="spinner-ring"></div>
      <div class="spinner-ring"></div>
      <div class="spinner-ring"></div>
    </div>
    <div v-if="message" class="loading-message">
      {{ message }}
    </div>
    <div v-if="submessage" class="loading-submessage">
      {{ submessage }}
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * Loading State Component
 * 
 * Displays a loading spinner with optional message.
 * Can be used inline or as a centered overlay.
 * 
 * Usage:
 * <LoadingState message="Loading dashboards..." />
 * <LoadingState size="small" inline />
 * <LoadingState message="Connecting..." submessage="This may take a moment..." />
 */

interface Props {
  message?: string
  submessage?: string
  size?: 'small' | 'medium' | 'large'
  inline?: boolean
  variant?: 'default' | 'primary' | 'accent'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'medium',
  inline: false,
  variant: 'default'
})

const spinnerVariant = computed(() => `spinner-${props.variant}`)
</script>

<script lang="ts">
import { computed } from 'vue'
export default { name: 'LoadingState' }
</script>

<style scoped>
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 32px;
}

.loading-state.inline {
  padding: 8px;
  gap: 8px;
}

/* Spinner sizes */
.loading-state.small .spinner {
  width: 24px;
  height: 24px;
}

.loading-state.medium .spinner {
  width: 40px;
  height: 40px;
}

.loading-state.large .spinner {
  width: 64px;
  height: 64px;
}

/* Spinner base */
.spinner {
  position: relative;
  width: 40px;
  height: 40px;
}

.spinner-ring {
  position: absolute;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  border: 3px solid transparent;
  animation: spin 1.5s cubic-bezier(0.5, 0, 0.5, 1) infinite;
}

/* Default variant (accent color) */
.spinner-default .spinner-ring:nth-child(1) {
  border-top-color: var(--accent, #58a6ff);
  animation-delay: -0.45s;
}

.spinner-default .spinner-ring:nth-child(2) {
  border-top-color: var(--accent, #58a6ff);
  opacity: 0.6;
  animation-delay: -0.3s;
}

.spinner-default .spinner-ring:nth-child(3) {
  border-top-color: var(--accent, #58a6ff);
  opacity: 0.3;
  animation-delay: -0.15s;
}

/* Primary variant */
.spinner-primary .spinner-ring:nth-child(1) {
  border-top-color: var(--primary, #3fb950);
  animation-delay: -0.45s;
}

.spinner-primary .spinner-ring:nth-child(2) {
  border-top-color: var(--primary, #3fb950);
  opacity: 0.6;
  animation-delay: -0.3s;
}

.spinner-primary .spinner-ring:nth-child(3) {
  border-top-color: var(--primary, #3fb950);
  opacity: 0.3;
  animation-delay: -0.15s;
}

/* Accent variant */
.spinner-accent .spinner-ring:nth-child(1) {
  border-top-color: var(--accent, #58a6ff);
  animation-delay: -0.45s;
}

.spinner-accent .spinner-ring:nth-child(2) {
  border-top-color: var(--accent, #58a6ff);
  opacity: 0.6;
  animation-delay: -0.3s;
}

.spinner-accent .spinner-ring:nth-child(3) {
  border-top-color: var(--accent, #58a6ff);
  opacity: 0.3;
  animation-delay: -0.15s;
}

@keyframes spin {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Messages */
.loading-message {
  font-size: 16px;
  font-weight: 500;
  color: var(--text, #e0e0e0);
  text-align: center;
}

.loading-submessage {
  font-size: 13px;
  color: var(--muted, #888);
  text-align: center;
  max-width: 300px;
  line-height: 1.4;
}

/* Inline variant adjustments */
.loading-state.inline .loading-message {
  font-size: 14px;
}

.loading-state.inline .loading-submessage {
  font-size: 12px;
}

/* Small size text adjustments */
.loading-state.small .loading-message {
  font-size: 14px;
}

.loading-state.small .loading-submessage {
  font-size: 11px;
}
</style>
