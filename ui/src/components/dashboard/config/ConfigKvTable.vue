<template>
  <div class="config-kvtable">
    <div class="form-group">
      <label>KV Bucket</label>
      <input
        v-model="form.kvTableBucket"
        type="text"
        class="form-input"
        :class="{ 'has-error': errors.kvTableBucket }"
        placeholder="occupancy"
      />
      <div v-if="errors.kvTableBucket" class="error-text">{{ errors.kvTableBucket }}</div>
      <div v-else class="help-text">
        Supports variables like <code v-pre>{{bucket_name}}</code>
      </div>
    </div>

    <div class="form-group">
      <label>Key Pattern</label>
      <input
        v-model="form.kvTableKeyPattern"
        type="text"
        class="form-input font-mono"
        :class="{ 'has-error': errors.kvTableKeyPattern }"
        placeholder="myprefix.>"
      />
      <div v-if="errors.kvTableKeyPattern" class="error-text">{{ errors.kvTableKeyPattern }}</div>
      <div v-else class="help-text">
        NATS wildcard pattern. Use <code>></code> for all keys or <code v-pre>{{location}}.></code> for a prefix.
      </div>
    </div>

    <!-- Column Builder -->
    <div class="form-group">
      <label>Columns</label>
      <div v-if="errors.kvTableColumns" class="error-text mb-2">{{ errors.kvTableColumns }}</div>

      <div class="columns-list">
        <div v-for="(col, i) in form.kvTableColumns" :key="col.id" class="column-item">
          <div class="column-fields">
            <div class="field-row">
              <input
                v-model="col.label"
                type="text"
                class="form-input"
                placeholder="Column label"
              />
              <select v-model="col.format" class="form-input format-select">
                <option value="text">Text</option>
                <option value="number">Number</option>
                <option value="relative-time">Relative Time</option>
                <option value="datetime">Date/Time</option>
              </select>
            </div>
            <div class="field-row">
              <input
                v-model="col.path"
                type="text"
                class="form-input font-mono"
                :placeholder="pathPlaceholder"
                list="meta-paths"
              />
              <input
                v-if="col.format === 'datetime'"
                v-model="col.formatOptions"
                type="text"
                class="form-input font-mono"
                placeholder="PPpp"
                title="date-fns format string"
              />
              <button
                class="btn-remove"
                title="Remove column"
                @click="removeColumn(i)"
              >
                ✕
              </button>
            </div>
          </div>
        </div>
      </div>

      <datalist id="meta-paths">
        <option value="__key_suffix__">Key suffix (after prefix)</option>
        <option value="__key__">Full key</option>
        <option value="__revision__">Revision number</option>
        <option value="__timestamp__">Entry timestamp</option>
      </datalist>

      <button class="btn-add" @click="addColumn">+ Add Column</button>
    </div>

    <!-- Default Sort -->
    <div class="form-group">
      <label>Default Sort</label>
      <div class="sort-row">
        <select v-model="form.kvTableDefaultSortColumn" class="form-input">
          <option value="">None</option>
          <option v-for="col in form.kvTableColumns" :key="col.id" :value="col.id">
            {{ col.label || col.path }}
          </option>
        </select>
        <select v-model="form.kvTableDefaultSortDirection" class="form-input direction-select">
          <option value="asc">Ascending</option>
          <option value="desc">Descending</option>
        </select>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import type { KvTableColumnFormat } from '@/types/dashboard'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const pathPlaceholder = '$.field or __key_suffix__'

function addColumn() {
  const id = `col_${Date.now()}_${Math.random().toString(36).substr(2, 5)}`
  props.form.kvTableColumns.push({
    id,
    label: '',
    path: '',
    format: 'text' as KvTableColumnFormat
  })
}

function removeColumn(index: number) {
  props.form.kvTableColumns.splice(index, 1)
}
</script>

<style scoped>
.font-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.columns-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}

.column-item {
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  padding: 8px;
  background: oklch(var(--b2) / 0.3);
}

.column-fields {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.field-row .form-input {
  flex: 1;
}

.format-select {
  max-width: 140px;
}

.direction-select {
  max-width: 120px;
}

.sort-row {
  display: flex;
  gap: 8px;
}

.sort-row .form-input {
  flex: 1;
}

.btn-remove {
  background: none;
  border: 1px solid oklch(var(--er) / 0.3);
  color: oklch(var(--er));
  border-radius: 4px;
  cursor: pointer;
  padding: 4px 8px;
  font-size: 14px;
  flex-shrink: 0;
  transition: all 0.2s;
}

.btn-remove:hover {
  background: oklch(var(--er) / 0.1);
  border-color: oklch(var(--er));
}

.btn-add {
  width: 100%;
  padding: 8px;
  border: 2px dashed oklch(var(--b3));
  border-radius: 6px;
  background: none;
  color: oklch(var(--bc) / 0.6);
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
}

.btn-add:hover {
  border-color: oklch(var(--a));
  color: oklch(var(--a));
  background: oklch(var(--a) / 0.05);
}

.mb-2 { margin-bottom: 8px; }
</style>
