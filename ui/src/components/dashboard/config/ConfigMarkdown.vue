<template>
  <div class="config-markdown">
    <!-- Data Source Section -->
    <div class="config-section">
      <div class="section-title">Data Source (Optional)</div>
      <div class="form-group">
        <label>Source Type</label>
        <select v-model="form.dataSourceType" class="form-input">
          <option value="none">None (Static)</option>
          <option value="subscription">NATS Subject (Core/JetStream)</option>
          <option value="kv">Key-Value Store (KV)</option>
        </select>
        <div class="help-text">
          Data from this source will be available as variables in your markdown (e.g. <code v-pre>{{value}}</code>).
        </div>
      </div>

      <!-- Subscription Mode -->
      <ConfigDataSource 
        v-if="form.dataSourceType === 'subscription'"
        :form="form" 
        :errors="errors" 
      />

      <!-- KV Inputs -->
      <template v-if="form.dataSourceType === 'kv'">
        <div class="form-group">
          <label>KV Bucket</label>
          <input 
            v-model="form.kvBucket" 
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors.kvBucket }"
            placeholder="my-bucket"
          />
          <div v-if="errors.kvBucket" class="error-text">{{ errors.kvBucket }}</div>
        </div>
        <div class="form-group">
          <label>KV Key</label>
          <input 
            v-model="form.kvKey" 
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors.kvKey }"
            placeholder="status.key"
          />
          <div v-if="errors.kvKey" class="error-text">{{ errors.kvKey }}</div>
        </div>
        <div class="form-group">
          <label>JSONPath (optional)</label>
          <input 
            v-model="form.jsonPath" 
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors.jsonPath }"
            placeholder="$.status"
          />
          <div v-if="errors.jsonPath" class="error-text">{{ errors.jsonPath }}</div>
        </div>
      </template>
    </div>

    <!-- Content Section -->
    <div class="config-section">
      <div class="section-title">Content</div>
      <div class="form-group">
        <textarea 
          v-model="form.markdownContent" 
          class="form-textarea"
          rows="10"
          placeholder="# Hello World"
        />
        <div class="help-text">
          Supports standard Markdown. You can use dashboard variables like <code v-pre>{{name}}</code>.
          <br>
          HTML is rendered (be careful with scripts).
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import ConfigDataSource from './ConfigDataSource.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>

<style scoped>
.config-section {
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 20px;
}

.section-title {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 12px;
  letter-spacing: 0.5px;
}

.form-textarea {
  font-family: var(--mono);
  font-size: 13px;
  line-height: 1.5;
}

.error-text {
  font-size: 12px;
  color: var(--color-error);
  margin-top: 4px;
}
</style>
