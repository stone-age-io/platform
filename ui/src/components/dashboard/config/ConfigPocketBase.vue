<!-- ui/src/components/dashboard/config/ConfigPocketBase.vue -->
<template>
  <div class="config-pocketbase">
    <div class="form-group">
      <label>Collection</label>
      <!-- Grug say: Dropdown hard. Input simple. Datalist best of both. -->
      <input 
        v-model="form.pbCollection" 
        list="pb-collections"
        type="text" 
        class="form-input"
        placeholder="e.g. audit_logs"
      />
      <datalist id="pb-collections">
        <option value="audit_logs">Audit Logs</option>
        <option value="things">Things</option>
        <option value="locations">Locations</option>
        <option value="users">Users</option>
        <option value="organizations">Organizations</option>
        <option value="invites">Invites</option>
        <option value="memberships">Memberships</option>
      </datalist>
      <div class="help-text">
        Enter the collection name (ID or Name).
      </div>
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
import type { WidgetFormState } from '@/types/config'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

// Grug say: No API call needed. User knows collection name or can guess.
</script>

<style scoped>
.font-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
