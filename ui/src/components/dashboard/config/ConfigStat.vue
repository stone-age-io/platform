<template>
  <div class="config-stat">
    <div class="form-group">
      <label>Format (optional)</label>
      <input 
        v-model="form.statFormat" 
        type="text" 
        class="form-input"
        placeholder="{value}°C"
      />
      <div class="help-text">
        Use {value} as placeholder. Example: "{value}°C" or "$%{value}"
      </div>
    </div>

    <div class="form-group">
      <label>Unit (optional)</label>
      <input 
        v-model="form.statUnit" 
        type="text" 
        class="form-input"
        placeholder="°C, MB, req/s"
      />
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" v-model="form.statShowTrend" />
        <span>Show trend indicator</span>
      </label>
    </div>

    <div v-if="form.statShowTrend" class="form-group">
      <label>Trend Window (messages)</label>
      <input 
        v-model.number="form.statTrendWindow" 
        type="number" 
        class="form-input"
        min="2"
        max="100"
        placeholder="10"
      />
      <div class="help-text">
        Compare current value to N messages ago
      </div>
    </div>

    <div class="form-group">
      <label>Conditional Formatting</label>
      <ThresholdEditor v-model="form.thresholds" />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import ThresholdEditor from '../ThresholdEditor.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
