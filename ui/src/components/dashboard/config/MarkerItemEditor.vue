<template>
  <div class="item-editor">
    <div class="item-header">
      <span class="item-badge" :class="item.type">
        {{ getItemIcon(item.type) }}
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
      <!-- Item Type -->
      <div class="form-row">
        <label>Type</label>
        <select :value="item.type" @change="handleTypeChange" class="form-input">
          <option value="publish">Publish (Button)</option>
          <option value="switch">Switch (Toggle)</option>
          <option value="text">Text (Display)</option>
          <option value="kv">KV (Display)</option>
        </select>
      </div>
      
      <!-- Label -->
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
      
      <!-- Publish Item Fields -->
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
      
      <!-- Switch Item Fields -->
      <template v-if="item.type === 'switch'">
        <div class="form-row">
          <label>Mode</label>
          <select 
            :value="switchConfig.mode" 
            @change="updateSwitchField('mode', ($event.target as HTMLSelectElement).value)"
            class="form-input"
          >
            <option value="kv">KV (Stateful)</option>
            <option value="core">CORE (Pub/Sub)</option>
          </select>
        </div>
        
        <!-- KV Mode -->
        <template v-if="switchConfig.mode === 'kv'">
          <div class="form-row">
            <label>KV Bucket</label>
            <input 
              :value="switchConfig.kvBucket"
              @input="updateSwitchField('kvBucket', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              :class="{ 'has-error': errors?.kvBucket }"
              placeholder="device-states"
            />
            <div v-if="errors?.kvBucket" class="error-text">{{ errors.kvBucket }}</div>
          </div>
          
          <div class="form-row">
            <label>KV Key</label>
            <input 
              :value="switchConfig.kvKey"
              @input="updateSwitchField('kvKey', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              :class="{ 'has-error': errors?.kvKey }"
              placeholder="device.switch"
            />
            <div v-if="errors?.kvKey" class="error-text">{{ errors.kvKey }}</div>
          </div>
        </template>
        
        <!-- Core Mode -->
        <template v-if="switchConfig.mode === 'core'">
          <div class="form-row">
            <label>Publish Subject</label>
            <input 
              :value="switchConfig.publishSubject"
              @input="updateSwitchField('publishSubject', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              :class="{ 'has-error': errors?.publishSubject }"
              placeholder="device.control"
            />
            <div v-if="errors?.publishSubject" class="error-text">{{ errors.publishSubject }}</div>
          </div>
          
          <div class="form-row">
            <label>State Subject (optional)</label>
            <input 
              :value="switchConfig.stateSubject"
              @input="updateSwitchField('stateSubject', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder="device.state"
            />
            <div class="help-text">Subscribe for state updates. Defaults to publish subject.</div>
          </div>
        </template>
        
        <!-- Payload fields (both modes) -->
        <div class="form-row-pair">
          <div class="form-row">
            <label>ON Payload</label>
            <input 
              :value="onPayloadStr"
              @change="updatePayload('on', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder='{"state":"on"}'
            />
          </div>
          <div class="form-row">
            <label>OFF Payload</label>
            <input 
              :value="offPayloadStr"
              @change="updatePayload('off', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder='{"state":"off"}'
            />
          </div>
        </div>
        
        <!-- Labels -->
        <div class="form-row-pair">
          <div class="form-row">
            <label>ON Label</label>
            <input 
              :value="switchLabels.on"
              @input="updateSwitchLabel('on', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder="ON"
            />
          </div>
          <div class="form-row">
            <label>OFF Label</label>
            <input 
              :value="switchLabels.off"
              @input="updateSwitchLabel('off', ($event.target as HTMLInputElement).value)"
              type="text" 
              class="form-input"
              placeholder="OFF"
            />
          </div>
        </div>
        
        <!-- Confirm toggle -->
        <div class="form-row">
          <label class="checkbox-label">
            <input 
              type="checkbox" 
              :checked="switchConfig.confirmOnChange"
              @change="updateSwitchField('confirmOnChange', ($event.target as HTMLInputElement).checked)"
            />
            <span>Require confirmation before toggle</span>
          </label>
        </div>
      </template>

      <!-- Text Item Fields -->
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

        <!-- JetStream Toggle -->
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

        <!-- JetStream Options -->
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

      <!-- KV Item Fields -->
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
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { MapMarkerItem, MapItemSwitchConfig, MapItemTextConfig, MapItemKvConfig, MapItemType } from '@/types/dashboard'
import { createDefaultItem } from '@/types/dashboard'

const props = defineProps<{
  item: MapMarkerItem
  errors?: Record<string, string>
}>()

const emit = defineEmits<{
  remove: []
  'update:item': [item: MapMarkerItem]
}>()

function getItemIcon(type: MapItemType) {
  switch (type) {
    case 'publish': return '📤'
    case 'switch': return '🔄'
    case 'text': return '📝'
    case 'kv': return '🗄️'
    default: return '?'
  }
}

const switchConfig = computed<MapItemSwitchConfig>(() => {
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

const switchLabels = computed(() => {
  return switchConfig.value.labels || { on: 'ON', off: 'OFF' }
})

const onPayloadStr = computed(() => {
  const val = switchConfig.value.onPayload
  // If it's an object, stringify it
  if (typeof val === 'object' && val !== null) {
    return JSON.stringify(val)
  }
  // If it's already a string (or number/boolean), return as string
  // This prevents double-encoding strings like "{" -> "\"{""
  return String(val ?? '')
})

const offPayloadStr = computed(() => {
  const val = switchConfig.value.offPayload
  if (typeof val === 'object' && val !== null) {
    return JSON.stringify(val)
  }
  return String(val ?? '')
})

function emitUpdate(updates: Partial<MapMarkerItem>) {
  emit('update:item', { ...props.item, ...updates })
}

function updateField(field: keyof MapMarkerItem, value: any) {
  emitUpdate({ [field]: value })
}

function updateSwitchField(field: keyof MapItemSwitchConfig, value: any) {
  const newSwitchConfig = { ...switchConfig.value, [field]: value }
  emitUpdate({ switchConfig: newSwitchConfig })
}

function updateTextField(field: keyof MapItemTextConfig, value: any) {
  const newTextConfig = { ...textConfig.value, [field]: value }
  emitUpdate({ textConfig: newTextConfig })
}

function updateKvField(field: keyof MapItemKvConfig, value: any) {
  const newKvConfig = { ...kvConfig.value, [field]: value }
  emitUpdate({ kvConfig: newKvConfig })
}

function updateSwitchLabel(which: 'on' | 'off', value: string) {
  const newLabels = { ...switchLabels.value, [which]: value }
  const newSwitchConfig = { ...switchConfig.value, labels: newLabels }
  emitUpdate({ switchConfig: newSwitchConfig })
}

function updatePayload(which: 'on' | 'off', value: string) {
  let parsed: any = value
  
  // Try to parse. If it looks like JSON (starts with { or [), try to parse.
  // If valid, save as object.
  // If invalid, SAVE AS STRING.
  const trimmed = value.trim()
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      parsed = JSON.parse(value)
    } catch {
      // Keep as string if parse fails
      parsed = value
    }
  } else {
    // Try parsing numbers/booleans
    try {
      parsed = JSON.parse(value)
    } catch {
      parsed = value
    }
  }
  
  const field = which === 'on' ? 'onPayload' : 'offPayload'
  const newSwitchConfig = { ...switchConfig.value, [field]: parsed }
  emitUpdate({ switchConfig: newSwitchConfig })
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

.form-row-pair {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
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

@media (max-width: 500px) {
  .form-row-pair {
    grid-template-columns: 1fr;
  }
}
</style>
