<template>
  <SliderFields v-model="form.sliderConfig" :errors="errors" />
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import SliderFields from './SliderFields.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
