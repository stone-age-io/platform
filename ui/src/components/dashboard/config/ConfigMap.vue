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

    <!-- Map Options -->
    <div class="config-section">
      <div class="section-header">
        <span class="section-icon">⚙️</span>
        <span class="section-title">Map Options</span>
      </div>

      <label class="toggle-row">
        <input v-model="form.mapEnableClustering" type="checkbox" class="toggle-input" />
        <span class="toggle-label">Enable marker clustering</span>
      </label>
      <div class="help-text" style="margin-bottom: 12px;">
        Groups nearby markers into clusters when zoomed out.
      </div>

      <label class="toggle-row">
        <input v-model="form.mapFitBoundsOnLoad" type="checkbox" class="toggle-input" />
        <span class="toggle-label">Fit bounds on load</span>
      </label>
      <div class="help-text">
        Auto-zoom to show all markers when the map loads.
      </div>
    </div>

    <!-- Dynamic Markers Section -->
    <div class="config-section">
      <div class="section-header">
        <span class="section-icon">🔄</span>
        <span class="section-title">Dynamic Markers</span>
      </div>

      <label class="toggle-row" style="margin-bottom: 12px;">
        <input v-model="form.mapDynamicEnabled" type="checkbox" class="toggle-input" />
        <span class="toggle-label">Enable KV-driven markers</span>
      </label>

      <template v-if="form.mapDynamicEnabled">
        <div class="form-group">
          <label>KV Bucket</label>
          <input
            v-model="form.mapDynamicBucket"
            type="text"
            class="form-input"
            placeholder="fleet-positions"
          />
          <div class="help-text">
            NATS KV bucket name. Supports <code>{<!-- -->{variable}}</code> syntax.
          </div>
        </div>

        <div class="form-group">
          <label>Key Pattern</label>
          <input
            v-model="form.mapDynamicKeyPattern"
            type="text"
            class="form-input"
            placeholder=">"
          />
          <div class="help-text">
            Wildcard pattern: <code>></code> (all keys) or <code>prefix.></code> (scoped).
            Supports <code>{<!-- -->{variable}}</code> syntax.
          </div>
        </div>

        <div class="form-group">
          <label>Position Paths</label>
          <div class="two-col">
            <div>
              <label class="coord-label">Latitude JSONPath</label>
              <input
                v-model="form.mapDynamicLatPath"
                type="text"
                class="form-input"
                placeholder="$.lat"
              />
            </div>
            <div>
              <label class="coord-label">Longitude JSONPath</label>
              <input
                v-model="form.mapDynamicLonPath"
                type="text"
                class="form-input"
                placeholder="$.lon"
              />
            </div>
          </div>
          <div class="help-text">
            JSONPath to extract lat/lon from each KV entry's JSON data.
          </div>
        </div>

        <div class="form-group">
          <label>Label Path</label>
          <input
            v-model="form.mapDynamicLabelPath"
            type="text"
            class="form-input"
            placeholder="__key_suffix__"
            list="label-path-suggestions"
          />
          <datalist id="label-path-suggestions">
            <option value="__key_suffix__" />
            <option value="__key__" />
            <option value="$.name" />
            <option value="$.label" />
          </datalist>
          <div class="help-text">
            JSONPath or meta-path for marker label.
            <code>__key_suffix__</code> uses the key suffix, <code>__key__</code> uses the full key.
          </div>
        </div>

        <!-- Popup Fields -->
        <div class="form-group">
          <label>Popup Fields</label>

          <div v-if="form.mapDynamicPopupFields.length === 0" class="empty-fields">
            No popup fields — click a dynamic marker to see KV key info only.
          </div>

          <div v-else class="popup-fields-list">
            <div
              v-for="(field, index) in form.mapDynamicPopupFields"
              :key="index"
              class="popup-field-row"
            >
              <input
                v-model="field.label"
                type="text"
                class="form-input field-label-input"
                placeholder="Label"
              />
              <input
                v-model="field.path"
                type="text"
                class="form-input field-path-input"
                placeholder="$.path"
              />
              <select v-model="field.format" class="form-input field-format-select">
                <option value="text">Text</option>
                <option value="number">Number</option>
                <option value="relative-time">Relative Time</option>
                <option value="datetime">Datetime</option>
              </select>
              <button class="btn-remove-field" @click="removePopupField(index)" title="Remove field">
                ✕
              </button>
            </div>
          </div>

          <button class="btn-add-field" @click="addPopupField">
            + Add Popup Field
          </button>
        </div>
      </template>
    </div>

    <!-- Static Markers Section -->
    <div class="config-section">
      <div class="section-header">
        <span class="section-icon">📍</span>
        <span class="section-title">Static Markers</span>
        <span class="section-count">{{ form.mapMarkers.length }}/{{ MAX_MARKERS }}</span>
      </div>

      <div v-if="form.mapMarkers.length === 0" class="empty-markers">
        <div class="empty-icon">📍</div>
        <div class="empty-text">No static markers configured</div>
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
          <strong>Dynamic markers</strong> auto-generate from a NATS KV bucket.
          Each key becomes a marker with lat/lon extracted via JSONPath.
        </span>
      </div>
      <div class="tip">
        <span class="tip-icon">📊</span>
        <span class="tip-text">
          <strong>Limits:</strong> {{ MAX_MARKERS }} static markers per map,
          {{ MAX_ITEMS_PER_MARKER }} items per marker, {{ MAX_DYNAMIC_MARKERS }} dynamic markers.
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkerEditor from './MarkerEditor.vue'
import { createDefaultMarker, MAP_LIMITS } from '@/types/dashboard'
import type { MapMarker, DynamicMarkerPopupField } from '@/types/dashboard'
import type { WidgetFormState } from '@/types/config'

const MAX_MARKERS = MAP_LIMITS.MAX_MARKERS
const MAX_ITEMS_PER_MARKER = MAP_LIMITS.MAX_ITEMS_PER_MARKER
const MAX_DYNAMIC_MARKERS = MAP_LIMITS.MAX_DYNAMIC_MARKERS

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const isAtMarkerLimit = computed(() => props.form.mapMarkers.length >= MAX_MARKERS)

function addMarker() {
  if (isAtMarkerLimit.value) return
  const marker = createDefaultMarker()
  marker.lat = props.form.mapCenterLat || 39.8283
  marker.lon = props.form.mapCenterLon || -98.5795
  props.form.mapMarkers.push(marker)
}

function removeMarker(index: number) {
  props.form.mapMarkers.splice(index, 1)
}

function updateMarker(index: number, updatedMarker: MapMarker) {
  props.form.mapMarkers[index] = updatedMarker
}

function useMapCenterForMarker(index: number) {
  const marker = props.form.mapMarkers[index]
  if (marker) {
    props.form.mapMarkers[index] = {
      ...marker,
      lat: props.form.mapCenterLat,
      lon: props.form.mapCenterLon
    }
  }
}

function addPopupField() {
  const field: DynamicMarkerPopupField = {
    label: '',
    path: '',
    format: 'text'
  }
  props.form.mapDynamicPopupFields.push(field)
}

function removePopupField(index: number) {
  props.form.mapDynamicPopupFields.splice(index, 1)
}

function getMarkerItemErrors(markerIndex: number): Record<number, Record<string, string>> {
  const result: Record<number, Record<string, string>> = {}
  const prefix = `marker_${markerIndex}_item_`

  for (const [key, value] of Object.entries(props.errors)) {
    if (key.startsWith(prefix)) {
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

.help-text code {
  background: rgba(0, 0, 0, 0.2);
  padding: 1px 4px;
  border-radius: 3px;
  font-family: var(--mono);
  font-size: 11px;
}

.two-col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

/* Toggle rows */
.toggle-row {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 0;
}

.toggle-input {
  width: 16px;
  height: 16px;
  accent-color: var(--color-accent);
  cursor: pointer;
}

.toggle-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

/* Popup fields */
.empty-fields {
  padding: 12px;
  font-size: 12px;
  color: var(--muted);
  text-align: center;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 6px;
  margin-bottom: 8px;
}

.popup-fields-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}

.popup-field-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.field-label-input {
  flex: 1;
  min-width: 0;
}

.field-path-input {
  flex: 1.5;
  min-width: 0;
}

.field-format-select {
  flex: 0 0 auto;
  width: 120px;
}

.btn-remove-field {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  background: rgba(255, 0, 0, 0.1);
  border: 1px solid rgba(255, 0, 0, 0.2);
  border-radius: 4px;
  color: var(--color-error);
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.btn-remove-field:hover {
  background: rgba(255, 0, 0, 0.2);
}

.btn-add-field {
  width: 100%;
  padding: 8px;
  background: transparent;
  border: 1px dashed var(--border);
  border-radius: 6px;
  color: var(--muted);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-add-field:hover {
  border-color: var(--color-accent);
  color: var(--color-accent);
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

  .two-col {
    grid-template-columns: 1fr;
  }

  .popup-field-row {
    flex-wrap: wrap;
  }

  .field-format-select {
    width: 100%;
  }
}
</style>
