<template>
  <div class="control-fields" :class="{ compact }">
    <div v-if="showStyle" class="form-group">
      <label>Style</label>
      <select
        :value="config.style === 'bar' ? 'bar' : 'arc'"
        class="form-input"
        @change="set('style', value($event) === 'bar' ? 'bar' : undefined)"
      >
        <option value="arc">Arc</option>
        <option value="bar">Bar (level)</option>
      </select>
    </div>

    <div class="form-group">
      <label>Range</label>
      <div class="range-inputs">
        <div class="range-input-group">
          <label class="range-label">Min</label>
          <input
            :value="config.min"
            type="number"
            class="form-input"
            placeholder="0"
            step="any"
            @input="set('min', Number(value($event)))"
          />
        </div>
        <div class="range-input-group">
          <label class="range-label">Max</label>
          <input
            :value="config.max"
            type="number"
            class="form-input"
            placeholder="100"
            step="any"
            @input="set('max', Number(value($event)))"
          />
        </div>
      </div>
      <div v-if="errors?.gaugeRange" class="error-text">{{ errors.gaugeRange }}</div>
    </div>

    <div class="form-group">
      <label>Unit (optional)</label>
      <input
        :value="config.unit"
        type="text"
        class="form-input"
        placeholder="%, °C, RPM"
        @input="set('unit', value($event) || undefined)"
      />
    </div>

    <div class="form-group">
      <label>Decimal places (optional)</label>
      <input
        :value="config.decimals"
        type="number"
        min="0"
        max="6"
        step="1"
        class="form-input"
        :class="{ 'has-error': errors?.gaugeDecimals }"
        placeholder="Automatic"
        @input="set('decimals', value($event) === '' ? undefined : Number(value($event)))"
      />
      <div v-if="errors?.gaugeDecimals" class="error-text">{{ errors.gaugeDecimals }}</div>
    </div>

    <div class="form-group">
      <label>Color Zones</label>
      <GaugeZoneEditor :model-value="config.zones || []" @update:model-value="set('zones', $event)" />
      <div v-if="!compact" class="help-text">Define color ranges for different value zones</div>
    </div>
  </div>
</template>

<script setup lang="ts">
// The gauge's scale, for the widget's configure modal and for a level row on a
// map or floor-plan marker. It edits the config object itself. Where the value
// comes from is not here: a widget has its data source panel, and a marker row
// its own source fields.
import { computed } from 'vue'
import type { GaugeWidgetConfig } from '@/types/dashboard'
import GaugeZoneEditor from '../GaugeZoneEditor.vue'

const props = defineProps<{
  modelValue: GaugeWidgetConfig
  errors?: Record<string, string>
  /** Arc or bar. Off for a marker row, which is always a bar. */
  showStyle?: boolean
  /** Tighter spacing and less explanatory text, for a row inside a marker. */
  compact?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [config: GaugeWidgetConfig] }>()

const config = computed(() => props.modelValue)

function value(event: Event): string {
  return (event.target as HTMLInputElement | HTMLSelectElement).value
}

function set<K extends keyof GaugeWidgetConfig>(field: K, v: GaugeWidgetConfig[K]) {
  emit('update:modelValue', { ...props.modelValue, [field]: v })
}
</script>

<style scoped src="./controlFields.css"></style>
