<template>
  <div class="variable-bar">
    <!-- Case A: Has Variables -->
    <div v-if="hasVariables" class="variables-list">
      <div 
        v-for="variable in variables" 
        :key="variable.id" 
        class="variable-item"
      >
        <label class="variable-label">{{ variable.label }}</label>
        
        <!-- Select Input -->
        <select 
          v-if="variable.type === 'select'"
          :value="currentValues[variable.name]"
          @change="updateValue(variable.name, ($event.target as HTMLSelectElement).value)"
          class="variable-input"
        >
          <option v-for="opt in variable.options" :key="opt" :value="opt">
            {{ opt }}
          </option>
        </select>
        
        <!-- Text Input -->
        <input 
          v-else
          :value="currentValues[variable.name]"
          @input="handleTextInput(variable.name, ($event.target as HTMLInputElement).value)"
          class="variable-input text"
          type="text"
        />
      </div>
    </div>
    
    <!-- Case B: Empty State -->
    <div v-else class="empty-state">
      <span class="empty-text">No variables defined</span>
      <button class="add-variable-btn" @click="$emit('edit')">
        + Add Variable
      </button>
    </div>
    
    <!-- Actions (Always visible) -->
    <div class="bar-actions">
      <button v-if="hasVariables" class="action-btn" @click="$emit('edit')" title="Edit Variables">
        ✎
      </button>
      <button class="action-btn" @click="$emit('close')" title="Close">
        ✕
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useDashboardStore } from '@/stores/dashboard'

const dashboardStore = useDashboardStore()

defineEmits<{
  edit: []
  close: []
}>()

const variables = computed(() => dashboardStore.activeDashboard?.variables || [])
const hasVariables = computed(() => variables.value.length > 0)
const currentValues = computed(() => dashboardStore.currentVariableValues)

function updateValue(name: string, value: string) {
  dashboardStore.setVariableValue(name, value)
}

// Debounce text input to avoid rapid re-subscriptions
let debounceTimer: number | null = null
function handleTextInput(name: string, value: string) {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    updateValue(name, value)
  }, 500) as unknown as number
}
</script>

<style scoped>
.variable-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 24px;
  background: var(--panel);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  min-height: 56px;
}

.variables-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  flex: 1;
  align-items: center;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--muted);
  font-size: 13px;
  font-style: italic;
}

.variable-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.variable-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
  white-space: nowrap;
}

.variable-input {
  padding: 6px 10px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text);
  font-size: 13px;
  min-width: 120px;
  max-width: 200px;
}

.variable-input:focus {
  outline: none;
  border-color: var(--color-accent);
}

.bar-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}

.action-btn {
  background: transparent;
  border: none;
  color: var(--muted);
  cursor: pointer;
  padding: 6px;
  border-radius: 4px;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.action-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
}

.add-variable-btn {
  background: transparent;
  border: 1px dashed var(--border);
  color: var(--color-accent);
  font-size: 12px;
  padding: 6px 12px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s;
  font-style: normal;
}

.add-variable-btn:hover {
  border-color: var(--color-accent);
  background: var(--color-info-bg);
}

/* Mobile Optimization */
@media (max-width: 600px) {
  .variable-bar {
    padding: 12px;
    flex-direction: column;
    align-items: stretch;
  }

  .variables-list {
    display: grid;
    grid-template-columns: 1fr 1fr; /* 2 columns */
    gap: 12px;
    width: 100%;
  }

  .empty-state {
    justify-content: center;
    padding: 8px 0;
  }

  .variable-item {
    flex-direction: column;
    align-items: stretch;
    gap: 4px;
  }

  .variable-input {
    width: 100%;
    min-width: 0;
    max-width: none;
  }

  .bar-actions {
    position: absolute;
    top: 8px;
    right: 8px;
  }
}
</style>
