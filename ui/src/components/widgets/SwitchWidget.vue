<template>
  <div class="switch-widget" :class="{ 'card-layout': layoutMode === 'card' }">
    <!-- Card Layout (Mobile) -->
    <template v-if="layoutMode === 'card'">
      <!-- Container lights up when ON -->
      <div 
        class="card-content" 
        :class="{ 'is-active': state === 'on' }"
      >
        <div class="card-info">
          <div class="card-title">{{ config.title }}</div>
          
          <!-- Subtle distinction: A tiny 'power' icon next to state -->
          <div class="card-state-row">
            <span class="card-state-icon">⏻</span>
            <span class="card-state">{{ currentStateLabel }}</span>
          </div>
        </div>
      </div>
      
      <button 
        class="card-overlay-btn"
        :disabled="isPending || !natsStore.isConnected"
        @click="toggle"
      ></button>
    </template>

    <!-- Standard Layout (Desktop) -->
    <template v-else>
      <div class="centered-content">
        <button 
          class="switch-btn"
          :class="['state-' + state, { pending: isPending }]"
          :disabled="isPending || !natsStore.isConnected"
          @click="toggle"
        >
          <!-- Big Icon for desktop -->
          <div class="icon-wrapper">
            <span class="icon">{{ state === 'on' ? '💡' : '🌑' }}</span>
          </div>
          <!-- Label below -->
          <div class="label">{{ currentStateLabel }}</div>
          
          <div v-if="isPending" class="spinner"></div>
        </button>
      </div>
    </template>
    
    <div v-if="error" class="error-toast">{{ error }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import { useSwitchState } from '@/composables/useSwitchState'
import { useDashboardStore } from '@/stores/dashboard'
import { useNatsStore } from '@/stores/nats'
import { resolveTemplate } from '@/utils/variables'
import type { WidgetConfig } from '@/types/dashboard'

const props = withDefaults(defineProps<{
  config: WidgetConfig
  layoutMode?: 'standard' | 'card'
}>(), {
  layoutMode: 'standard'
})

const dashboardStore = useDashboardStore()
const natsStore = useNatsStore()
const requestConfirm = inject('requestConfirm') as (title: string, message: string, onConfirm: () => void) => void

const cfg = computed(() => props.config.switchConfig!)

const resolvedConfig = computed(() => {
  const vars = dashboardStore.currentVariableValues
  return {
    mode: cfg.value.mode,
    kvBucket: resolveTemplate(cfg.value.kvBucket, vars),
    kvKey: resolveTemplate(cfg.value.kvKey, vars),
    publishSubject: resolveTemplate(cfg.value.publishSubject, vars),
    stateSubject: resolveTemplate(cfg.value.stateSubject, vars),
    onPayload: cfg.value.onPayload,
    offPayload: cfg.value.offPayload,
    defaultState: cfg.value.defaultState
  }
})

const { state, error, isPending, toggle: executeToggle } = useSwitchState(resolvedConfig)

const labels = computed(() => cfg.value.labels || { on: 'ON', off: 'OFF' })
const currentStateLabel = computed(() => {
  if (state.value === 'pending') return '...'
  return state.value === 'on' ? labels.value.on : labels.value.off
})

function toggle() {
  if (cfg.value.confirmOnChange) {
    requestConfirm(
      'Confirm Action',
      cfg.value.confirmMessage || `Switch ${state.value === 'on' ? 'OFF' : 'ON'}?`,
      () => executeToggle()
    )
  } else {
    executeToggle()
  }
}
</script>

<style scoped>
.switch-widget {
  height: 100%;
  width: 100%;
  position: relative;
}

/* --- CARD LAYOUT STYLES --- */
.switch-widget.card-layout {
  padding: 0; 
  display: flex;
}

.card-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  padding: 12px;
  transition: background-color 0.3s ease;
  border-radius: inherit; 
}

/* State: ON (Light up the background) */
.card-content.is-active {
  background-color: var(--color-warning-bg);
  box-shadow: inset 0 0 0 1px var(--color-warning-border);
}

/* State: OFF (Subtle border to show interactivity) */
.card-content:not(.is-active) {
  /* Subtle bottom border to imply 'toggle' vs flat button */
  border-bottom: 2px solid rgba(0,0,0,0.1);
}

.card-info {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  width: 100%;
  overflow: hidden;
  text-align: center;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
  margin-bottom: 2px;
}

.card-state-row {
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0.8;
}

.card-state-icon {
  font-size: 10px;
  margin-top: 1px;
}

.card-state {
  font-size: 13px;
  color: var(--muted);
  font-weight: 500;
}

/* Active State Text Color */
.card-content.is-active .card-state,
.card-content.is-active .card-state-icon {
  color: var(--color-warning);
}

.card-overlay-btn {
  position: absolute;
  inset: 0;
  background: transparent;
  border: none;
  cursor: pointer;
  z-index: 5;
}

.card-overlay-btn:disabled {
  cursor: not-allowed;
}

/* --- STANDARD LAYOUT STYLES (Desktop) --- */
.centered-content {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px;
}

.switch-btn {
  width: 100%;
  height: 100%;
  border: none;
  border-radius: 8px;
  background: var(--panel);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  transition: all 0.2s;
  position: relative;
  overflow: hidden;
  border: 1px solid var(--border);
}

.switch-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.05);
  border-color: var(--color-accent);
}

.switch-btn.state-on {
  border-color: var(--color-warning);
  background: var(--color-warning-bg);
}

.switch-btn.state-on .icon {
  filter: drop-shadow(0 0 8px var(--color-warning));
}

.switch-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.icon-wrapper {
  font-size: 24px;
  transition: transform 0.2s;
}

.switch-btn:active .icon-wrapper {
  transform: scale(0.9);
}

.label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.spinner {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 12px;
  height: 12px;
  border: 2px solid transparent;
  border-top-color: var(--text);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

.error-toast {
  position: absolute;
  bottom: 4px;
  left: 4px;
  right: 4px;
  background: var(--color-error-bg);
  color: var(--color-error);
  font-size: 10px;
  padding: 4px;
  border-radius: 4px;
  text-align: center;
  pointer-events: none;
}
</style>
