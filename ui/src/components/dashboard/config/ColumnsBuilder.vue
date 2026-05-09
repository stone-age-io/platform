<!-- Shared column-builder used by KvTable and StreamTable widget config forms. -->
<template>
  <div class="columns-builder">
    <div v-if="errorText" class="error-text mb-2">{{ errorText }}</div>

    <div class="columns-list">
      <div v-for="(col, i) in columns" :key="col.id" class="column-item">
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
              :list="datalistId"
            />
            <input
              v-if="col.format === 'datetime'"
              v-model="col.formatOptions"
              type="text"
              class="form-input font-mono"
              placeholder="PPpp"
              title="date-fns format string"
            />
            <button class="btn-remove" title="Remove column" @click="removeColumn(i)">✕</button>
          </div>

          <details class="rules-details" :open="(col.rules?.length ?? 0) > 0">
            <summary>
              Conditional formatting
              <span v-if="col.rules?.length" class="rule-count">{{ col.rules.length }}</span>
            </summary>
            <div class="rules-list">
              <div v-for="(rule, ri) in col.rules || []" :key="ri" class="rule-row">
                <select v-model="rule.op" class="form-input rule-op">
                  <option value="eq">=</option>
                  <option value="gt">&gt;</option>
                  <option value="gte">&gt;=</option>
                  <option value="lt">&lt;</option>
                  <option value="lte">&lt;=</option>
                  <option value="contains">contains</option>
                </select>
                <input
                  v-model="rule.value"
                  type="text"
                  class="form-input rule-value"
                  placeholder="value"
                />
                <select v-model="rule.style" class="form-input rule-style" :class="`style-${rule.style}`">
                  <option value="success">success</option>
                  <option value="info">info</option>
                  <option value="warning">warning</option>
                  <option value="error">error</option>
                </select>
                <button class="btn-remove rule-remove" title="Remove rule" @click="removeRule(col, ri)">✕</button>
              </div>
              <button class="btn-add-rule" @click="addRule(col)">+ Add rule</button>
              <div class="help-text">First match wins. Numeric ops parse both sides as numbers.</div>
            </div>
          </details>
        </div>
      </div>
    </div>

    <datalist :id="datalistId">
      <option v-for="mp in metaPaths" :key="mp.value" :value="mp.value">{{ mp.label }}</option>
    </datalist>

    <button class="btn-add" @click="addColumn">+ Add Column</button>
  </div>
</template>

<script setup lang="ts">
import { useId } from 'vue'
import type { TableColumn, TableColumnFormat, ConditionalRule } from '@/types/dashboard'

const columns = defineModel<TableColumn[]>({ required: true })

withDefaults(defineProps<{
  metaPaths: Array<{ value: string; label: string }>
  errorText?: string
  pathPlaceholder?: string
}>(), {
  pathPlaceholder: '$.field or __key__'
})

const datalistId = `meta-paths-${useId()}`

function addColumn() {
  const id = `col_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`
  columns.value.push({
    id,
    label: '',
    path: '',
    format: 'text' as TableColumnFormat
  })
}

function removeColumn(index: number) {
  columns.value.splice(index, 1)
}

function addRule(col: TableColumn) {
  if (!col.rules) col.rules = []
  col.rules.push({ op: 'gt', value: '', style: 'warning' } as ConditionalRule)
}

function removeRule(col: TableColumn, index: number) {
  col.rules?.splice(index, 1)
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

.rules-details {
  margin-top: 4px;
  border-top: 1px dashed oklch(var(--b3));
  padding-top: 6px;
}

.rules-details summary {
  cursor: pointer;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: oklch(var(--bc) / 0.6);
  user-select: none;
  display: flex;
  align-items: center;
  gap: 6px;
}

.rules-details summary:hover {
  color: oklch(var(--bc));
}

.rule-count {
  background: oklch(var(--p) / 0.15);
  color: oklch(var(--p));
  border-radius: 999px;
  padding: 0 6px;
  font-size: 10px;
}

.rules-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
}

.rule-row {
  display: flex;
  gap: 4px;
  align-items: center;
}

.rule-op { max-width: 90px; }
.rule-value { flex: 1; }
.rule-style { max-width: 110px; }

.rule-style.style-success { color: oklch(var(--su)); }
.rule-style.style-info    { color: oklch(var(--in)); }
.rule-style.style-warning { color: oklch(var(--wa)); }
.rule-style.style-error   { color: oklch(var(--er)); }

.rule-remove {
  padding: 2px 6px;
  font-size: 12px;
}

.btn-add-rule {
  align-self: flex-start;
  background: none;
  border: 1px dashed oklch(var(--b3));
  border-radius: 4px;
  color: oklch(var(--bc) / 0.6);
  cursor: pointer;
  font-size: 11px;
  padding: 3px 8px;
  margin-top: 2px;
}

.btn-add-rule:hover {
  border-color: oklch(var(--a));
  color: oklch(var(--a));
}

.help-text {
  font-size: 10px;
  color: oklch(var(--bc) / 0.5);
  margin-top: 2px;
}
</style>
