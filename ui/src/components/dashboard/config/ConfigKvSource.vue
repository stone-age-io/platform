<template>
  <div class="config-kv-source">
    <div class="form-group">
      <label>KV Bucket</label>
      <input
        v-model="form.kvBucket"
        type="text"
        class="form-input"
        :class="{ 'has-error': errors.kvBucket }"
        :placeholder="bucketPlaceholder"
      />
      <div v-if="errors.kvBucket" class="error-text">{{ errors.kvBucket }}</div>
      <div v-else-if="bucketHelp" class="help-text">{{ bucketHelp }}</div>
    </div>

    <div class="form-group">
      <label>KV Key</label>
      <input
        v-model="form.kvKey"
        type="text"
        class="form-input"
        :class="{ 'has-error': errors.kvKey }"
        :placeholder="keyPlaceholder"
      />
      <div v-if="errors.kvKey" class="error-text">{{ errors.kvKey }}</div>
      <div v-else-if="keyHelp" class="help-text">{{ keyHelp }}</div>
    </div>

    <div v-if="showJsonPath" class="form-group">
      <label>JSONPath (optional)</label>
      <input
        v-model="form.jsonPath"
        type="text"
        class="form-input"
        :class="{ 'has-error': errors.jsonPath }"
        placeholder="$.data.value"
      />
      <div v-if="errors.jsonPath" class="error-text">{{ errors.jsonPath }}</div>
      <div v-else class="help-text">Extract specific data from JSON values</div>
    </div>

    <div class="help-text variable-hint">
      Supports variables like <code v-pre>{{device_id}}</code>.
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'

withDefaults(defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
  bucketPlaceholder?: string
  keyPlaceholder?: string
  bucketHelp?: string
  keyHelp?: string
  showJsonPath?: boolean
}>(), {
  bucketPlaceholder: 'my-bucket',
  keyPlaceholder: 'app.version',
  bucketHelp: '',
  keyHelp: '',
  showJsonPath: true,
})
</script>

<style scoped>
.variable-hint {
  font-size: 11px;
  opacity: 0.7;
  margin-top: -4px;
}
</style>
