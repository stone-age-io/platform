<template>
  <div class="config-status">
    
    <!-- Data Source Selection -->
    <div class="config-section">
      <div class="section-title">Data Source</div>
      <div class="form-group">
        <label>Source Type</label>
        <select v-model="form.dataSourceType" class="form-input">
          <option value="subscription">NATS Subject (Core/JetStream)</option>
          <option value="kv">Key-Value Store (KV)</option>
        </select>
      </div>

      <!-- Subscription Mode (Reusing ConfigDataSource) -->
      <ConfigDataSource 
        v-if="form.dataSourceType === 'subscription'"
        :form="form" 
        :errors="errors" 
      />

      <!-- KV Inputs (Only shown if KV mode) -->
      <template v-if="form.dataSourceType === 'kv'">
        <div class="form-group">
          <label>KV Bucket</label>
          <input 
            v-model="form.kvBucket" 
            type="text" 
            class="form-input"
            placeholder="my-bucket"
          />
        </div>
        <div class="form-group">
          <label>KV Key</label>
          <input 
            v-model="form.kvKey" 
            type="text" 
            class="form-input"
            placeholder="status.key"
          />
        </div>
        <div class="form-group">
          <label>JSONPath (optional)</label>
          <input 
            v-model="form.jsonPath" 
            type="text" 
            class="form-input"
            placeholder="$.status"
          />
        </div>
      </template>
    </div>

    <!-- Mappings Section -->
    <div class="config-section">
      <div class="section-title">State Mappings</div>
      <div class="help-text mb-2">
        Define how values map to colors. First match wins.
        <br><strong>Tip:</strong> Use <code>*</code> as value to match ANY message (e.g. for heartbeats).
      </div>
      
      <div class="mappings-list">
        <div v-for="(map, index) in form.statusMappings" :key="map.id" class="mapping-row">
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

    <!-- Default State -->
    <div class="config-section">
      <div class="section-title">Default State</div>
      <div class="form-row">
        <div class="input-group">
          <label>Color</label>
          <select 
            v-model="form.statusDefaultColor" 
            class="form-input"
            :style="{ color: form.statusDefaultColor }"
          >
            <option value="var(--color-info)" style="color: var(--color-info)">Info</option>
            <option value="var(--muted)" style="color: var(--muted)">Muted</option>
            <option value="var(--color-success)" style="color: var(--color-success)">Success</option>
            <option value="var(--color-warning)" style="color: var(--color-warning)">Warning</option>
            <option value="var(--color-error)" style="color: var(--color-error)">Error</option>
          </select>
        </div>
        <div class="input-group">
          <label>Label</label>
          <input 
            v-model="form.statusDefaultLabel" 
            type="text" 
            class="form-input"
            placeholder="Unknown"
          />
        </div>
      </div>
    </div>

    <!-- Staleness Logic -->
    <div class="config-section">
      <div class="section-title">Staleness (Watchdog)</div>
      <div class="form-group">
        <label class="checkbox-label">
          <input type="checkbox" v-model="form.statusShowStale" />
          <span>Enable Staleness Check</span>
        </label>
      </div>
      
      <template v-if="form.statusShowStale">
        <div class="form-group">
          <label>Threshold (ms)</label>
          <input 
            v-model.number="form.statusStalenessThreshold" 
            type="number" 
            class="form-input"
            min="1000"
            step="1000"
          />
          <div class="help-text">
            If no message received for {{ (form.statusStalenessThreshold / 1000).toFixed(1) }}s, show stale state.
          </div>
        </div>
        
        <div class="form-row">
          <div class="input-group">
            <label>Stale Color</label>
            <select 
              v-model="form.statusStaleColor" 
              class="form-input"
              :style="{ color: form.statusStaleColor }"
            >
              <option value="var(--muted)" style="color: var(--muted)">Muted (Grey)</option>
              <option value="var(--color-error)" style="color: var(--color-error)">Error (Red)</option>
              <option value="var(--color-warning)" style="color: var(--color-warning)">Warning (Yellow)</option>
            </select>
          </div>
          <div class="input-group">
            <label>Stale Label</label>
            <input 
              v-model="form.statusStaleLabel" 
              type="text" 
              class="form-input"
              placeholder="Stale"
            />
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import ConfigDataSource from './ConfigDataSource.vue'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

function addMapping() {
  props.form.statusMappings.push({
    id: Date.now().toString(),
    value: '',
    color: 'var(--color-success)',
    label: '',
    blink: false
  })
}

function removeMapping(index: number) {
  props.form.statusMappings.splice(index, 1)
}
</script>

<style scoped>
.config-status {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.config-section {
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
}

.section-title {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 12px;
  letter-spacing: 0.5px;
}

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

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
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
