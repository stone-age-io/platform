<template>
  <div class="config-chart">
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

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
