<template>
  <div class="control-fields" :class="{ compact }">
    <div class="form-group">
      <label>Mode</label>
      <select :value="config.mode" class="form-input" @change="set('mode', value($event) as SliderWidgetConfig['mode'])">
        <option value="core">CORE (Pub/Sub)</option>
        <option value="kv">KV (Stateful)</option>
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
          placeholder="device-config"
          @input="set('kvBucket', value($event))"
        />
        <div v-if="errors?.kvBucket" class="error-text">{{ errors.kvBucket }}</div>
      </div>
      <div class="form-group">
        <label>KV Key</label>
        <input
          :value="config.kvKey"
          type="text"
          class="form-input"
          :class="{ 'has-error': errors?.kvKey }"
          :placeholder="perThing ? 'thing.{thing}.setpoint' : 'device.brightness'"
          @input="set('kvKey', value($event))"
        />
        <div v-if="errors?.kvKey" class="error-text">{{ errors.kvKey }}</div>
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
          :placeholder="perThing ? 'cmd.{thing}.setpoint' : 'device.set_brightness'"
          @input="set('publishSubject', value($event))"
        />
        <div v-if="errors?.publishSubject" class="error-text">{{ errors.publishSubject }}</div>
        <div v-else-if="!compact" class="help-text">Subject to publish value to</div>
      </div>
      <div class="form-group">
        <label>State Subject (optional)</label>
        <input
          :value="config.stateSubject"
          type="text"
          class="form-input"
          placeholder="device.brightness_changed"
          @input="set('stateSubject', value($event))"
        />
        <div class="help-text">Subject to listen to for updates. Defaults to the publish subject.</div>
      </div>
    </template>

    <div class="form-group">
      <label>Value Template (optional)</label>
      <input
        :value="config.valueTemplate"
        type="text"
        class="form-input"
        placeholder="{value}"
        @input="set('valueTemplate', value($event))"
      />
      <div class="help-text">
        Empty publishes the bare number. <span v-pre>{value}</span> is replaced, for example
        <span v-pre>{"brightness": {value}}</span>.
      </div>
    </div>

    <div class="form-group">
      <label>JSONPath (optional)</label>
      <input
        :value="config.jsonPath"
        type="text"
        class="form-input"
        :class="{ 'has-error': errors?.jsonPath }"
        placeholder="$.brightness"
        @input="set('jsonPath', value($event))"
      />
      <div v-if="errors?.jsonPath" class="error-text">{{ errors.jsonPath }}</div>
      <div v-else-if="!compact" class="help-text">Extract value from incoming JSON messages</div>
    </div>

    <div class="form-group">
      <label>Range</label>
      <div class="range-inputs">
        <div v-for="n in NUMBERS" :key="n.field" class="range-input-group">
          <label class="range-label">{{ n.label }}</label>
          <input
            :value="config[n.field]"
            type="number"
            class="form-input"
            step="any"
            @input="set(n.field, Number(value($event)))"
          />
        </div>
      </div>
    </div>

    <div class="form-group">
      <label>Unit (optional)</label>
      <input
        :value="config.unit"
        type="text"
        class="form-input"
        placeholder="%, °C, dB"
        @input="set('unit', value($event))"
      />
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input
          type="checkbox"
          :checked="!!config.confirmOnChange"
          @change="set('confirmOnChange', ($event.target as HTMLInputElement).checked)"
        />
        <span>Require confirmation before publishing value</span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
// The slider's settings, for the widget's configure modal and for a setpoint
// row on a map or floor-plan marker. It edits the config object itself.
import { computed } from 'vue'
import type { SliderWidgetConfig } from '@/types/dashboard'

const props = defineProps<{
  modelValue: SliderWidgetConfig
  errors?: Record<string, string>
  /** Tighter spacing and less explanatory text, for a row inside a marker. */
  compact?: boolean
  /**
   * True on a floor plan, where one row is applied to every Thing of a type
   * and `{thing}` is filled in per marker. A map marker has no Thing behind
   * it, so its examples must not suggest the token.
   */
  perThing?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [config: SliderWidgetConfig] }>()

const config = computed(() => props.modelValue)

const NUMBERS = [
  { field: 'min', label: 'Min' },
  { field: 'max', label: 'Max' },
  { field: 'step', label: 'Step' },
  { field: 'defaultValue', label: 'Default' },
] as const

function value(event: Event): string {
  return (event.target as HTMLInputElement | HTMLSelectElement).value
}

function set<K extends keyof SliderWidgetConfig>(field: K, v: SliderWidgetConfig[K]) {
  emit('update:modelValue', { ...props.modelValue, [field]: v })
}
</script>

<style scoped src="./controlFields.css"></style>
