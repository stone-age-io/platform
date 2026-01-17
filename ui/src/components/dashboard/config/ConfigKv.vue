<template>
  <div class="config-kv">
    <div class="form-group">
      <label>KV Bucket Name</label>
      <input 
        v-model="form.kvBucket" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.kvBucket }"
        placeholder="my-bucket"
      />
      <div v-if="errors.kvBucket" class="error-text">
        {{ errors.kvBucket }}
      </div>
    </div>
    
    <div class="form-group">
      <label>KV Key</label>
      <input 
        v-model="form.kvKey" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.kvKey }"
        placeholder="app.version"
      />
      <div v-if="errors.kvKey" class="error-text">
        {{ errors.kvKey }}
      </div>
    </div>

    <div class="form-group">
      <label>JSONPath Filter (optional)</label>
      <input 
        v-model="form.jsonPath" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.jsonPath }"
        placeholder="$.data.value"
      />
      <div v-if="errors.jsonPath" class="error-text">
        {{ errors.jsonPath }}
      </div>
      <div v-else class="help-text">
        Extract specific data from JSON values
      </div>
    </div>

    <div class="form-group">
      <label>Conditional Formatting (Single Value Mode)</label>
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
