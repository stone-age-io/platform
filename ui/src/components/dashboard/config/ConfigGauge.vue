<template>
  <GaugeFields v-model="form.gaugeConfig" :errors="errors" show-style />
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import GaugeFields from './GaugeFields.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
