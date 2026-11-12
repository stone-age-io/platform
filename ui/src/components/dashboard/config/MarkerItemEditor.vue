<template>
  <div class="item-editor">
    <div class="item-header">
      <span class="item-badge" :class="item.type">
        <component :is="getItemIcon(item.type)" :size="16" aria-hidden="true" />
      </span>
      <span class="item-label">{{ item.label || 'Unnamed Item' }}</span>
      <button 
        class="btn-icon-small danger" 
        @click="$emit('remove')"
        title="Remove item"
      >
        ✕
      </button>
    </div>
    
    <div class="item-body">
      <div class="form-row">
        <label>Type</label>
        <select :value="item.type" @change="handleTypeChange" class="form-input">
          <option value="publish">Publish (Button)</option>
          <option value="switch">Switch (Toggle)</option>
          <option value="text">Text (Display)</option>
          <option value="kv">KV (Display)</option>
          <option value="slider">Slider (Setpoint)</option>
          <option value="gauge">Gauge (Level)</option>
        </select>
      </div>
      
      <div class="form-row">
        <label>Label</label>
        <input 
          :value="item.label"
          @input="updateField('label', ($event.target as HTMLInputElement).value)"
          type="text" 
          class="form-input"
          placeholder="Item label"
        />
      </div>
      
      <template v-if="item.type === 'publish'">
        <div class="form-row">
          <label>Action Type</label>
          <select 
            :value="item.actionType || 'publish'"
            @change="updateField('actionType', ($event.target as HTMLSelectElement).value)"
            class="form-input"
          >
            <option value="publish">Publish (Fire & Forget)</option>
            <option value="request">Request (Wait for Reply)</option>
          </select>
        </div>

        <div v-if="item.actionType === 'request'" class="form-row">
          <label>Timeout (ms)</label>
          <input 
            :value="item.timeout || 1000"
            @input="updateField('timeout', parseInt(($event.target as HTMLInputElement).value))"
            type="number"
            min="100"
            step="100"
            class="form-input"
          />
        </div>

        <div class="form-row">
          <label>Subject</label>
          <input 
            :value="item.subject"
            @input="updateField('subject', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors?.subject }"
            placeholder="building.a.control"
          />
          <div v-if="errors?.subject" class="error-text">{{ errors.subject }}</div>
        </div>
        
        <div class="form-row">
          <label>Payload</label>
          <textarea 
            :value="item.payload"
            @change="updateField('payload', ($event.target as HTMLTextAreaElement).value)"
            class="form-textarea"
            :class="{ 'has-error': errors?.payload }"
            rows="2"
            placeholder='{"action": "trigger"}'
          />
          <div v-if="errors?.payload" class="error-text">{{ errors.payload }}</div>
        </div>
      </template>
      
      <SwitchFields
        v-if="item.type === 'switch'"
        compact
        :model-value="switchConfig"
        :errors="errors"
        @update:model-value="emitUpdate({ switchConfig: $event })"
      />

      <template v-if="item.type === 'text'">
        <div class="form-row">
          <label>Subject</label>
          <input 
            :value="textConfig.subject"
            @input="updateTextField('subject', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors?.subject }"
            placeholder="sensors.temp"
          />
          <div v-if="errors?.subject" class="error-text">{{ errors.subject }}</div>
        </div>

        <div class="form-row">
          <label class="checkbox-label">
            <input 
              type="checkbox" 
              :checked="textConfig.useJetStream"
              @change="updateTextField('useJetStream', ($event.target as HTMLInputElement).checked)"
            />
            <span>Use JetStream (History)</span>
          </label>
        </div>

        <template v-if="textConfig.useJetStream">
          <div class="form-row">
            <label>Deliver Policy</label>
            <select 
              :value="textConfig.deliverPolicy || 'last'" 
              @change="updateTextField('deliverPolicy', ($event.target as HTMLSelectElement).value)"
              class="form-input"
            >
              <option value="all">All</option>
              <option value="last">Last</option>
              <option value="last_per_subject">Last Per Subject</option>
              <option value="new">New</option>
              <option value="by_start_time">By Time Window</option>
            </select>
          </div>

          <div v-if="textConfig.deliverPolicy === 'by_start_time'" class="form-row">
            <label>Time Window</label>
            <input 
              :value="textConfig.timeWindow"
              @input="updateTextField('timeWindow', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder="10m"
            />
          </div>
        </template>

        <div class="form-row">
          <label>JSONPath (optional)</label>
          <input 
            :value="textConfig.jsonPath"
            @input="updateTextField('jsonPath', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            placeholder="$.value"
          />
        </div>

        <div class="form-row">
          <label>Unit (optional)</label>
          <input 
            :value="textConfig.unit"
            @input="updateTextField('unit', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            placeholder="°C"
          />
        </div>
      </template>

      <template v-if="item.type === 'kv'">
        <div class="form-row">
          <label>KV Bucket</label>
          <input 
            :value="kvConfig.kvBucket"
            @input="updateKvField('kvBucket', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors?.kvBucket }"
            placeholder="my-bucket"
          />
          <div v-if="errors?.kvBucket" class="error-text">{{ errors.kvBucket }}</div>
        </div>

        <div class="form-row">
          <label>KV Key</label>
          <input 
            :value="kvConfig.kvKey"
            @input="updateKvField('kvKey', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            :class="{ 'has-error': errors?.kvKey }"
            placeholder="my-key"
          />
          <div v-if="errors?.kvKey" class="error-text">{{ errors.kvKey }}</div>
        </div>

        <div class="form-row">
          <label>JSONPath (optional)</label>
          <input 
            :value="kvConfig.jsonPath"
            @input="updateKvField('jsonPath', ($event.target as HTMLInputElement).value)"
            type="text" 
            class="form-input"
            placeholder="$.status"
          />
        </div>
      </template>
      <SliderFields
        v-if="item.type === 'slider'"
        compact
        :per-thing="perThing"
        :model-value="sliderConfig"
        :errors="errors"
        @update:model-value="emitUpdate({ sliderConfig: $event })"
      />

      <template v-if="item.type === 'gauge'">
        <div class="form-row">
          <label>Mode</label>
          <select
            :value="gaugeConfig.mode"
            @change="updateGaugeField('mode', ($event.target as HTMLSelectElement).value)"
            class="form-input"
          >
            <option value="kv">KV (Stateful)</option>
            <option value="core">CORE (Pub/Sub)</option>
          </select>
        </div>

        <template v-if="gaugeConfig.mode === 'kv'">
        <div class="form-row">
          <label>KV Bucket</label>
          <input
            :value="gaugeConfig.kvBucket"
            @input="updateGaugeField('kvBucket', ($event.target as HTMLInputElement).value)"
            type="text"
            class="form-input"
            placeholder="twin"
          />
        </div>
        <div class="form-row">
          <label>KV Key</label>
          <input
            :value="gaugeConfig.kvKey"
            @input="updateGaugeField('kvKey', ($event.target as HTMLInputElement).value)"
            type="text"
            class="form-input"
            :placeholder="perThing ? 'thing.{thing}.level' : 'device.level'"
          />
        </div>
        </template>

        <template v-else>
        <div class="form-row">
          <label>Subject</label>
          <input
            :value="gaugeConfig.subject"
            @input="updateGaugeField('subject', ($event.target as HTMLInputElement).value)"
            type="text"
            class="form-input"
            :placeholder="perThing ? 'sensor.{thing}.level' : 'sensors.level'"
          />
        </div>
        </template>

        <div class="form-row">
          <label>JSONPath (optional)</label>
          <input
            :value="gaugeConfig.jsonPath"
            @input="updateGaugeField('jsonPath', ($event.target as HTMLInputElement).value)"
            type="text"
            class="form-input"
            placeholder="$.level"
          />
        </div>

        <!-- The scale is the gauge widget's own form; only the source above is the row's. -->
        <GaugeFields
          compact
          :model-value="gaugeConfig"
          :errors="errors"
          @update:model-value="emitUpdate({ gaugeConfig: { ...gaugeConfig, ...$event } })"
        />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'
import { CircleHelp, Database, Gauge, Send, SlidersHorizontal, ToggleRight, Type } from '@lucide/vue'
import { computed } from 'vue'
import type { MapMarkerItem, SwitchWidgetConfig, MapItemTextConfig, MapItemKvConfig, SliderWidgetConfig, MapItemGaugeConfig, MapItemType } from '@/types/dashboard'
import { createDefaultItem } from '@/types/dashboard'
import GaugeFields from './GaugeFields.vue'
import SwitchFields from './SwitchFields.vue'
import SliderFields from './SliderFields.vue'

const props = defineProps<{
  item: MapMarkerItem
  errors?: Record<string, string>
  /**
   * True on a floor plan, where one item is applied to every Thing of a type
   * and `{thing}` is filled in per marker. A map marker is a single place with
   * no Thing behind it, so its examples must not suggest the token.
   */
  perThing?: boolean
}>()

const emit = defineEmits<{
  remove: []
  'update:item': [item: MapMarkerItem]
}>()

function getItemIcon(type: MapItemType): Component {
  switch (type) {
    case 'publish': return Send
    case 'switch': return ToggleRight
    case 'text': return Type
    case 'kv': return Database
    case 'slider': return SlidersHorizontal
    case 'gauge': return Gauge
    default: return CircleHelp
  }
}

const switchConfig = computed<SwitchWidgetConfig>(() => {
  return props.item.switchConfig || {
    mode: 'kv',
    kvBucket: '',
    kvKey: '',
    onPayload: { state: 'on' },
    offPayload: { state: 'off' },
    labels: { on: 'ON', off: 'OFF' }
  }
})

const textConfig = computed<MapItemTextConfig>(() => {
  return props.item.textConfig || {
    subject: '',
    jsonPath: '',
    unit: '',
    useJetStream: false,
    deliverPolicy: 'last',
    timeWindow: '10m'
  }
})

const kvConfig = computed<MapItemKvConfig>(() => {
  return props.item.kvConfig || {
    kvBucket: '',
    kvKey: '',
    jsonPath: ''
  }
})

// The fallbacks mirror createDefaultItem, so an item with no config object (a
// dashboard exported before these types existed) edits as a blank one instead
// of throwing.
const sliderConfig = computed<SliderWidgetConfig>(() =>
  props.item.sliderConfig || { mode: 'kv', min: 0, max: 100, step: 1, defaultValue: 0 })

const gaugeConfig = computed<MapItemGaugeConfig>(() =>
  props.item.gaugeConfig || { mode: 'kv', min: 0, max: 100 })

function emitUpdate(updates: Partial<MapMarkerItem>) {
  emit('update:item', { ...props.item, ...updates })
}

function updateField(field: keyof MapMarkerItem, value: any) {
  emitUpdate({ [field]: value })
}

function updateTextField(field: keyof MapItemTextConfig, value: any) {
  const newTextConfig = { ...textConfig.value, [field]: value }
  emitUpdate({ textConfig: newTextConfig })
}

function updateKvField(field: keyof MapItemKvConfig, value: any) {
  const newKvConfig = { ...kvConfig.value, [field]: value }
  emitUpdate({ kvConfig: newKvConfig })
}

function updateGaugeField(field: keyof MapItemGaugeConfig, value: any) {
  emitUpdate({ gaugeConfig: { ...gaugeConfig.value, [field]: value } })
}

function handleTypeChange(event: Event) {
  const newType = (event.target as HTMLSelectElement).value as MapItemType
  const newItem = createDefaultItem(newType)
  
  newItem.id = props.item.id
  newItem.label = props.item.label
  
  emit('update:item', newItem)
}
</script>

<style scoped>
.item-editor {
  background: rgba(0, 0, 0, 0.15);
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}

.item-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: rgba(0, 0, 0, 0.2);
  border-bottom: 1px solid var(--border);
}

.item-badge {
  font-size: 16px;
  line-height: 1;
}

.item-badge.switch { color: var(--color-warning); }
.item-badge.publish { color: var(--color-info); }
.item-badge.text { color: var(--color-success); }
.item-badge.slider { color: var(--color-primary); }
.item-badge.gauge { color: var(--color-info); }

.item-badge.kv { color: var(--color-secondary); }

.item-label {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btn-icon-small {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 4px;
  font-size: 14px;
  transition: all 0.2s;
}

.btn-icon-small.danger:hover {
  background: var(--color-error-bg);
  color: var(--color-error);
}

.item-body {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-row label {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
}

.form-input,
.form-textarea {
  width: 100%;
  padding: 8px 10px;
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text);
  font-family: var(--mono);
  font-size: 13px;
}

.form-input:focus,
.form-textarea:focus {
  outline: none;
  border-color: var(--color-accent);
}

.form-input.has-error,
.form-textarea.has-error {
  border-color: var(--color-error);
}

.form-textarea {
  resize: vertical;
  min-height: 60px;
}

.error-text {
  font-size: 11px;
  color: var(--color-error);
}

.help-text {
  font-size: 11px;
  color: var(--muted);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text);
}

.checkbox-label input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: var(--color-primary);
}
</style>
