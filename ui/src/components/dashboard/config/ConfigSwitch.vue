<template>
  <SwitchFields v-model="form.switchConfig" :errors="errors" />
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import SwitchFields from './SwitchFields.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
