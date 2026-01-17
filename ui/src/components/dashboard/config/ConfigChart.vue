<template>
  <!-- Currently no specific chart config, but ready for future options -->
  <div class="config-chart"></div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
