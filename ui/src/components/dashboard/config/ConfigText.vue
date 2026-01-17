<template>
  <div class="config-text">
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
