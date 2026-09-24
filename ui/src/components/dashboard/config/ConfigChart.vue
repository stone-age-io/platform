<template>
  <div class="config-chart">
    <div class="form-group">
      <label>Chart Type</label>
      <select v-model="form.chartType" class="form-input">
        <option value="line">Line</option>
        <option value="bar">Bar</option>
        <option value="timeline">State Timeline</option>
      </select>
      <div v-if="form.chartType === 'timeline'" class="help-text">
        One row per series; each run of the same value is one segment. A row
        stays blank until its first message. A device that goes silent keeps
        its last state stretched to now, so have it republish its state
        periodically if silence should show.
      </div>
    </div>

    <div class="form-group">
      <label>Series</label>
      <div v-if="errors.chartSeries" class="error-text mb-2">{{ errors.chartSeries }}</div>

      <div class="series-list">
        <div v-for="(s, i) in form.chartSeries" :key="i" class="series-item">
          <div class="field-row">
            <input
              v-model="s.label"
              type="text"
              class="form-input"
              :placeholder="`Series ${i + 1}`"
            />
            <button
              class="btn-remove"
              title="Remove series"
              :disabled="form.chartSeries.length === 1"
              @click="removeSeries(i)"
            >✕</button>
          </div>
          <input
            v-model="s.path"
            type="text"
            class="form-input font-mono"
            placeholder="$.value or $.pumps.s1.flow"
          />
          <input
            v-model="s.subject"
            type="text"
            class="form-input font-mono"
            placeholder="Subject filter (optional)"
          />
        </div>
      </div>

      <button class="btn-add" @click="addSeries">+ Add Series</button>
      <div class="help-text">
        Each series reads the whole message. One message can feed every series,
        or each can come from its own subject: set a <strong>subject filter</strong>
        (exact, variables like <code v-pre>{{device_id}}</code> allowed) when
        several subjects publish the same shape. Blank = every message.
      </div>
    </div>

    <div v-if="form.chartType === 'timeline'" class="form-group">
      <label>Value Colours</label>
      <ThresholdEditor v-model="form.chartThresholds" show-label />
      <div class="help-text">
        First match wins, e.g. <code>== true</code> → Success, label <em>running</em>.
        A value no rule matches keeps its own colour and shows as-is.
      </div>
    </div>

    <div class="form-group">
      <label>Time Window (optional)</label>
      <input
        v-model="form.chartWindow"
        type="text"
        class="form-input"
        :class="{ 'has-error': errors.chartWindow }"
        placeholder="30m"
      />
      <div v-if="errors.chartWindow" class="error-text">
        {{ errors.chartWindow }}
      </div>
      <div v-else class="help-text">
        Show only the last <code>30m</code>, <code>1h</code>, <code>1h30m</code>…
        Empty = keep the last N messages. The buffer size still caps memory.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import ThresholdEditor from '../ThresholdEditor.vue'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

function addSeries() {
  props.form.chartSeries.push({ label: '', path: '$.value', subject: '' })
}

function removeSeries(i: number) {
  if (props.form.chartSeries.length > 1) props.form.chartSeries.splice(i, 1)
}
</script>

<style scoped>
.series-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 8px;
}

.series-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  padding: 8px;
  background: oklch(var(--b2) / 0.3);
}

.field-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.field-row .form-input {
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
}

.btn-remove:hover:not(:disabled) {
  background: oklch(var(--er) / 0.1);
  border-color: oklch(var(--er));
}

.btn-remove:disabled {
  opacity: 0.3;
  cursor: not-allowed;
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
}

.btn-add:hover {
  border-color: oklch(var(--a));
  color: oklch(var(--a));
  background: oklch(var(--a) / 0.05);
}

.mb-2 { margin-bottom: 8px; }
</style>
