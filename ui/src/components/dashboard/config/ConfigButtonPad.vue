<template>
  <div class="config-buttonpad">
    <div class="size-row">
      <div class="form-group">
        <label for="pad-rows">Rows</label>
        <input
          id="pad-rows"
          v-model.number="form.buttonpadRows"
          type="number"
          class="form-input"
          :class="{ 'has-error': errors.buttonpadSize }"
          min="1"
          :max="BUTTON_PAD_MAX"
        />
      </div>
      <div class="form-group">
        <label for="pad-cols">Columns</label>
        <input
          id="pad-cols"
          v-model.number="form.buttonpadCols"
          type="number"
          class="form-input"
          :class="{ 'has-error': errors.buttonpadSize }"
          min="1"
          :max="BUTTON_PAD_MAX"
        />
      </div>
    </div>
    <div v-if="errors.buttonpadSize" class="error-text">{{ errors.buttonpadSize }}</div>

    <div class="form-group">
      <label>Buttons</label>
      <div class="pad-preview" :style="previewGrid">
        <button
          v-for="pos in positions"
          :key="`${pos.row}:${pos.col}`"
          type="button"
          class="pad-slot"
          :class="{ 'is-filled': !!pos.cell, 'is-selected': isSelected(pos) }"
          :style="pos.cell ? slotStyle(pos.cell) : undefined"
          :aria-label="pos.cell ? `Edit ${pos.cell.label || 'button'} (row ${pos.row + 1}, column ${pos.col + 1})` : `Add a button at row ${pos.row + 1}, column ${pos.col + 1}`"
          @click="selected = { row: pos.row, col: pos.col }"
        >
          {{ pos.cell ? (pos.cell.label || '?') : '+' }}
        </button>
      </div>
      <div class="help-text">Click a position to add or edit its button. An empty position stays a gap.</div>
      <div v-if="outsideCount" class="help-text warn">
        {{ outsideCount === 1 ? '1 button is' : `${outsideCount} buttons are` }} outside the grid and will be
        removed when you save.
      </div>
      <div v-if="errors.buttonpadCells" class="error-text">{{ errors.buttonpadCells }}</div>
    </div>

    <div v-if="selected && inGrid(selected, size.rows, size.cols)" class="cell-editor">
      <div class="cell-editor-head">
        <span>Row {{ selected.row + 1 }}, column {{ selected.col + 1 }}</span>
        <button v-if="selectedCell" type="button" class="btn btn-xs btn-ghost text-error" @click="removeSelected">
          Remove button
        </button>
      </div>

      <template v-if="selectedCell">
        <div class="form-group">
          <label for="pad-label">Label</label>
          <input id="pad-label" v-model="selectedCell.label" type="text" class="form-input" placeholder="↑" />
          <div class="help-text">Short text, or a glyph like ↑ ← → ↓ ▶ ⏸.</div>
        </div>

        <div class="form-group">
          <label for="pad-subject">Subject</label>
          <input id="pad-subject" v-model="selectedCell.subject" type="text" class="form-input" placeholder="cmd.thing.DEVICE_01.key" />
          <div class="help-text">Supports variables like <code v-pre>{device_id}</code>.</div>
        </div>

        <div class="form-group">
          <label for="pad-payload">Payload</label>
          <textarea id="pad-payload" v-model="selectedCell.payload" class="form-textarea" rows="3" placeholder='{"key": "UP"}' />
        </div>

        <div class="form-group">
          <label for="pad-color">Color</label>
          <select id="pad-color" v-model="selectedCell.color" class="form-input">
            <option :value="undefined">Default (Primary)</option>
            <option v-for="c in COLORS" :key="c.value" :value="c.value">{{ c.label }}</option>
            <option v-if="isCustomColor(selectedCell.color)" :value="selectedCell.color">Custom ({{ selectedCell.color }})</option>
          </select>
        </div>
      </template>

      <button v-else type="button" class="btn btn-sm btn-primary" @click="addAtSelected">Add a button here</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { WidgetFormState } from '@/types/config'
import { BUTTON_PAD_MAX, type ButtonPadCell } from '@/types/dashboard'
import { padSize, inGrid } from '@/utils/buttonPad'

const props = defineProps<{
  form: WidgetFormState
  errors: Record<string, string>
}>()

// The same choices as the Button widget. A colour typed into imported JSON
// (a hex value) is kept and shown as Custom.
const COLORS = [
  { value: 'var(--color-secondary)', label: 'Secondary' },
  { value: 'var(--color-accent)', label: 'Accent' },
  { value: 'var(--color-success)', label: 'Success' },
  { value: 'var(--color-warning)', label: 'Warning' },
  { value: 'var(--color-error)', label: 'Danger' },
  { value: 'var(--color-info)', label: 'Info' },
  { value: 'var(--panel)', label: 'Neutral' },
]

function isCustomColor(color: string | undefined): boolean {
  return !!color && !COLORS.some(c => c.value === color)
}

const selected = ref<{ row: number; col: number } | null>(null)

// Clamped, so the preview still draws while a number is half typed.
const size = computed(() => padSize({ rows: props.form.buttonpadRows, cols: props.form.buttonpadCols }))

function cellAt(row: number, col: number): ButtonPadCell | undefined {
  return props.form.buttonpadCells.find(c => c.row === row && c.col === col)
}

const positions = computed(() => {
  const out: { row: number; col: number; cell?: ButtonPadCell }[] = []
  for (let row = 0; row < size.value.rows; row++) {
    for (let col = 0; col < size.value.cols; col++) {
      out.push({ row, col, cell: cellAt(row, col) })
    }
  }
  return out
})

const previewGrid = computed(() => ({
  gridTemplateColumns: `repeat(${size.value.cols}, minmax(0, 1fr))`,
}))

const outsideCount = computed(() =>
  props.form.buttonpadCells.filter(c => !inGrid(c, size.value.rows, size.value.cols)).length
)

const selectedCell = computed(() => selected.value ? cellAt(selected.value.row, selected.value.col) : undefined)

function isSelected(pos: { row: number; col: number }): boolean {
  return selected.value?.row === pos.row && selected.value?.col === pos.col
}

function slotStyle(cell: ButtonPadCell) {
  const bg = cell.color || 'var(--color-primary)'
  return { backgroundColor: bg, color: bg === 'var(--panel)' ? 'var(--text)' : 'white' }
}

// A new button copies the subject and colour of the last one added: the keys
// of one pad usually share a subject and differ only in payload.
function addAtSelected() {
  if (!selected.value) return
  const last = props.form.buttonpadCells[props.form.buttonpadCells.length - 1]
  props.form.buttonpadCells.push({
    row: selected.value.row,
    col: selected.value.col,
    label: '',
    subject: last?.subject ?? '',
    payload: '{}',
    color: last?.color,
  })
}

function removeSelected() {
  const cell = selectedCell.value
  if (!cell) return
  props.form.buttonpadCells.splice(props.form.buttonpadCells.indexOf(cell), 1)
}
</script>

<style scoped>
.size-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.pad-preview {
  display: grid;
  gap: 6px;
  max-width: 360px;
}

.pad-slot {
  min-width: 0;
  height: 40px;
  border-radius: 6px;
  border: 1px dashed var(--border);
  background: transparent;
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 0 4px;
  cursor: pointer;
}

.pad-slot.is-filled {
  border-style: solid;
  border-color: transparent;
}

.pad-slot.is-selected {
  outline: 2px solid var(--color-accent);
  outline-offset: 2px;
}

.help-text.warn {
  color: var(--color-warning);
}

.cell-editor {
  margin-top: 4px;
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
}

.cell-editor-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
  font-size: 13px;
}
</style>
