<template>
  <div class="config-slider">
    <div class="form-group">
      <label>Mode</label>
      <select v-model="form.sliderMode" class="form-input">
        <option value="core">CORE (Pub/Sub)</option>
        <option value="kv">KV (Stateful)</option>
      </select>
      <div class="help-text">
        KV mode writes directly to KV store. CORE mode uses pub/sub messaging.
      </div>
    </div>

    <!-- KV Mode Fields -->
    <template v-if="form.sliderMode === 'kv'">
      <div class="form-group">
        <label>KV Bucket</label>
        <input 
          v-model="form.kvBucket" 
          type="text" 
          class="form-input"
          :class="{ 'has-error': errors.kvBucket }"
          placeholder="device-config"
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
          placeholder="device.brightness"
        />
        <div v-if="errors.kvKey" class="error-text">
          {{ errors.kvKey }}
        </div>
      </div>
    </template>

    <!-- CORE Mode Fields -->
    <template v-if="form.sliderMode === 'core'">
      <div class="form-group">
        <label>Publish Subject</label>
        <input 
          v-model="form.subject" 
          type="text" 
          class="form-input"
          :class="{ 'has-error': errors.subject }"
          placeholder="device.set_brightness"
        />
        <div v-if="errors.subject" class="error-text">
          {{ errors.subject }}
        </div>
        <div class="help-text">
          Subject to publish value to
        </div>
      </div>

      <div class="form-group">
        <label>State Subject (optional)</label>
        <input 
          v-model="form.sliderStateSubject" 
          type="text" 
          class="form-input"
          placeholder="device.brightness_changed"
        />
        <div class="help-text">
          Subject to listen to for updates. Defaults to Publish Subject if empty.
        </div>
      </div>
    </template>

    <div class="form-group">
      <label>Value Template</label>
      <input 
        v-model="form.sliderValueTemplate" 
        type="text" 
        class="form-input"
        placeholder="{{value}}"
      />
      <div class="help-text">
        Use <span v-pre>{{value}}</span> as placeholder. Example: <span v-pre>{"brightness": {{value}}}</span>
      </div>
    </div>

    <div class="form-group">
      <label>JSONPath Extraction (optional)</label>
      <input 
        v-model="form.jsonPath" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.jsonPath }"
        placeholder="$.brightness"
      />
      <div v-if="errors.jsonPath" class="error-text">
        {{ errors.jsonPath }}
      </div>
      <div class="help-text">
        Extract value from incoming JSON messages
      </div>
    </div>

    <div class="form-group">
      <label>Range</label>
      <div class="range-inputs">
        <div class="range-input-group">
          <label class="range-label">Min</label>
          <input 
            v-model.number="form.sliderMin" 
            type="number" 
            class="form-input"
            placeholder="0"
            step="any"
          />
        </div>
        <div class="range-input-group">
          <label class="range-label">Max</label>
          <input 
            v-model.number="form.sliderMax" 
            type="number" 
            class="form-input"
            placeholder="100"
            step="any"
          />
        </div>
        <div class="range-input-group">
          <label class="range-label">Step</label>
          <input 
            v-model.number="form.sliderStep" 
            type="number" 
            class="form-input"
            placeholder="1"
            step="any"
          />
        </div>
      </div>
    </div>

    <div class="form-group">
      <label>Default Value</label>
      <input 
        v-model.number="form.sliderDefault" 
        type="number" 
        class="form-input"
        placeholder="50"
        step="any"
      />
    </div>

    <div class="form-group">
      <label>Unit (optional)</label>
      <input 
        v-model="form.sliderUnit" 
        type="text" 
        class="form-input"
        placeholder="%, °C, dB"
      />
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" v-model="form.sliderConfirm" />
        <span>Require confirmation before publishing value</span>
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
