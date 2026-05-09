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

    <div class="form-group">
      <label>Columns</label>
      <ColumnsBuilder
        v-model="form.kvTableColumns"
        :meta-paths="kvMetaPaths"
        :error-text="errors.kvTableColumns"
        path-placeholder="$.field or __key_suffix__"
      />
    </div>

    <div class="form-group">
      <label>Max Rows</label>
      <input
        v-model.number="form.kvTableMaxRows"
        type="number"
        min="0"
        class="form-input"
        placeholder="500"
      />
      <div class="help-text">
        Hard cap on rendered rows. Use 0 for unlimited (not recommended for large buckets).
      </div>
    </div>

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
import ColumnsBuilder from './ColumnsBuilder.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const kvMetaPaths = [
  { value: '__key_suffix__', label: 'Key suffix (after prefix)' },
  { value: '__key__',        label: 'Full key' },
  { value: '__revision__',   label: 'Revision number' },
  { value: '__timestamp__',  label: 'Entry timestamp' },
]
</script>

<style scoped>
.font-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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
</style>
