<template>
  <div class="control-fields" :class="{ compact }">
    <div class="form-group">
      <label>Mode</label>
      <select :value="config.mode" class="form-input" @change="set('mode', value($event) as SwitchWidgetConfig['mode'])">
        <option value="kv">KV (Stateful)</option>
        <option value="core">CORE (Pub/Sub)</option>
      </select>
      <div v-if="!compact" class="help-text">
        KV mode writes directly to KV store. CORE mode uses pub/sub messaging.
      </div>
    </div>

    <template v-if="config.mode === 'kv'">
      <div class="form-group">
        <label>KV Bucket</label>
        <input
          :value="config.kvBucket"
          type="text"
          class="form-input"
          :class="{ 'has-error': errors?.kvBucket }"
          placeholder="device-states"
          @input="set('kvBucket', value($event))"
        />
        <div v-if="errors?.kvBucket" class="error-text">{{ errors.kvBucket }}</div>
        <div v-else-if="!compact" class="help-text">Bucket where switch state will be stored</div>
      </div>
      <div class="form-group">
        <label>KV Key</label>
        <input
          :value="config.kvKey"
          type="text"
          class="form-input"
          :class="{ 'has-error': errors?.kvKey }"
          placeholder="device.switch"
          @input="set('kvKey', value($event))"
        />
        <div v-if="errors?.kvKey" class="error-text">{{ errors.kvKey }}</div>
        <div v-else-if="!compact" class="help-text">Key to store switch state (will be watched for changes)</div>
      </div>
    </template>

    <template v-else>
      <div class="form-group">
        <label>Publish Subject</label>
        <input
          :value="config.publishSubject"
          type="text"
          class="form-input"
          :class="{ 'has-error': errors?.publishSubject }"
          placeholder="device.control"
          @input="set('publishSubject', value($event))"
        />
        <div v-if="errors?.publishSubject" class="error-text">{{ errors.publishSubject }}</div>
        <div v-else-if="!compact" class="help-text">Subject to publish control commands</div>
      </div>
      <div class="form-group">
        <label>State Subject (optional)</label>
        <input
          :value="config.stateSubject"
          type="text"
          class="form-input"
          placeholder="device.state"
          @input="set('stateSubject', value($event))"
        />
        <div class="help-text">Subscribe for state updates. Defaults to the publish subject.</div>
      </div>
      <div class="form-group">
        <label>Default State</label>
        <select
          :value="config.defaultState || 'off'"
          class="form-input"
          @change="set('defaultState', value($event) as 'on' | 'off')"
        >
          <option value="off">OFF</option>
          <option value="on">ON</option>
        </select>
        <div v-if="!compact" class="help-text">Shown until a state update arrives</div>
      </div>
    </template>

    <div class="pair">
      <div v-for="p in payloads" :key="p.field" class="form-group">
        <label>{{ p.label }} Payload</label>
        <textarea
          :value="p.text.value"
          class="form-textarea"
          :rows="compact ? 1 : 3"
          :placeholder="p.placeholder"
          @input="setPayload(p, value($event))"
        />
        <div v-if="notJson(p.text.value)" class="error-text">
          Not valid JSON, so it is sent as this exact text.
        </div>
      </div>
    </div>

    <div class="form-group">
      <label>Labels</label>
      <div class="label-inputs">
        <input
          :value="config.labels?.on"
          type="text"
          class="form-input"
          placeholder="ON"
          @input="setLabel('on', value($event))"
        />
        <input
          :value="config.labels?.off"
          type="text"
          class="form-input"
          placeholder="OFF"
          @input="setLabel('off', value($event))"
        />
      </div>
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input
          type="checkbox"
          :checked="!!config.confirmOnChange"
          @change="set('confirmOnChange', ($event.target as HTMLInputElement).checked)"
        />
        <span>Require confirmation before changing state</span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
// The switch's settings, for the widget's configure modal and for a switch row
// on a map or floor-plan marker. It edits the config object itself: there is no
// second copy of these fields to keep in step.
import { computed, ref, watch, type Ref } from 'vue'
import type { SwitchWidgetConfig } from '@/types/dashboard'

const props = defineProps<{
  modelValue: SwitchWidgetConfig
  errors?: Record<string, string>
  /** Tighter spacing and no explanatory text, for a row inside a marker. */
  compact?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [config: SwitchWidgetConfig] }>()

const config = computed(() => props.modelValue)

function value(event: Event): string {
  return (event.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement).value
}

function set<K extends keyof SwitchWidgetConfig>(field: K, v: SwitchWidgetConfig[K]) {
  emit('update:modelValue', { ...props.modelValue, [field]: v })
}

function setLabel(which: 'on' | 'off', text: string) {
  set('labels', { ...props.modelValue.labels, [which]: text })
}

// --- Payloads ---
// A payload is stored as a value, not as text: `{"state":"on"}` is an object,
// `true` a boolean, and anything that is not JSON is the text itself. The box
// keeps what was typed, so half-typed JSON is not rewritten under the cursor.

interface PayloadField {
  field: 'onPayload' | 'offPayload'
  label: string
  placeholder: string
  text: Ref<string>
}

function toText(payload: unknown): string {
  return typeof payload === 'object' && payload !== null ? JSON.stringify(payload) : String(payload ?? '')
}

function toPayload(text: string): unknown {
  try { return JSON.parse(text) } catch { return text }
}

const payloads: PayloadField[] = [
  { field: 'onPayload', label: 'ON', placeholder: '{"state":"on"}', text: ref(toText(props.modelValue.onPayload)) },
  { field: 'offPayload', label: 'OFF', placeholder: '{"state":"off"}', text: ref(toText(props.modelValue.offPayload)) },
]

// Refill the box only when the stored payload is not what the box already
// says, which means it changed from outside (another row, a type change).
for (const p of payloads) {
  watch(() => props.modelValue[p.field], (stored) => {
    if (JSON.stringify(toPayload(p.text.value)) !== JSON.stringify(stored)) {
      p.text.value = toText(stored)
    }
  })
}

function setPayload(p: PayloadField, text: string) {
  p.text.value = text
  set(p.field, toPayload(text))
}

/** Looks like JSON was meant, and is not: the usual cause is a missing quote. */
function notJson(text: string): boolean {
  const t = text.trim()
  if (!t.startsWith('{') && !t.startsWith('[')) return false
  try { JSON.parse(t); return false } catch { return true }
}
</script>

<style scoped src="./controlFields.css"></style>
