<template>
  <div class="marker-item-publish">
    <button 
      class="publish-btn"
      :class="{ 'is-success': showSuccess, 'is-loading': isLoading }"
      :disabled="!natsStore.isConnected || isLoading"
      @click="handlePublish"
    >
      <span v-if="isLoading" class="btn-spinner"></span>
      <span v-else-if="showSuccess" class="btn-icon">✓</span>
      <span class="btn-label">{{ showSuccess ? 'Sent!' : item.label }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useNatsStore } from '@/stores/nats'
import { useDashboardStore } from '@/stores/dashboard'
import { encodeString, decodeBytes } from '@/utils/encoding'
import { resolveTemplate } from '@/utils/variables'
import type { MapMarkerItem } from '@/types/dashboard'

/**
 * Marker Item: Publish Button
 * 
 * Handles fire-and-forget publish or request/reply pattern.
 * Emits response event for request/reply results.
 */

const props = defineProps<{
  item: MapMarkerItem
}>()

const emit = defineEmits<{
  response: [data: { success: boolean; data: string; latency: number }]
}>()

const natsStore = useNatsStore()
const dashboardStore = useDashboardStore()

const isLoading = ref(false)
const showSuccess = ref(false)

async function handlePublish() {
  if (!natsStore.nc || !natsStore.isConnected) return
  
  const subject = resolveTemplate(props.item.subject, dashboardStore.currentVariableValues)
  const payloadStr = resolveTemplate(props.item.payload || '{}', dashboardStore.currentVariableValues)
  
  if (!subject) return
  
  const payload = encodeString(payloadStr)
  
  if (props.item.actionType === 'request') {
    // Request/Reply pattern
    isLoading.value = true
    const timeout = props.item.timeout || 1000
    const start = Date.now()
    
    try {
      const msg = await natsStore.nc.request(subject, payload, { timeout })
      const latency = Date.now() - start
      
      emit('response', {
        success: true,
        data: decodeBytes(msg.data),
        latency
      })
      
      triggerSuccess()
    } catch (err: any) {
      emit('response', {
        success: false,
        data: err.message || 'Request failed',
        latency: Date.now() - start
      })
    } finally {
      isLoading.value = false
    }
  } else {
    // Fire and forget
    try {
      natsStore.nc.publish(subject, payload)
      triggerSuccess()
    } catch (err) {
      console.error('[MarkerItemPublish] Publish error:', err)
    }
  }
}

function triggerSuccess() {
  showSuccess.value = true
  setTimeout(() => {
    showSuccess.value = false
  }, 1500)
}
</script>

<style scoped>
.marker-item-publish {
  width: 100%;
}

.publish-btn {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 12px;
  background: var(--color-primary);
  color: white;
  border: none;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.publish-btn:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.publish-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.publish-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.publish-btn.is-success {
  background: var(--color-success);
}

.publish-btn.is-loading {
  opacity: 0.8;
}

.btn-icon {
  font-size: 14px;
}

.btn-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
