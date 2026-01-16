<template>
  <div class="marker-item-switch">
    <span class="switch-label">{{ item.label }}</span>
    
    <div class="switch-control">
      <button 
        class="switch-track"
        :class="[`state-${state}`]"
        :disabled="!natsStore.isConnected || isPending"
        @click="handleToggle"
      >
        <span class="switch-thumb"></span>
      </button>
      
      <span class="switch-state-text" :class="[`state-${state}`]">
        {{ stateLabel }}
      </span>
    </div>
    
    <div v-if="error" class="switch-error">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { useSwitchState } from '@/composables/useSwitchState'
import { resolveTemplate } from '@/utils/variables'
import type { MapMarkerItem } from '@/types/dashboard'

/**
 * Marker Item: Switch Toggle
 * 
 * Reuses useSwitchState composable for KV or Core mode switching.
 * Handles its own subscription lifecycle.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()

// Get confirm function from parent if available
const requestConfirm = inject<(title: string, message: string, onConfirm: () => void, confirmText?: string) => void>('requestConfirm')

// Build resolved config from item.switchConfig
const switchConfig = computed(() => {
  if (!props.item.switchConfig) {
    return {
      mode: 'kv' as const,
      kvBucket: '',
      kvKey: '',
      publishSubject: '',
      onPayload: { state: 'on' },
      offPayload: { state: 'off' },
      defaultState: 'off' as const
    }
  }
  
  const vars = dashboardStore.currentVariableValues
  const cfg = props.item.switchConfig
  
  return {
    mode: cfg.mode,
    kvBucket: resolveTemplate(cfg.kvBucket, vars),
    kvKey: resolveTemplate(cfg.kvKey, vars),
    publishSubject: resolveTemplate(cfg.publishSubject, vars),
    stateSubject: resolveTemplate(cfg.stateSubject, vars),
    onPayload: cfg.onPayload,
    offPayload: cfg.offPayload,
    defaultState: 'off' as const
  }
})

// Use the switch state composable - handles subscription lifecycle
const { state, error, isPending, toggle } = useSwitchState(switchConfig)

const stateLabel = computed(() => {
  const labels = props.item.switchConfig?.labels
  switch (state.value) {
    case 'on': return labels?.on || 'ON'
    case 'off': return labels?.off || 'OFF'
    case 'pending': return '...'
    default: return '?'
  }
})

function handleToggle() {
  if (props.item.switchConfig?.confirmOnChange && requestConfirm) {
    const action = state.value === 'on' ? 'turn off' : 'turn on'
    requestConfirm(
      'Confirm Toggle',
      `Are you sure you want to ${action} "${props.item.label}"?`,
      () => toggle(),
      'Confirm'
    )
  } else {
    toggle()
  }
}
</script>

<style scoped>
.marker-item-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}

.switch-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.switch-control {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.switch-track {
  width: 36px;
  height: 20px;
  background: var(--muted);
  border-radius: 10px;
  position: relative;
  border: none;
  cursor: pointer;
  transition: background 0.3s;
  padding: 0;
}

.switch-track:hover:not(:disabled) {
  filter: brightness(1.1);
}

.switch-track:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.switch-track.state-on {
  background: var(--color-success);
}

.switch-track.state-pending {
  background: var(--color-warning);
  opacity: 0.7;
}

.switch-thumb {
  width: 16px;
  height: 16px;
  background: white;
  border-radius: 50%;
  position: absolute;
  top: 2px;
  left: 2px;
  transition: transform 0.3s cubic-bezier(0.4, 0.0, 0.2, 1);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
}

.switch-track.state-on .switch-thumb {
  transform: translateX(16px);
}

.switch-state-text {
  font-size: 11px;
  font-family: var(--mono);
  font-weight: 600;
  color: var(--muted);
  min-width: 24px;
  text-align: right;
}

.switch-state-text.state-on {
  color: var(--color-success);
}

.switch-state-text.state-pending {
  color: var(--color-warning);
}

.switch-error {
  font-size: 10px;
  color: var(--color-error);
  padding: 2px 6px;
  background: var(--color-error-bg);
  border-radius: 3px;
  margin-top: 4px;
  width: 100%;
}
</style>
