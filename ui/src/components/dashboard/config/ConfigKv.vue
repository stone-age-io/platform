<template>
  <div class="config-kv">
    <ConfigKvSource :form="form" :errors="errors" />

    <div class="form-group">
      <label>Conditional Formatting (Single Value Mode)</label>
      <ThresholdEditor v-model="form.thresholds" />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import ThresholdEditor from '../ThresholdEditor.vue'
import ConfigKvSource from './ConfigKvSource.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
