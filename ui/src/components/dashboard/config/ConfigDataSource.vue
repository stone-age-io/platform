<template>
  <div class="config-data-source">
    
    <!-- MULTI-SUBJECT MODE -->
    <div v-if="allowMultiple" class="form-group">
      <label>NATS Subjects</label>
      <div class="tag-input-container" :class="{ 'has-error': errors.subjects }">
        <div class="tags-area">
          <span v-for="(sub, index) in localSubjects" :key="index" class="subject-tag">
            {{ sub }}
            <button class="remove-tag" @click="removeSubject(index)">×</button>
          </span>
          <input 
            ref="tagInput"
            v-model="newSubject"
            type="text" 
            class="tag-input-field"
            placeholder="Add subject (Enter)..."
            @keydown.enter.prevent="addSubject"
            @keydown.backspace="handleBackspace"
            @blur="addSubject"
          />
        </div>
      </div>
      <div v-if="errors.subjects" class="error-text">
        {{ errors.subjects }}
      </div>
      <div v-else class="help-text">
        Type subject and press Enter. Supports wildcards (e.g. <code>logs.></code>).
      </div>
    </div>

    <!-- SINGLE SUBJECT MODE -->
    <div v-else class="form-group">
      <label>NATS Subject</label>
      <input 
        v-model="form.subject" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.subject }"
        placeholder="sensors.temperature"
      />
      <div v-if="errors.subject" class="error-text">
        {{ errors.subject }}
      </div>
      <div v-else class="help-text">
        NATS subject pattern to subscribe to
      </div>
    </div>
    
    <!-- JetStream Toggle -->
    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" v-model="form.useJetStream" />
        <span>Use JetStream (History)</span>
      </label>
    </div>

    <!-- JetStream Options -->
    <div v-if="form.useJetStream" class="form-group jetstream-options">
      <label>Deliver Policy</label>
      <select v-model="form.deliverPolicy" class="form-input">
        <option value="all">All (Entire History)</option>
        <option value="last">Last (Last Message)</option>
        <option value="last_per_subject">Last Per Subject (Current State)</option>
        <option value="new">New (From Now)</option>
        <option value="by_start_time">By Time Window</option>
      </select>
      
      <!-- Time Window Input -->
      <div v-if="form.deliverPolicy === 'by_start_time'" class="mt-2">
        <label class="sub-label">Time Window</label>
        <input 
          v-model="form.jetstreamTimeWindow" 
          type="text" 
          class="form-input"
          placeholder="10m"
        />
        <div class="help-text">
          Examples: <code>10m</code> (minutes), <code>1h</code> (hour), <code>24h</code>.
        </div>
      </div>
      
      <div class="help-text" v-else>
        Controls how much historical data to load.
      </div>
    </div>
    
    <div class="form-group">
      <label>JSONPath (optional)</label>
      <input 
        v-model="form.jsonPath" 
        type="text" 
        class="form-input"
        :class="{ 'has-error': errors.jsonPath }"
        placeholder="$.value or $.sensors[0].temp"
      />
      <div v-if="errors.jsonPath" class="error-text">
        {{ errors.jsonPath }}
      </div>
      <div v-else class="help-text">
        Extract specific data from messages. Leave empty to show full message.
      </div>
    </div>
    
    <div class="form-group">
      <label>Buffer Size</label>
      <input 
        v-model.number="form.bufferSize" 
        type="number" 
        class="form-input"
        :class="{ 'has-error': errors.bufferSize }"
        min="10"
        max="1000"
      />
      <div v-if="errors.bufferSize" class="error-text">
        {{ errors.bufferSize }}
      </div>
      <div v-else class="help-text">
        Number of messages to keep in history (10-1000)
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { WidgetFormState } from '@/types/config'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
  allowMultiple?: boolean
}>()

// --- Multi-Subject Logic ---
const newSubject = ref('')
const localSubjects = ref<string[]>([])

// Sync local array with form state
watch(() => props.form.subjects, (val) => {
  if (val) localSubjects.value = [...val]
  else localSubjects.value = []
}, { immediate: true })

// Sync array back to form state
function updateForm() {
  props.form.subjects = [...localSubjects.value]
  // Update legacy field for compatibility
  if (localSubjects.value.length > 0) {
    props.form.subject = localSubjects.value[0]
  } else {
    props.form.subject = ''
  }
}

function addSubject() {
  const sub = newSubject.value.trim()
  if (!sub) return
  
  if (!localSubjects.value.includes(sub)) {
    localSubjects.value.push(sub)
    updateForm()
  }
  newSubject.value = ''
}

function removeSubject(index: number) {
  localSubjects.value.splice(index, 1)
  updateForm()
}

function handleBackspace() {
  if (newSubject.value === '' && localSubjects.value.length > 0) {
    localSubjects.value.pop()
    updateForm()
  }
}
</script>

<style scoped>
.jetstream-options {
  padding: 12px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 6px;
  border: 1px solid var(--border);
  margin-top: -10px;
  margin-bottom: 20px;
  animation: slideDown 0.2s ease-out;
}

.sub-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
  margin-bottom: 4px;
  margin-top: 8px;
}

.mt-2 {
  margin-top: 8px;
}

/* Tag Input Styles */
.tag-input-container {
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px;
  min-height: 42px;
  cursor: text;
  display: flex;
  flex-wrap: wrap;
}

.tag-input-container:focus-within {
  border-color: var(--color-accent);
}

.tag-input-container.has-error {
  border-color: var(--color-error);
}

.tags-area {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  width: 100%;
}

.subject-tag {
  display: inline-flex;
  align-items: center;
  background: var(--color-info-bg);
  color: var(--color-info);
  border: 1px solid var(--color-info-border);
  border-radius: 4px;
  padding: 2px 6px;
  font-family: var(--mono);
  font-size: 12px;
}

.remove-tag {
  background: none;
  border: none;
  color: inherit;
  margin-left: 6px;
  cursor: pointer;
  font-weight: bold;
  padding: 0 2px;
  opacity: 0.7;
}

.remove-tag:hover {
  opacity: 1;
}

.tag-input-field {
  flex: 1;
  min-width: 120px;
  background: transparent;
  border: none;
  color: var(--text);
  font-family: var(--mono);
  font-size: 13px;
  padding: 4px;
  outline: none;
}

@keyframes slideDown {
  from { opacity: 0; transform: translateY(-5px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
