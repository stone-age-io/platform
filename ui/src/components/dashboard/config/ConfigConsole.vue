<template>
  <div class="config-console">
    <div class="form-group">
      <label>Font Size (px)</label>
      <input 
        v-model.number="form.consoleFontSize" 
        type="number" 
        class="form-input"
        min="8"
        max="24"
        placeholder="12"
      />
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" v-model="form.consoleShowTimestamp" />
        <span>Show Timestamp</span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
