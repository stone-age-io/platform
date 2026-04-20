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

        <div class="form-group">
          <label>Validation Rules</label>
          <div class="help-text rules-help">
            All rules must pass for GO. Empty list → any found record is GO.
            Use dot-paths for nested fields (e.g. <code>metadata.level</code>).
          </div>

          <div v-if="form.scannerRules.length > 0" class="rules-list">
            <div
              v-for="(rule, idx) in form.scannerRules"
              :key="idx"
              class="rule-row"
            >
              <div class="rule-row-main">
                <input
                  v-model="rule.field"
                  type="text"
                  class="form-input font-mono rule-field"
                  placeholder="field.path"
                />
                <select v-model="rule.op" class="form-input rule-op">
                  <option v-for="op in RULE_OPS" :key="op.value" :value="op.value">
                    {{ op.label }}
                  </option>
                </select>
                <input
                  v-if="opUsesValue(rule.op)"
                  :value="valueToInput(rule)"
                  type="text"
                  class="form-input font-mono rule-value"
                  :placeholder="rule.op === 'in' || rule.op === 'not_in' ? 'a, b, c' : 'value'"
                  @input="onValueInput(rule, ($event.target as HTMLInputElement).value)"
                />
                <button
                  type="button"
                  class="btn btn-xs btn-ghost rule-remove"
                  @click="removeRule(idx)"
                  aria-label="Remove rule"
                >×</button>
              </div>
              <input
                v-model="rule.reason"
                type="text"
                class="form-input rule-reason"
                placeholder="NO-GO label (optional)"
              />
              <span v-if="errors[`scannerRules.${idx}.field`]" class="error-text">
                {{ errors[`scannerRules.${idx}.field`] }}
              </span>
              <span v-if="errors[`scannerRules.${idx}.value`]" class="error-text">
                {{ errors[`scannerRules.${idx}.value`] }}
              </span>
            </div>
          </div>

          <button type="button" class="btn btn-xs btn-ghost rule-add" @click="addRule">
            + Add Rule
          </button>
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
            placeholder='{ "value": "{value}", "passed": {passed}, "reason": "{reason}", "ts": "{ts}" }'
          ></textarea>
          <span v-if="errors.scannerPublishPayloadTemplate" class="error-text">{{ errors.scannerPublishPayloadTemplate }}</span>
          <div class="help-text">
            JSON template. <strong>Strings</strong> (wrap in quotes):
            <code>"{value}"</code> <code>"{scanner}"</code> <code>"{scanner_kind}"</code>
            <code>"{device_label}"</code> <code>"{purpose}"</code> <code>"{location}"</code>
            <code>"{reason}"</code> <code>"{ts}"</code>.
            <br />
            <strong>Booleans</strong> (use bare): <code>{found}</code> (record located in lookup),
            <code>{passed}</code> (GO/NO-GO — found and all rules passed).
            <br />
            <strong>Objects</strong> (use bare): <code>{record}</code> (full matched record),
            <code>{metadata}</code> (just <code>record.metadata</code>).
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
import type { ScannerRule, ScannerRuleOp } from '@/types/dashboard'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

const RULE_OPS: { value: ScannerRuleOp; label: string }[] = [
  { value: 'truthy',     label: 'is truthy' },
  { value: 'falsy',      label: 'is falsy' },
  { value: 'equals',     label: 'equals' },
  { value: 'not_equals', label: 'does not equal' },
  { value: 'in',         label: 'is one of' },
  { value: 'not_in',     label: 'is not one of' },
  { value: 'future',     label: 'is in the future' },
  { value: 'past',       label: 'is in the past' },
  { value: 'exists',     label: 'exists' },
  { value: 'missing',    label: 'is missing' },
]

function opUsesValue(op: ScannerRuleOp): boolean {
  return op === 'equals' || op === 'not_equals' || op === 'in' || op === 'not_in'
}

function valueToInput(rule: ScannerRule): string {
  const v = rule.value
  if (v === undefined || v === null) return ''
  if (Array.isArray(v)) return v.join(', ')
  return String(v)
}

function onValueInput(rule: ScannerRule, raw: string) {
  if (rule.op === 'in' || rule.op === 'not_in') {
    rule.value = raw.split(',').map(s => s.trim()).filter(s => s.length > 0)
  } else {
    rule.value = raw
  }
}

function addRule() {
  props.form.scannerRules.push({ field: '', op: 'falsy', reason: '' })
}

function removeRule(idx: number) {
  props.form.scannerRules.splice(idx, 1)
}
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

.rules-help {
  margin-bottom: 6px;
}

.rules-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 6px;
}

.rule-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px;
  border: 1px solid oklch(var(--b3));
  border-radius: 6px;
  background: oklch(var(--b2) / 0.3);
}

.rule-row-main {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) minmax(0, 1.2fr) auto;
  gap: 4px;
  align-items: center;
}

.rule-row-main:has(.rule-value:not([hidden])) {
  /* three inputs + remove button */
}

.rule-row-main:not(:has(.rule-value)) {
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) auto;
}

.rule-field,
.rule-op,
.rule-value,
.rule-reason {
  min-width: 0;
}

.rule-remove {
  font-size: 16px;
  line-height: 1;
  padding: 0 6px;
}

.rule-add {
  align-self: flex-start;
}
</style>
