<!-- ui/src/components/dashboard/config/ConfigScanner.vue -->
<template>
  <div class="config-scanner">
    <!-- KV Lookup Section -->
    <div class="config-section">
      <label class="toggle-label">
        <input type="checkbox" v-model="form.scannerKvEnabled" class="checkbox checkbox-sm" />
        <span class="section-title">KV Lookup</span>
      </label>

      <template v-if="form.scannerKvEnabled">
        <div class="form-group">
          <label>Bucket</label>
          <input
            v-model="form.scannerKvBucket"
            type="text"
            class="form-input"
            placeholder="e.g. badges"
          />
          <span v-if="errors.scannerKvBucket" class="error-text">{{ errors.scannerKvBucket }}</span>
        </div>

        <div class="form-group">
          <label>Key Template</label>
          <input
            v-model="form.scannerKvKeyTemplate"
            type="text"
            class="form-input font-mono"
            placeholder="{value}"
          />
          <div class="help-text">
            Use <code>{value}</code> as a placeholder for the scanned QR content.
          </div>
        </div>
      </template>
    </div>

    <!-- PocketBase Lookup Section -->
    <div class="config-section">
      <label class="toggle-label">
        <input type="checkbox" v-model="form.scannerPbEnabled" class="checkbox checkbox-sm" />
        <span class="section-title">PocketBase Lookup</span>
      </label>

      <template v-if="form.scannerPbEnabled">
        <div class="form-group">
          <label>Collection</label>
          <input
            v-model="form.scannerPbCollection"
            list="scanner-pb-collections"
            type="text"
            class="form-input"
            placeholder="e.g. nats_users"
          />
          <datalist id="scanner-pb-collections">
            <option value="nats_users">NATS Users</option>
            <option value="things">Things</option>
            <option value="locations">Locations</option>
            <option value="users">Users</option>
            <option value="memberships">Memberships</option>
          </datalist>
          <span v-if="errors.scannerPbCollection" class="error-text">{{ errors.scannerPbCollection }}</span>
        </div>

        <div class="form-group">
          <label>Filter</label>
          <input
            v-model="form.scannerPbFilter"
            type="text"
            class="form-input font-mono"
            placeholder='public_key = "{value}"'
          />
          <div class="help-text">
            PocketBase filter syntax. Use <code>{value}</code> for the scanned content.
          </div>
        </div>

        <div class="form-group">
          <label>Fields (optional)</label>
          <input
            v-model="form.scannerPbFields"
            type="text"
            class="form-input font-mono"
            placeholder="id,name,email,public_key"
          />
          <div class="help-text">Comma separated. Leave empty for all.</div>
        </div>
      </template>
    </div>

    <!-- Publish Audit Section -->
    <div class="config-section">
      <label class="toggle-label">
        <input type="checkbox" v-model="form.scannerPublishEnabled" class="checkbox checkbox-sm" />
        <span class="section-title">Publish Scan Event</span>
      </label>

      <template v-if="form.scannerPublishEnabled">
        <div class="form-group">
          <label>Subject</label>
          <input
            v-model="form.scannerPublishSubject"
            type="text"
            class="form-input font-mono"
            placeholder="scans.badge"
          />
          <div class="help-text">
            Each scan publishes a JSON event with the scanned value and timestamp.
          </div>
          <span v-if="errors.scannerPublishSubject" class="error-text">{{ errors.scannerPublishSubject }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { WidgetFormState } from '@/types/config'

defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()
</script>

<style scoped>
.config-scanner {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.config-section {
  border: 1px solid oklch(var(--b3));
  border-radius: 8px;
  padding: 12px;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  margin-bottom: 4px;
}

.section-title {
  font-size: 13px;
  font-weight: 600;
  color: oklch(var(--bc));
}

.font-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.error-text {
  color: oklch(var(--er));
  font-size: 12px;
  margin-top: 2px;
}
</style>
