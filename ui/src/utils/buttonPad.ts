// The button pad's grid arithmetic, kept pure so it can be tested. The widget,
// the phone layout (DashboardGrid) and the config form all read through here.
//
// The config arrives from user-editable JSON, so every function copes with a
// missing or nonsensical one rather than trusting the type.

import { BUTTON_PAD_MAX, type ButtonPadCell, type ButtonPadWidgetConfig } from '@/types/dashboard'

/** Rows and columns, clamped to 1..BUTTON_PAD_MAX. */
export function padSize(config: Partial<ButtonPadWidgetConfig> | undefined): { rows: number; cols: number } {
  const clamp = (n: unknown) => {
    const v = Math.floor(Number(n))
    return Number.isFinite(v) ? Math.min(BUTTON_PAD_MAX, Math.max(1, v)) : 1
  }
  return { rows: clamp(config?.rows), cols: clamp(config?.cols) }
}

export function inGrid(cell: Pick<ButtonPadCell, 'row' | 'col'>, rows: number, cols: number): boolean {
  return Number.isInteger(cell.row) && Number.isInteger(cell.col) &&
    cell.row >= 0 && cell.row < rows && cell.col >= 0 && cell.col < cols
}

/** The cells to draw: inside the grid, one per position (the first wins). */
export function padCells(config: Partial<ButtonPadWidgetConfig> | undefined): ButtonPadCell[] {
  const { rows, cols } = padSize(config)
  const seen = new Set<string>()
  const out: ButtonPadCell[] = []
  for (const cell of Array.isArray(config?.cells) ? config.cells : []) {
    if (!cell || !inGrid(cell, rows, cols)) continue
    const key = `${cell.row}:${cell.col}`
    if (seen.has(key)) continue
    seen.add(key)
    out.push(cell)
  }
  return out
}

// A key is 56px tall on a phone: comfortably over the 44px a finger needs.
const PHONE_KEY_PX = 56
const GAP_PX = 8
const PADDING_PX = 16
const HEADER_PX = 32

/**
 * Height on a phone, from the row count. A widget body is a size container,
 * so this is the widget's whole height there; the phone layout has no other
 * way to know a 4-row pad needs more room than a 1-row one.
 */
export function padPhoneHeight(config: Partial<ButtonPadWidgetConfig> | undefined): number {
  const { rows } = padSize(config)
  return rows * PHONE_KEY_PX + (rows - 1) * GAP_PX + PADDING_PX + HEADER_PX
}
