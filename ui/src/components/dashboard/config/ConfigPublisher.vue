<template>
  <div class="config-publisher">
    <div class="form-group">
      <label>Default Subject</label>
      <input 
        v-model="form.publisherDefaultSubject" 
        type="text" 
        class="form-input"
        placeholder="service.action"
      />
      <div class="help-text">
        Initial subject when widget loads.
      </div>
    </div>

    <div class="form-group">
      <label>Default Payload</label>
      <textarea 
        v-model="form.publisherDefaultPayload" 
        class="form-textarea"
        rows="6"
        placeholder='{"key": "value"}'
      />
      <div class="help-text">
        Initial payload. Supports JSON or raw text.
      </div>
    </div>

    <div class="form-group">
      <label>Request Timeout (ms)</label>
      <input 
        v-model.number="form.publisherTimeout" 
        type="number" 
        class="form-input"
        min="100"
        step="100"
        placeholder="2000"
      />
      <div class="help-text">
        Max time to wait for a reply in Request mode.
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
