<template>
  <div class="config-map">
    <!-- Map View Settings -->
    <div class="config-section">
      <div class="section-header">
        <span class="section-icon">🗺️</span>
        <span class="section-title">Map View</span>
      </div>
      
      <div class="form-group">
        <label>Default Center</label>
        <div class="coord-inputs">
          <div class="coord-input-group">
            <label class="coord-label">Latitude</label>
            <input 
              v-model.number="form.mapCenterLat" 
              type="number" 
              class="form-input"
              placeholder="39.8283"
              step="any"
            />
          </div>
          <div class="coord-input-group">
            <label class="coord-label">Longitude</label>
            <input 
              v-model.number="form.mapCenterLon" 
              type="number" 
              class="form-input"
              placeholder="-98.5795"
              step="any"
            />
          </div>
          <div class="coord-input-group">
            <label class="coord-label">Zoom</label>
            <input 
              v-model.number="form.mapZoom" 
              type="number" 
              class="form-input"
              placeholder="4"
              min="1"
              max="19"
            />
          </div>
        </div>
        <div class="help-text">
          Initial view when map loads. Zoom: 1 (world) to 19 (street level).
        </div>
      </div>
    </div>

    <!-- Markers Section -->
    <div class="config-section">
      <div class="section-header">
        <span class="section-icon">📍</span>
        <span class="section-title">Markers</span>
        <span class="section-count">{{ form.mapMarkers.length }}/{{ MAX_MARKERS }}</span>
      </div>
      
      <div v-if="form.mapMarkers.length === 0" class="empty-markers">
        <div class="empty-icon">📍</div>
        <div class="empty-text">No markers configured</div>
        <div class="empty-hint">Add a marker to place interactive points on the map</div>
      </div>
      
      <div class="markers-list">
        <MarkerEditor
          v-for="(marker, index) in form.mapMarkers"
          :key="marker.id"
          :marker="marker"
          :item-errors="getMarkerItemErrors(index)"
          @remove="removeMarker(index)"
          @use-center="useMapCenterForMarker(index)"
          @update:marker="updateMarker(index, $event)"
        />
      </div>
      
      <!-- Marker limit warning -->
      <div v-if="isAtMarkerLimit" class="limit-warning">
        ⚠️ Maximum {{ MAX_MARKERS }} markers per map widget
      </div>
      
      <button 
        v-else
        class="btn-add-marker" 
        @click="addMarker"
      >
        <span class="btn-icon">+</span>
        <span class="btn-text">Add Marker</span>
      </button>
    </div>

    <!-- Tips -->
    <div class="tips-section">
      <div class="tip">
        <span class="tip-icon">💡</span>
        <span class="tip-text">
          Find coordinates using 
          <a href="https://www.google.com/maps" target="_blank" rel="noopener">Google Maps</a>
          (right-click → "What's here?")
        </span>
      </div>
      <div class="tip">
        <span class="tip-icon">🔄</span>
        <span class="tip-text">
          <strong>Switch items</strong> create live toggles. 
          State updates when popup is open.
        </span>
      </div>
      <div class="tip">
        <span class="tip-icon">📊</span>
        <span class="tip-text">
          <strong>Limits:</strong> {{ MAX_MARKERS }} markers per map, 
          {{ MAX_ITEMS_PER_MARKER }} items per marker.
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkerEditor from './MarkerEditor.vue'
import { createDefaultMarker, MAP_LIMITS } from '@/types/dashboard'
import type { MapMarker } from '@/types/dashboard'
import type { WidgetFormState } from '@/types/config'

/**
 * Map Widget Configuration Form
 * 
 * Grug say: Full V2 implementation.
 * - Multiple markers (max 50)
 * - Multiple items per marker (max 10)
 * - Publish, Switch, Text, KV item types
 */

// Use centralized limits from types
const MAX_MARKERS = MAP_LIMITS.MAX_MARKERS
const MAX_ITEMS_PER_MARKER = MAP_LIMITS.MAX_ITEMS_PER_MARKER

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const isAtMarkerLimit = computed(() => props.form.mapMarkers.length >= MAX_MARKERS)

/**
 * Add new marker
 */
function addMarker() {
  if (isAtMarkerLimit.value) return
  
  const marker = createDefaultMarker()
  // Default to map center
  marker.lat = props.form.mapCenterLat || 39.8283
  marker.lon = props.form.mapCenterLon || -98.5795
  props.form.mapMarkers.push(marker)
}

/**
 * Remove marker
 */
function removeMarker(index: number) {
  props.form.mapMarkers.splice(index, 1)
}

/**
 * Update a marker at specific index
 */
function updateMarker(index: number, updatedMarker: MapMarker) {
  props.form.mapMarkers[index] = updatedMarker
}

/**
 * Set marker coords to map center
 */
function useMapCenterForMarker(index: number) {
  const marker = props.form.mapMarkers[index]
  if (marker) {
    // Create new marker object to trigger reactivity properly
    props.form.mapMarkers[index] = {
      ...marker,
      lat: props.form.mapCenterLat,
      lon: props.form.mapCenterLon
    }
  }
}

/**
 * Get errors for a specific marker's items
 * Grug say: Errors are keyed by "marker_X_item_Y_field"
 */
function getMarkerItemErrors(markerIndex: number): Record<number, Record<string, string>> {
  const result: Record<number, Record<string, string>> = {}
  const prefix = `marker_${markerIndex}_item_`
  
  for (const [key, value] of Object.entries(props.errors)) {
    if (key.startsWith(prefix)) {
      // Extract item index and field name
      const rest = key.substring(prefix.length)
      const underscorePos = rest.indexOf('_')
      if (underscorePos > 0) {
        const itemIndex = parseInt(rest.substring(0, underscorePos))
        const field = rest.substring(underscorePos + 1)
        if (!isNaN(itemIndex)) {
          if (!result[itemIndex]) result[itemIndex] = {}
          result[itemIndex][field] = value
        }
      }
    }
  }
  
  return result
}
</script>

<style scoped>
.config-map {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.config-section {
  background: rgba(0, 0, 0, 0.1);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.section-icon {
  font-size: 18px;
  line-height: 1;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  flex: 1;
}

.section-count {
  font-size: 12px;
  color: var(--muted);
  background: rgba(0, 0, 0, 0.2);
  padding: 2px 8px;
  border-radius: 10px;
}

.form-group {
  margin-bottom: 16px;
}

.form-group:last-child {
  margin-bottom: 0;
}

.form-group > label {
  display: block;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.coord-inputs {
  display: grid;
  grid-template-columns: 1fr 1fr 80px;
  gap: 12px;
}

.coord-input-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.coord-label {
  font-size: 11px;
  color: var(--muted);
  font-weight: 500;
}

.form-input {
  width: 100%;
  padding: 8px 10px;
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text);
  font-family: var(--mono);
  font-size: 13px;
}

.form-input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.help-text {
  margin-top: 8px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.4;
}

/* Empty state */
.empty-markers {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 32px 16px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 6px;
  text-align: center;
  margin-bottom: 16px;
}

.empty-icon {
  font-size: 32px;
  opacity: 0.5;
}

.empty-text {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}

.empty-hint {
  font-size: 12px;
  color: var(--muted);
}

/* Markers list */
.markers-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 16px;
}

/* Limit warning */
.limit-warning {
  padding: 12px 16px;
  background: var(--color-warning-bg);
  border: 1px solid var(--color-warning-border);
  border-radius: 6px;
  color: var(--color-warning);
  font-size: 13px;
  font-weight: 500;
  text-align: center;
}

/* Add marker button */
.btn-add-marker {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px 16px;
  background: var(--color-primary);
  border: none;
  border-radius: 6px;
  color: white;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-add-marker:hover {
  background: var(--color-primary-hover);
  transform: translateY(-1px);
}

.btn-add-marker .btn-icon {
  font-size: 18px;
  font-weight: 700;
}

/* Tips section */
.tips-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  background: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-radius: 6px;
  font-size: 13px;
  color: var(--color-info);
}

.tip-icon {
  font-size: 14px;
  line-height: 1.4;
  flex-shrink: 0;
}

.tip-text {
  line-height: 1.4;
}

.tip-text a {
  color: var(--color-accent);
  text-decoration: underline;
}

.tip-text a:hover {
  text-decoration: none;
}

/* Responsive */
@media (max-width: 500px) {
  .coord-inputs {
    grid-template-columns: 1fr 1fr;
  }
  
  .coord-inputs .coord-input-group:last-child {
    grid-column: span 2;
  }
}
</style>
