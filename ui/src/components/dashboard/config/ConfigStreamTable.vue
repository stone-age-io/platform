<template>
  <div class="config-streamtable">
    <div class="form-group">
      <label>Columns</label>
      <ColumnsBuilder
        v-model="form.streamTableColumns"
        :meta-paths="streamMetaPaths"
        :error-text="errors.streamTableColumns"
        path-placeholder="$.field or __subject__"
      />
    </div>

    <div class="form-group">
      <label>Default Sort</label>
      <div class="sort-row">
        <select v-model="form.streamTableDefaultSortColumn" class="form-input">
          <option value="">None (newest first)</option>
          <option v-for="col in form.streamTableColumns" :key="col.id" :value="col.id">
            {{ col.label || col.path }}
          </option>
        </select>
        <select v-model="form.streamTableDefaultSortDirection" class="form-input direction-select">
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

const streamMetaPaths = [
  { value: '__subject__',   label: 'Full subject' },
  { value: '__subject.0__', label: 'Subject token 0' },
  { value: '__subject.1__', label: 'Subject token 1' },
  { value: '__subject.2__', label: 'Subject token 2' },
  { value: '__subject.3__', label: 'Subject token 3' },
  { value: '__timestamp__', label: 'Receive time' },
]
</script>

<style scoped>
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

.help-text {
  font-size: 11px;
  color: oklch(var(--bc) / 0.5);
  margin-top: 4px;
}
</style>
