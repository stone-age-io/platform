<template>
  <div class="form-group">
    <label>Widget Title</label>
    <input 
      v-model="form.title" 
      type="text" 
      class="form-input"
      :class="{ 'has-error': errors.title }"
      placeholder="My Widget"
    />
    <div v-if="errors.title" class="error-text">
      {{ errors.title }}
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
