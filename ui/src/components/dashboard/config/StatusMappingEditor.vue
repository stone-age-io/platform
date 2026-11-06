<template>
  <div class="mapping-editor">
    <div class="help-text mb-2">
      Define how values map to colors. First match wins.
      <br><strong>Tip:</strong> Use <code>*</code> as value to match ANY message (e.g. for heartbeats).
    </div>

    <div class="mappings-list">
      <div v-for="(map, index) in mappings" :key="map.id" class="mapping-row">
        <div class="row-header">
          <span class="row-index">#{{ index + 1 }}</span>
          <button class="btn-icon danger" @click="removeMapping(index)">✕</button>
        </div>

        <div class="row-inputs">
          <div class="input-group">
            <label>If Value ==</label>
            <input
              v-model="map.value"
              type="text"
              class="form-input"
              placeholder="on (or *)"
            />
          </div>

          <div class="input-group">
            <label>Color</label>
            <select
              v-model="map.color"
              class="form-input color-select"
              :style="{ color: map.color }"
            >
              <option value="var(--color-success)" style="color: var(--color-success)">Success (Green)</option>
              <option value="var(--color-warning)" style="color: var(--color-warning)">Warning (Yellow)</option>
              <option value="var(--color-error)" style="color: var(--color-error)">Error (Red)</option>
              <option value="var(--color-info)" style="color: var(--color-info)">Info (Blue)</option>
              <option value="var(--color-primary)" style="color: var(--color-primary)">Primary</option>
              <option value="var(--color-secondary)" style="color: var(--color-secondary)">Secondary</option>
              <option value="var(--muted)" style="color: var(--muted)">Muted (Grey)</option>
            </select>
          </div>

          <div class="input-group">
            <label>Label (Optional)</label>
            <input
              v-model="map.label"
              type="text"
              class="form-input"
              placeholder="Running"
            />
          </div>

          <div class="input-group checkbox">
            <label class="checkbox-label">
              <input type="checkbox" v-model="map.blink" />
              <span>Pulse</span>
            </label>
          </div>
        </div>
      </div>
    </div>

    <button class="btn-add" @click="addMapping">
      + Add Mapping
    </button>
  </div>
</template>

<script setup lang="ts">
import type { StatusMapping } from '@/types/dashboard'

/**
 * The value-to-colour rows, shared by ConfigStatus and the floor plan bindings
 * so the two editors cannot drift: a dashboard saved in either renders in both.
 *
 * The array is mutated in place, not emitted: both callers already hold it by
 * reference inside the object they are editing.
 */
const props = defineProps<{
  mappings: StatusMapping[]
}>()

function addMapping() {
  props.mappings.push({
    id: Date.now().toString(),
    value: '',
    color: 'var(--color-success)',
    label: '',
    blink: false,
  })
}

function removeMapping(index: number) {
  props.mappings.splice(index, 1)
}
</script>

<style scoped>
.mappings-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.mapping-row {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px;
}

.row-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.row-index {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
}

.row-inputs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.input-group label {
  font-size: 11px;
  font-weight: 500;
  color: var(--muted);
}

.input-group.checkbox {
  grid-column: span 2;
  flex-direction: row;
  align-items: center;
  margin-top: 4px;
}

.btn-icon {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 4px;
}

.btn-icon.danger:hover {
  color: var(--color-error);
}

.btn-add {
  width: 100%;
  padding: 8px;
  background: transparent;
  border: 1px dashed var(--border);
  color: var(--color-primary);
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
}

.btn-add:hover {
  background: var(--color-primary-hover);
  color: white;
  border-style: solid;
}

.mb-2 { margin-bottom: 8px; }

.color-select {
  font-weight: 600;
}
</style>
