<template>
  <div class="zone-editor">
    <div class="zone-list">
      <div v-if="modelValue.length === 0" class="empty-zones">
        No color zones defined. Gauge will use default colors.
      </div>
      
      <div v-for="(zone, index) in modelValue" :key="index" class="zone-row">
        <span class="row-label">Range:</span>
        
        <!-- Min value -->
        <input 
          v-model.number="zone.min" 
          type="number" 
          class="input-number" 
          placeholder="Min"
          step="any"
        />
        
        <span class="row-separator">to</span>
        
        <!-- Max value -->
        <input 
          v-model.number="zone.max" 
          type="number" 
          class="input-number" 
          placeholder="Max"
          step="any"
        />
        
        <span class="row-label">Color:</span>
        
        <!-- Color picker -->
        <select 
          v-model="zone.color" 
          class="input-select color" 
          :style="{ color: zone.color }"
        >
          <option value="var(--color-primary)" style="color: var(--color-primary)">Primary</option>
          <option value="var(--color-secondary)" style="color: var(--color-secondary)">Secondary</option>
          <option value="var(--color-accent)" style="color: var(--color-accent)">Accent</option>
          <option value="var(--color-success)" style="color: var(--color-success)">Success</option>
          <option value="var(--color-warning)" style="color: var(--color-warning)">Warning</option>
          <option value="var(--color-error)" style="color: var(--color-error)">Error</option>
          <option value="var(--color-info)" style="color: var(--color-info)">Info</option>
          <option value="var(--text)" style="color: var(--text)">Default Text</option>
          <option value="var(--muted)" style="color: var(--muted)">Muted</option>
        </select>
        
        <!-- Delete -->
        <button class="btn-icon danger" @click="removeZone(index)" title="Remove Zone">
          ✕
        </button>
      </div>
    </div>
    
    <button class="btn-add" @click="addZone">
      + Add Zone
    </button>
  </div>
</template>

<script setup lang="ts">
interface GaugeZone {
  min: number
  max: number
  color: string
}

const props = defineProps<{
  modelValue: GaugeZone[]
}>()

const emit = defineEmits<{
  'update:modelValue': [zones: GaugeZone[]]
}>()

function addZone() {
  const newZone: GaugeZone = {
    min: 0,
    max: 100,
    color: 'var(--color-success)'
  }
  emit('update:modelValue', [...props.modelValue, newZone])
}

function removeZone(index: number) {
  const newZones = [...props.modelValue]
  newZones.splice(index, 1)
  emit('update:modelValue', newZones)
}
</script>

<style scoped>
.zone-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(0, 0, 0, 0.1);
  padding: 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
}

.empty-zones {
  color: var(--muted);
  font-size: 13px;
  font-style: italic;
  text-align: center;
  padding: 8px;
}

.zone-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.zone-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.row-label {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
}

.row-separator {
  font-size: 12px;
  color: var(--muted);
}

.input-number,
.input-select {
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 6px 8px;
  border-radius: 4px;
  font-family: var(--mono);
  font-size: 13px;
}

.input-number:focus,
.input-select:focus {
  outline: none;
  border-color: var(--color-accent);
}

.input-number {
  width: 80px;
}

.input-select.color {
  flex: 1;
  min-width: 140px;
  font-weight: 600;
}

.btn-icon {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
}

.btn-icon:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
}

.btn-icon.danger:hover {
  color: var(--color-error);
  background: var(--color-error-bg);
}

.btn-add {
  align-self: flex-start;
  background: transparent;
  border: 1px dashed var(--border);
  color: var(--color-accent);
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
}

.btn-add:hover {
  background: var(--color-info-bg);
  border-color: var(--color-accent);
}

/* Responsive */
@media (max-width: 600px) {
  .zone-row {
    flex-direction: column;
    align-items: stretch;
  }
  
  .input-number {
    width: 100%;
  }
  
  .row-label,
  .row-separator {
    align-self: flex-start;
  }
}
</style>
