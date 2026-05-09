// Shared helpers for column-based table widgets (KvTableWidget, StreamTableWidget).
// Pure functions only — no Vue, no DOM dependencies beyond the CSV download helper.

import type { ConditionalRule, TableColumn } from '@/types/dashboard'

export const NULL_DISPLAY = '—'

export function isEmpty(v: unknown): boolean {
  return v === null || v === undefined || v === ''
}

export function displayValue(v: unknown): string {
  return isEmpty(v) ? NULL_DISPLAY : String(v)
}

export function compareMixed(a: unknown, b: unknown): number {
  if (a == null && b == null) return 0
  if (a == null) return -1
  if (b == null) return 1
  if (typeof a === 'number' && typeof b === 'number') return a - b
  if (a instanceof Date && b instanceof Date) return a.getTime() - b.getTime()
  return String(a).localeCompare(String(b))
}

export function evalRule(raw: unknown, rule: ConditionalRule): boolean {
  if (raw === null || raw === undefined) return false
  if (rule.op === 'eq') return String(raw) === String(rule.value)
  if (rule.op === 'contains') {
    return String(raw).toLowerCase().includes(String(rule.value).toLowerCase())
  }
  const a = Number(raw)
  const b = Number(rule.value)
  if (!Number.isFinite(a) || !Number.isFinite(b)) return false
  switch (rule.op) {
    case 'gt': return a > b
    case 'lt': return a < b
    case 'gte': return a >= b
    case 'lte': return a <= b
  }
  return false
}

export function cellClass(raw: unknown, col: TableColumn): string {
  if (!col.rules || col.rules.length === 0) return ''
  for (const rule of col.rules) {
    if (evalRule(raw, rule)) return `cell-style-${rule.style}`
  }
  return ''
}

export function csvField(v: unknown): string {
  if (isEmpty(v)) return ''
  const s = String(v)
  if (/[",\n\r]/.test(s)) return '"' + s.replace(/"/g, '""') + '"'
  return s
}

export function exportCsv(opts: {
  columns: TableColumn[]
  rows: Array<Record<string, unknown>>
  filename: string
}): void {
  const { columns, rows, filename } = opts
  if (columns.length === 0) return
  const header = columns.map(c => csvField(c.label || c.id)).join(',')
  const lines = rows.map(row => columns.map(c => csvField(row[c.id])).join(','))
  const csv = [header, ...lines].join('\r\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  const safe = (filename || 'table').replace(/[^a-z0-9._-]+/gi, '_')
  a.download = `${safe}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
