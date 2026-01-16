<template>
  <div class="threshold-editor">
    <div class="threshold-list">
      <div v-if="modelValue.length === 0" class="empty-rules">
        No conditional formatting rules.
      </div>
      
      <div v-for="(rule, index) in modelValue" :key="rule.id" class="threshold-row">
        <span class="row-label">If val</span>
        
        <!-- Operator -->
        <select v-model="rule.operator" class="input-select operator">
          <option value=">">&gt;</option>
          <option value=">=">&ge;</option>
          <option value="<">&lt;</option>
          <option value="<=">&le;</option>
          <option value="==">==</option>
          <option value="!=">!=</option>
        </select>
        
        <!-- Value -->
        <input 
          v-model="rule.value" 
          type="text" 
          class="input-text value" 
          placeholder="Value" 
        />
        
        <span class="row-label">then</span>
        
        <!-- Color Picker -->
        <select 
          v-model="rule.color" 
          class="input-select color" 
          :style="{ color: rule.color }"
        >
          <option value="var(--color-primary)" style="color: var(--color-primary)">Primary</option>
          <option value="var(--color-secondary)" style="color: var(--color-secondary)">Secondary</option>
          <option value="var(--color-accent)" style="color: var(--color-accent)">Accent</option>
          <option value="var(--color-success)" style="color: var(--color-success)">Success</option>
          <option value="var(--color-warning)" style="color: var(--color-warning)">Warning</option>
          <option value="var(--color-error)" style="color: var(--color-error)">Error</option>
          <option value="var(--color-info)" style="color: var(--color-info)">Info</option>
          <option value="var(--text)" style="color: var(--text)">Default Text</option>
          <option value="var(--muted)" style="color: var(--muted)">Muted</option>
        </select>
        
        <!-- Delete -->
        <button class="btn-icon danger" @click="removeRule(index)" title="Remove Rule">
          ✕
        </button>
      </div>
    </div>
    
    <button class="btn-add" @click="addRule">
      + Add Rule
    </button>
  </div>
</template>

<script setup lang="ts">
import type { ThresholdRule } from '@/types/dashboard'

const props = defineProps<{
  modelValue: ThresholdRule[]
}>()

const emit = defineEmits<{
  'update:modelValue': [rules: ThresholdRule[]]
}>()

function addRule() {
  const newRule: ThresholdRule = {
    id: Date.now().toString(),
    operator: '>',
    value: '',
    color: 'var(--color-error)' 
  }
  emit('update:modelValue', [...props.modelValue, newRule])
}

function removeRule(index: number) {
  const newRules = [...props.modelValue]
  newRules.splice(index, 1)
  emit('update:modelValue', newRules)
}
</script>

<style scoped>
.threshold-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(0,0,0,0.1);
  padding: 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
}

.empty-rules {
  color: var(--muted);
  font-size: 13px;
  font-style: italic;
  text-align: center;
  padding: 8px;
}

.threshold-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.row-label {
  font-size: 12px;
  color: var(--muted);
  white-space: nowrap;
}

.input-select, .input-text {
  background: var(--input-bg);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 6px;
  border-radius: 4px;
  font-family: var(--mono);
  font-size: 13px;
}

.input-select:focus, .input-text:focus {
  outline: none;
  border-color: var(--color-accent);
}

.operator { width: 50px; }
.value { flex: 1; min-width: 60px; }
.color { flex: 1; min-width: 100px; font-weight: 600; }

.input-select option {
  background: var(--input-bg);
}

.btn-icon {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-icon:hover {
  background: rgba(255,255,255,0.1);
  color: var(--text);
}

.btn-icon.danger:hover {
  color: var(--color-error);
  background: var(--color-error-bg);
}

.btn-add {
  align-self: flex-start;
  background: transparent;
  border: 1px dashed var(--border);
  color: var(--color-accent);
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
}

.btn-add:hover {
  background: var(--color-info-bg);
  border-color: var(--color-accent);
}
</style>
