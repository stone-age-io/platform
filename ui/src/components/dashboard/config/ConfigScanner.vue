<!-- ui/src/components/dashboard/config/ConfigScanner.vue -->
<template>
  <div class="config-scanner">
    <!-- Scan Context Section -->
    <div class="config-section">
      <span class="section-title">Scan Context</span>

      <div class="form-group">
        <label>Purpose</label>
        <select v-model="form.scannerPurpose" class="form-input">
          <option value="verify">Verify</option>
          <option value="muster">Muster</option>
          <option value="other">Other</option>
        </select>
        <div class="help-text">
          Feeds <code>{purpose}</code> in subject and payload templates.
        </div>
      </div>

      <div class="form-group">
        <label>Device Label</label>
        <input
          v-model="form.scannerDeviceLabel"
          type="text"
          class="form-input"
          placeholder="e.g. North-Gate-Station"
        />
        <div class="help-text">Feeds <code>{device_label}</code>.</div>
      </div>

      <div class="form-group">
        <label>Location (optional)</label>
        <input
          v-model="form.scannerLocation"
          type="text"
          class="form-input"
          placeholder="location id or free text"
        />
        <div class="help-text">Feeds <code>{location}</code>.</div>
      </div>
    </div>

    <!-- KV Lookup (Badges) Section -->
    <div class="config-section">
      <label class="toggle-label">
        <input type="checkbox" v-model="form.scannerKvEnabled" class="checkbox checkbox-sm" />
        <span class="section-title">KV Lookup (badges)</span>
      </label>

      <template v-if="form.scannerKvEnabled">
        <div class="form-group">
          <label>Bucket</label>
          <input
            v-model="form.scannerKvBucket"
            type="text"
            class="form-input"
            placeholder="badges"
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
            <code>{value}</code> is the scanned string (nkey for badges).
          </div>
        </div>
      </template>
    </div>

    <!-- PocketBase Lookup (Optional Fallback for Non-Badge Scans) -->
    <div class="config-section">
      <label class="toggle-label">
        <input type="checkbox" v-model="form.scannerPbEnabled" class="checkbox checkbox-sm" />
        <span class="section-title">PocketBase Lookup (optional, for asset scans)</span>
      </label>

      <template v-if="form.scannerPbEnabled">
        <div class="form-group">
          <label>Collection</label>
          <input
            v-model="form.scannerPbCollection"
            list="scanner-pb-collections"
            type="text"
            class="form-input"
            placeholder="e.g. things"
          />
          <datalist id="scanner-pb-collections">
            <option value="things">Things</option>
            <option value="locations">Locations</option>
            <option value="nats_users">NATS Users</option>
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
            placeholder='id = "{value}"'
          />
          <div class="help-text">
            PB filter; use <code>{value}</code> for the scanned content.
          </div>
        </div>

        <div class="form-group">
          <label>Fields (optional)</label>
          <input
            v-model="form.scannerPbFields"
            type="text"
            class="form-input font-mono"
            placeholder="id,name"
          />
          <div class="help-text">Comma separated. Leave empty for all.</div>
        </div>
      </template>
    </div>

    <!-- Publish Section -->
    <div class="config-section">
      <label class="toggle-label">
        <input type="checkbox" v-model="form.scannerPublishEnabled" class="checkbox checkbox-sm" />
        <span class="section-title">Publish Scan Event</span>
      </label>

      <template v-if="form.scannerPublishEnabled">
        <div class="form-group">
          <label>Subject Template</label>
          <input
            v-model="form.scannerPublishSubjectTemplate"
            type="text"
            class="form-input font-mono"
            placeholder="scans.{purpose}.{scanner}"
          />
          <span v-if="errors.scannerPublishSubjectTemplate" class="error-text">{{ errors.scannerPublishSubjectTemplate }}</span>
        </div>

        <div class="form-group">
          <label>Payload Template (JSON)</label>
          <textarea
            v-model="form.scannerPublishPayloadTemplate"
            rows="5"
            class="form-input font-mono"
            placeholder='{ "value": "{value}", "found": {found}, "ts": "{ts}" }'
          ></textarea>
          <span v-if="errors.scannerPublishPayloadTemplate" class="error-text">{{ errors.scannerPublishPayloadTemplate }}</span>
          <div class="help-text">
            JSON template. Tokens: <code>{value}</code> <code>{scanner}</code> <code>{scanner_kind}</code>
            <code>{device_label}</code> <code>{purpose}</code> <code>{location}</code>
            <code>{found}</code> <code>{reason}</code> <code>{ts}</code> <code>{metadata}</code>.
            Quote tokens for strings (<code>"{ts}"</code>); leave bare for typed values (<code>{found}</code>, <code>{metadata}</code>).
          </div>
        </div>
      </template>
    </div>

    <!-- Behavior Section -->
    <div class="config-section">
      <span class="section-title">Behavior</span>

      <div class="form-group">
        <label>Dedup Window (ms)</label>
        <input
          v-model.number="form.scannerDedupWindowMs"
          type="number"
          min="0"
          step="500"
          class="form-input"
        />
        <span v-if="errors.scannerDedupWindowMs" class="error-text">{{ errors.scannerDedupWindowMs }}</span>
        <div class="help-text">Suppress repeat scans of the same value within this window. 0 to disable.</div>
      </div>

      <div class="form-group">
        <label>Lookup Timeout (ms)</label>
        <input
          v-model.number="form.scannerLookupTimeoutMs"
          type="number"
          min="100"
          step="500"
          class="form-input"
        />
        <span v-if="errors.scannerLookupTimeoutMs" class="error-text">{{ errors.scannerLookupTimeoutMs }}</span>
      </div>

      <div class="form-group">
        <label class="toggle-label">
          <input type="checkbox" v-model="form.scannerAllowManualEntry" class="checkbox checkbox-sm" />
          <span>Allow manual entry (fallback when camera unavailable)</span>
        </label>
      </div>
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
