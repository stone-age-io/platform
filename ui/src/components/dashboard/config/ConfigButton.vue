<template>
  <div class="config-button">
    <div class="form-group">
      <label>Button Label</label>
      <input 
        v-model="form.buttonLabel" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.buttonLabel }"
        placeholder="Send Message"
      />
      <div v-if="errors.buttonLabel" class="error-text">
        {{ errors.buttonLabel }}
      </div>
    </div>

    <div class="form-group">
      <label>Button Color</label>
      <select 
        v-model="form.buttonColor" 
        class="form-input" 
        :style="{ color: form.buttonColor || 'inherit' }"
      >
        <option value="">Default (Primary)</option>
        <option value="var(--color-secondary)" style="color: var(--color-secondary)">Secondary</option>
        <option value="var(--color-accent)" style="color: var(--color-accent)">Accent</option>
        <option value="var(--color-success)" style="color: var(--color-success)">Success</option>
        <option value="var(--color-warning)" style="color: var(--color-warning)">Warning</option>
        <option value="var(--color-error)" style="color: var(--color-error)">Danger</option>
        <option value="var(--color-info)" style="color: var(--color-info)">Info</option>
        <option value="var(--panel)" style="color: var(--text); background: var(--panel)">Neutral</option>
      </select>
    </div>
    
    <div class="form-group">
      <label>Action Type</label>
      <select v-model="form.buttonActionType" class="form-input">
        <option value="publish">Publish (Fire & Forget)</option>
        <option value="request">Request (Wait for Reply)</option>
      </select>
      <div class="help-text">
        {{ form.buttonActionType === 'publish' 
           ? 'Sends a message without waiting for a response.' 
           : 'Sends a message and waits for a reply. Displays response in a modal.' }}
      </div>
    </div>

    <div v-if="form.buttonActionType === 'request'" class="form-group">
      <label>Timeout (ms)</label>
      <input 
        v-model.number="form.buttonTimeout" 
        type="number" 
        class="form-input"
        min="100"
        step="100"
        placeholder="1000"
      />
      <div class="help-text">
        Max time to wait for a response before failing.
      </div>
    </div>
    
    <div class="form-group">
      <label>Subject</label>
      <input 
        v-model="form.subject" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.subject }"
        placeholder="service.action"
      />
      <div v-if="errors.subject" class="error-text">
        {{ errors.subject }}
      </div>
    </div>
    
    <div class="form-group">
      <label>Message Payload</label>
      <textarea 
        v-model="form.buttonPayload" 
        class="form-textarea"
        :class="{ 'has-error': errors.buttonPayload }"
        rows="6"
        placeholder='{"action": "clicked"}'
      />
      <div v-if="errors.buttonPayload" class="error-text">
        {{ errors.buttonPayload }}
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
