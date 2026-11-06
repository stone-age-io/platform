<template>
  <div class="config-status">
    
    <div class="config-section">
      <div class="section-title">Data Source</div>
      <div class="form-group">
        <label>Source Type</label>
        <select v-model="form.dataSourceType" class="form-input">
          <option value="subscription">NATS Subject (Core/JetStream)</option>
          <option value="kv">Key-Value Store (KV)</option>
        </select>
      </div>

      <ConfigDataSource 
        v-if="form.dataSourceType === 'subscription'"
        :form="form" 
        :errors="errors" 
      />

      <ConfigKvSource
        v-if="form.dataSourceType === 'kv'"
        :form="form"
        :errors="errors"
        key-placeholder="status.key"
      />
    </div>

    <div class="config-section">
      <div class="section-title">State Mappings</div>
      <StatusMappingEditor :mappings="form.statusMappings" />
    </div>

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
import ConfigKvSource from './ConfigKvSource.vue'
import StatusMappingEditor from './StatusMappingEditor.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()


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
