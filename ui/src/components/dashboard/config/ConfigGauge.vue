<template>
  <div class="config-gauge">
    <div class="form-group">
      <label>Gauge Range</label>
      <div class="range-inputs">
        <div class="range-input-group">
          <label class="range-label">Min</label>
          <input 
            v-model.number="form.gaugeMin" 
            type="number" 
            class="form-input"
            placeholder="0"
            step="any"
          />
        </div>
        <div class="range-input-group">
          <label class="range-label">Max</label>
          <input 
            v-model.number="form.gaugeMax" 
            type="number" 
            class="form-input"
            placeholder="100"
            step="any"
          />
        </div>
      </div>
    </div>

    <div class="form-group">
      <label>Unit (optional)</label>
      <input 
        v-model="form.gaugeUnit" 
        type="text" 
        class="form-input"
        placeholder="%, °C, RPM"
      />
    </div>

    <div class="form-group">
      <label>Color Zones</label>
      <GaugeZoneEditor v-model="form.gaugeZones" />
      <div class="help-text">
        Define color ranges for different value zones
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'
import GaugeZoneEditor from '../GaugeZoneEditor.vue'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>
