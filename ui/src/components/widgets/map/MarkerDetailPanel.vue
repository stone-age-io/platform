<template>
  <div class="marker-detail-panel" :class="{ 'is-mobile': isMobile }">
    <!-- Header -->
    <div class="panel-header">
      <div class="header-content">
        <span class="marker-icon">📍</span>
        <h3 class="marker-label">{{ marker.label }}</h3>
      </div>
      <button class="close-btn" @click="$emit('close')" title="Close panel">
        ✕
      </button>
    </div>
    
    <!-- Items -->
    <div class="panel-body">
      <div v-if="marker.items.length === 0" class="empty-items">
        <span class="empty-icon">📭</span>
        <span class="empty-text">No items configured</span>
      </div>
      
      <div v-else class="items-list">
        <template v-for="item in marker.items" :key="item.id">
          <!-- Publish Button -->
          <MarkerItemPublish 
            v-if="item.type === 'publish'"
            :item="item"
            @response="handleResponse"
          />
          
          <!-- Switch Toggle -->
          <MarkerItemSwitch 
            v-else-if="item.type === 'switch'"
            :item="item"
          />
          
          <!-- Text Display -->
          <MarkerItemText 
            v-else-if="item.type === 'text'"
            :item="item"
          />
          
          <!-- KV Display -->
          <MarkerItemKv 
            v-else-if="item.type === 'kv'"
            :item="item"
          />
        </template>
      </div>
    </div>
    
    <!-- Footer with coordinates (if static) or subject (if dynamic) -->
    <div class="panel-footer">
      <template v-if="marker.positionConfig?.mode === 'dynamic'">
        <span class="footer-label">Subject:</span>
        <span class="footer-value mono">{{ marker.positionConfig.subject }}</span>
      </template>
      <template v-else>
        <span class="footer-label">Location:</span>
        <span class="footer-value mono">
          {{ marker.lat.toFixed(5) }}, {{ marker.lon.toFixed(5) }}
        </span>
      </template>
    </div>
    
    <!-- Response Modal for Request/Reply -->
    <ResponseModal
      v-model="responseModal.show"
      :title="responseModal.title"
      :status="responseModal.status"
      :data="responseModal.data"
      :latency="responseModal.latency"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import MarkerItemPublish from './MarkerItemPublish.vue'
import MarkerItemSwitch from './MarkerItemSwitch.vue'
import MarkerItemText from './MarkerItemText.vue'
import MarkerItemKv from './MarkerItemKv.vue'
import ResponseModal from '@/components/common/ResponseModal.vue'
import type { MapMarker } from '@/types/dashboard'

/**
 * Marker Detail Panel
 * 
 * Displays marker information and interactive items.
 * - Desktop: Right sidebar (280px wide)
 * - Mobile: Full overlay covering the map
 */

defineProps<{
  marker: MapMarker
  isMobile: boolean
}>()

defineEmits<{
  close: []
}>()

// Response modal state
const responseModal = ref({
  show: false,
  title: 'Response',
  status: 'success' as 'success' | 'error',
  data: '',
  latency: 0
})

function handleResponse(response: { success: boolean; data: string; latency: number }) {
  responseModal.value = {
    show: true,
    title: response.success ? 'Response' : 'Request Failed',
    status: response.success ? 'success' : 'error',
    data: response.data,
    latency: response.latency
  }
}
</script>

<style scoped>
/* Desktop: Right sidebar */
.marker-detail-panel {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 280px;
  background: var(--panel);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  z-index: 10;
  box-shadow: -4px 0 12px rgba(0, 0, 0, 0.2);
}

/* Mobile: Full overlay */
.marker-detail-panel.is-mobile {
  right: 0;
  left: 0;
  top: 0;
  bottom: 0;
  width: 100%;
  border-left: none;
  border-radius: 0;
  box-shadow: none;
}

/* Header */
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.1);
  flex-shrink: 0;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.marker-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.marker-label {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.close-btn {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: var(--muted);
  font-size: 18px;
  width: 32px;
  height: 32px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  flex-shrink: 0;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.15);
  color: var(--text);
}

/* Body */
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}

.empty-items {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 16px;
  color: var(--muted);
  text-align: center;
}

.empty-icon {
  font-size: 32px;
  opacity: 0.5;
}

.empty-text {
  font-size: 14px;
}

.items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* Footer */
.panel-footer {
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.05);
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.footer-label {
  font-size: 11px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.footer-value {
  font-size: 12px;
  color: var(--text);
}

.footer-value.mono {
  font-family: var(--mono);
}
</style>
