<template>
  <div class="marker-editor" :class="{ 'is-expanded': isExpanded }">
    <!-- Marker Header (collapsible) -->
    <div class="marker-header" @click="isExpanded = !isExpanded">
      <span class="marker-icon">📍</span>
      <span class="marker-label">{{ marker.label || 'Unnamed Marker' }}</span>
      <span class="marker-coords">
        <template v-if="isDynamic">Dynamic</template>
        <template v-else>{{ marker.lat.toFixed(4) }}, {{ marker.lon.toFixed(4) }}</template>
      </span>
      <span class="item-count">
        {{ marker.items.length }} item{{ marker.items.length !== 1 ? 's' : '' }}
      </span>
      <span class="expand-icon">{{ isExpanded ? '▼' : '▶' }}</span>
      <button 
        class="btn-icon-small danger" 
        @click.stop="$emit('remove')"
        title="Remove marker"
      >
        ✕
      </button>
    </div>
    
    <!-- Marker Body -->
    <div v-if="isExpanded" class="marker-body">
      
      <!-- POSITION SECTION -->
      <div class="marker-section">
        <div class="section-title">Position</div>
        
        <div class="form-row">
          <label>Label</label>
          <input 
            :value="marker.label"
            @input="updateMarkerField('label', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            placeholder="Truck 1"
          />
        </div>

        <!-- Mode Toggle -->
        <div class="form-row">
          <label>Mode</label>
          <div class="mode-toggle">
            <button 
              class="mode-btn" 
              :class="{ active: !isDynamic }"
              @click="setMode('static')"
            >
              Static
            </button>
            <button 
              class="mode-btn" 
              :class="{ active: isDynamic }"
              @click="setMode('dynamic')"
            >
              Dynamic (Live)
            </button>
          </div>
        </div>
        
        <!-- STATIC MODE INPUTS -->
        <div v-if="!isDynamic" class="coord-row">
          <div class="form-row">
            <label>Latitude</label>
            <input 
              :value="marker.lat"
              @input="updateMarkerField('lat', parseFloat(($event.target as HTMLInputElement).value) || 0)"
              type="number" 
              class="form-input"
              step="any"
              placeholder="39.8283"
            />
          </div>
          <div class="form-row">
            <label>Longitude</label>
            <input 
              :value="marker.lon"
              @input="updateMarkerField('lon', parseFloat(($event.target as HTMLInputElement).value) || 0)"
              type="number" 
              class="form-input"
              step="any"
              placeholder="-98.5795"
            />
          </div>
        </div>
        
        <button 
          v-if="!isDynamic"
          type="button"
          class="btn-use-center"
          @click="$emit('use-center')"
        >
          📍 Use Map Center
        </button>

        <!-- DYNAMIC MODE INPUTS -->
        <template v-if="isDynamic">
          <div class="form-row">
            <label>Subject</label>
            <input 
              :value="posConfig.subject"
              @input="updatePosConfig('subject', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder="fleet.truck1.gps"
            />
          </div>

          <div class="coord-row">
            <div class="form-row">
              <label>Lat JSONPath</label>
              <input 
                :value="posConfig.latJsonPath"
                @input="updatePosConfig('latJsonPath', ($event.target as HTMLInputElement).value)"
                type="text" 
                class="form-input"
                placeholder="$.lat"
              />
            </div>
            <div class="form-row">
              <label>Lon JSONPath</label>
              <input 
                :value="posConfig.lonJsonPath"
                @input="updatePosConfig('lonJsonPath', ($event.target as HTMLInputElement).value)"
                type="text" 
                class="form-input"
                placeholder="$.lon"
              />
            </div>
          </div>

          <!-- JetStream Options -->
          <div class="form-row checkbox-row">
            <label class="checkbox-label">
              <input 
                type="checkbox" 
                :checked="posConfig.useJetStream"
                @change="updatePosConfig('useJetStream', ($event.target as HTMLInputElement).checked)"
              />
              <span>Use JetStream (History)</span>
            </label>
          </div>

          <div v-if="posConfig.useJetStream" class="form-row">
            <label>Deliver Policy</label>
            <select 
              :value="posConfig.deliverPolicy || 'last'"
              @change="updatePosConfig('deliverPolicy', ($event.target as HTMLSelectElement).value)"
              class="form-input"
            >
              <option value="last">Last (Current Position)</option>
              <option value="new">New (Live Only)</option>
            </select>
          </div>
        </template>
      </div>
      
      <!-- ITEMS SECTION -->
      <div class="marker-section">
        <div class="section-title">
          Popup Items
          <span class="section-hint">({{ marker.items.length }}/{{ MAX_ITEMS_PER_MARKER }})</span>
        </div>
        
        <div v-if="marker.items.length === 0" class="empty-items">
          No items configured. Add an item to display data or controls.
        </div>
        
        <div class="items-list">
          <MarkerItemEditor
            v-for="(item, index) in marker.items"
            :key="item.id"
            :item="item"
            :errors="itemErrors ? itemErrors[index] : undefined"
            @remove="removeItem(index)"
            @update:item="updateItem(index, $event)"
          />
        </div>
        
        <!-- Limit warning -->
        <div v-if="isAtLimit" class="limit-warning">
          ⚠️ Maximum {{ MAX_ITEMS_PER_MARKER }} items per marker
        </div>
        
        <div v-else class="add-item-row">
          <button class="btn-add" @click="addItem('publish')">
            + Add Item
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import MarkerItemEditor from './MarkerItemEditor.vue'
import { createDefaultItem, MAP_LIMITS } from '@/types/dashboard'
import type { MapMarker, MapMarkerItem, MapItemType, MarkerPositionConfig } from '@/types/dashboard'

const MAX_ITEMS_PER_MARKER = MAP_LIMITS.MAX_ITEMS_PER_MARKER

const props = defineProps<{
  marker: MapMarker
  itemErrors?: Record<number, Record<string, string>>
}>()

const emit = defineEmits<{
  remove: []
  'use-center': []
  'update:marker': [marker: MapMarker]
}>()

const isExpanded = ref(true)

const isAtLimit = computed(() => props.marker.items.length >= MAX_ITEMS_PER_MARKER)

const isDynamic = computed(() => props.marker.positionConfig?.mode === 'dynamic')

const posConfig = computed<MarkerPositionConfig>(() => {
  return props.marker.positionConfig || {
    mode: 'static',
    subject: '',
    latJsonPath: '$.lat',
    lonJsonPath: '$.lon',
    useJetStream: false,
    deliverPolicy: 'last'
  }
})

function emitMarkerUpdate(updates: Partial<MapMarker>) {
  emit('update:marker', { ...props.marker, ...updates })
}

function updateMarkerField(field: keyof MapMarker, value: any) {
  emitMarkerUpdate({ [field]: value })
}

function setMode(mode: 'static' | 'dynamic') {
  const newConfig = { ...posConfig.value, mode }
  emitMarkerUpdate({ positionConfig: newConfig })
}

function updatePosConfig(field: keyof MarkerPositionConfig, value: any) {
  const newConfig = { ...posConfig.value, [field]: value }
  emitMarkerUpdate({ positionConfig: newConfig })
}

function addItem(type: MapItemType) {
  if (isAtLimit.value) return
  const item = createDefaultItem(type)
  const newItems = [...props.marker.items, item]
  emitMarkerUpdate({ items: newItems })
}

function removeItem(index: number) {
  const newItems = [...props.marker.items]
  newItems.splice(index, 1)
  emitMarkerUpdate({ items: newItems })
}

function updateItem(index: number, updatedItem: MapMarkerItem) {
  const newItems = [...props.marker.items]
  newItems[index] = updatedItem
  emitMarkerUpdate({ items: newItems })
}
</script>

<style scoped>
.marker-editor {
  background: rgba(0, 0, 0, 0.1);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
  transition: all 0.2s;
}

.marker-editor.is-expanded {
  border-color: var(--color-accent);
}

.marker-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  background: rgba(0, 0, 0, 0.2);
  cursor: pointer;
  user-select: none;
  transition: background 0.2s;
}

.marker-header:hover {
  background: rgba(0, 0, 0, 0.3);
}

.marker-icon {
  font-size: 18px;
  line-height: 1;
}

.marker-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.marker-coords {
  font-size: 11px;
  font-family: var(--mono);
  color: var(--muted);
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
}

.item-count {
  font-size: 11px;
  color: var(--color-accent);
  background: var(--color-info-bg);
  padding: 2px 8px;
  border-radius: 10px;
}

.expand-icon {
  font-size: 10px;
  color: var(--muted);
  transition: transform 0.2s;
}

.btn-icon-small {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 4px;
  font-size: 14px;
  transition: all 0.2s;
}

.btn-icon-small.danger:hover {
  background: var(--color-error-bg);
  color: var(--color-error);
}

.marker-body {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  border-top: 1px solid var(--border);
}

.marker-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-hint {
  font-weight: 400;
  text-transform: none;
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-row label {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
}

.coord-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
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

/* Mode Toggle */
.mode-toggle {
  display: flex;
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 2px;
}

.mode-btn {
  flex: 1;
  padding: 6px;
  font-size: 12px;
  font-weight: 500;
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  border-radius: 3px;
  transition: all 0.2s;
}

.mode-btn.active {
  background: var(--color-accent);
  color: white;
}

.checkbox-row {
  flex-direction: row;
  align-items: center;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text);
}

.checkbox-label input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--color-primary);
}

.btn-use-center {
  align-self: flex-start;
  padding: 6px 12px;
  background: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-radius: 4px;
  color: var(--color-info);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-use-center:hover {
  background: var(--color-accent);
  border-color: var(--color-accent);
  color: white;
}

.empty-items {
  color: var(--muted);
  font-size: 13px;
  font-style: italic;
  text-align: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 6px;
}

.items-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.limit-warning {
  padding: 8px 12px;
  background: var(--color-warning-bg);
  border: 1px solid var(--color-warning-border);
  border-radius: 4px;
  color: var(--color-warning);
  font-size: 12px;
  font-weight: 500;
  text-align: center;
}

.add-item-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.btn-add {
  flex: 1;
  min-width: 140px;
  padding: 8px 12px;
  background: transparent;
  border: 1px dashed var(--border);
  border-radius: 4px;
  color: var(--color-info);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-add:hover {
  background: var(--color-info-bg);
  border-color: var(--color-info);
}

@media (max-width: 500px) {
  .coord-row {
    grid-template-columns: 1fr;
  }
  
  .marker-coords {
    display: none;
  }
  
  .add-item-row {
    flex-direction: column;
  }
  
  .btn-add {
    width: 100%;
  }
}
</style>
