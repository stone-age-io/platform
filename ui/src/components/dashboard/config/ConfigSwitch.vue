<template>
  <div class="config-switch">
    <div class="form-group">
      <label>Mode</label>
      <select v-model="form.switchMode" class="form-input">
        <option value="kv">KV (Stateful)</option>
        <option value="core">CORE (Pub/Sub)</option>
      </select>
      <div class="help-text">
        KV mode writes directly to KV store. CORE mode uses pub/sub messaging.
      </div>
    </div>

    <!-- KV Mode Fields -->
    <template v-if="form.switchMode === 'kv'">
      <div class="form-group">
        <label>KV Bucket</label>
        <input 
          v-model="form.kvBucket" 
          type="text" 
          class="form-input"
          :class="{ 'has-error': errors.kvBucket }"
          placeholder="device-states"
        />
        <div v-if="errors.kvBucket" class="error-text">
          {{ errors.kvBucket }}
        </div>
        <div v-else class="help-text">
          Bucket where switch state will be stored
        </div>
      </div>

      <div class="form-group">
        <label>KV Key</label>
        <input 
          v-model="form.kvKey" 
          type="text" 
          class="form-input"
          :class="{ 'has-error': errors.kvKey }"
          placeholder="device.switch"
        />
        <div v-if="errors.kvKey" class="error-text">
          {{ errors.kvKey }}
        </div>
        <div v-else class="help-text">
          Key to store switch state (will be watched for changes)
        </div>
      </div>
    </template>

    <!-- CORE Mode Fields -->
    <template v-if="form.switchMode === 'core'">
      <div class="form-group">
        <label>Publish Subject</label>
        <input 
          v-model="form.subject" 
          type="text" 
          class="form-input"
          :class="{ 'has-error': errors.subject }"
          placeholder="device.control"
        />
        <div v-if="errors.subject" class="error-text">
          {{ errors.subject }}
        </div>
        <div v-else class="help-text">
          Subject to publish control commands
        </div>
      </div>

      <div class="form-group">
        <label>State Subject (optional)</label>
        <input 
          v-model="form.switchStateSubject" 
          type="text" 
          class="form-input"
          placeholder="device.state (leave empty to use publish subject)"
        />
        <div class="help-text">
          Subject to subscribe for state confirmation. Defaults to publish subject if empty.
        </div>
      </div>

      <div class="form-group">
        <label>Default State</label>
        <select v-model="form.switchDefaultState" class="form-input">
          <option value="off">OFF</option>
          <option value="on">ON</option>
        </select>
        <div class="help-text">
          Initial state to display before receiving state updates
        </div>
      </div>
    </template>

    <div class="form-group">
      <label>ON Payload</label>
      <textarea 
        v-model="form.switchOnPayload" 
        class="form-textarea"
        rows="3"
        placeholder='{"state": "on"}'
      />
    </div>

    <div class="form-group">
      <label>OFF Payload</label>
      <textarea 
        v-model="form.switchOffPayload" 
        class="form-textarea"
        rows="3"
        placeholder='{"state": "off"}'
      />
    </div>

    <div class="form-group">
      <label>Labels</label>
      <div class="label-inputs">
        <input 
          v-model="form.switchLabelOn" 
          type="text" 
          class="form-input"
          placeholder="ON"
        />
        <input 
          v-model="form.switchLabelOff" 
          type="text" 
          class="form-input"
          placeholder="OFF"
        />
      </div>
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" v-model="form.switchConfirm" />
        <span>Require confirmation before changing state</span>
      </label>
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
