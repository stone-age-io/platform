<template>
  <div class="config-pocketbase">
    <div class="form-group">
      <label>Collection</label>
      <select v-model="form.pbCollection" class="form-input" :disabled="loading">
        <option value="" disabled>Select Collection</option>
        <option v-for="col in collections" :key="col.id" :value="col.name">
          {{ col.name }} ({{ col.type }})
        </option>
      </select>
    </div>

    <div class="form-group">
      <label>Filter (optional)</label>
      <input 
        v-model="form.pbFilter" 
        type="text" 
        class="form-input font-mono"
        placeholder='status = "active"'
      />
      <div class="help-text">
        PocketBase filter syntax. Supports variables like <code v-pre>{{device_id}}</code>.
      </div>
    </div>

    <div class="form-group">
      <label>Sort (optional)</label>
      <input 
        v-model="form.pbSort" 
        type="text" 
        class="form-input font-mono"
        placeholder="-created"
      />
    </div>

    <div class="form-group">
      <label>Fields (optional)</label>
      <input 
        v-model="form.pbFields" 
        type="text" 
        class="form-input font-mono"
        placeholder="id,name,status"
      />
      <div class="help-text">Comma separated. Leave empty for all.</div>
    </div>

    <div class="form-group">
      <label>Limit</label>
      <input 
        v-model.number="form.pbLimit" 
        type="number" 
        class="form-input"
        min="1"
        max="500"
      />
    </div>

    <div class="form-group">
      <label>Auto-Refresh (seconds)</label>
      <input 
        v-model.number="form.pbRefreshInterval" 
        type="number" 
        class="form-input"
        min="0"
        placeholder="0 (Disabled)"
      />
      <div class="help-text">Set to 0 to disable auto-refresh.</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { pb } from '@/utils/pb'
import type { WidgetFormState } from '@/types/config'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const collections = ref<any[]>([])
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    const result = await pb.collections.getFullList({ sort: 'name' })
    collections.value = result
  } catch (e) {
    console.error('Failed to load collections', e)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.font-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
