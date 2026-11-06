import { format, formatDistanceToNow } from 'date-fns'
import { parseTimestamp } from './timestamp'

export function formatBytes(bytes: number, decimals = 2): string {
  if (!+bytes) return '0 Bytes'

  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB']

  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}

/** `formatStr` is a date-fns format string. */
export function formatDate(date: string | Date, formatStr = 'PPpp'): string {
  try {
    const d = typeof date === 'string' ? new Date(date) : date
    return format(d, formatStr)
  } catch (error) {
    return 'Invalid date'
  }
}

/** e.g. "2 hours ago", "in 5 days". */
export function formatRelativeTime(date: string | Date): string {
  try {
    const d = typeof date === 'string' ? new Date(date) : date
    return formatDistanceToNow(d, { addSuffix: true })
  } catch (error) {
    return 'Invalid date'
  }
}

/**
 * A table cell's value as epoch ms, or null when it is not a timestamp. Uses
 * the same parser messages are stamped with; never falls back to now.
 */
function cellTimestamp(val: unknown): number | null {
  if (val instanceof Date) return Number.isNaN(val.getTime()) ? null : val.getTime()
  return parseTimestamp(val)
}

export function formatColumnValue(value: any, columnFormat: string, formatOptions?: string): string {
  if (value === null || value === undefined) return '-'

  switch (columnFormat) {
    case 'number': {
      const num = Number(value)
      if (isNaN(num)) return String(value)
      return num.toLocaleString()
    }
    case 'relative-time': {
      const ms = cellTimestamp(value)
      return ms === null ? '-' : formatRelativeTime(new Date(ms))
    }
    case 'datetime': {
      const ms = cellTimestamp(value)
      return ms === null ? '-' : formatDate(new Date(ms), formatOptions || 'PPpp')
    }
    default:
      return String(value)
  }
}

/**
 * Short age for a widget header: "now", "12s ago", "4m ago", "3h ago", "2d ago".
 * A negative age (device clock ahead) reads as "now".
 */
export function formatAge(ms: number): string {
  const s = Math.floor(ms / 1000)
  if (s < 1) return 'now'
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

export function truncate(str: string, length: number): string {
  if (str.length <= length) return str
  return str.substring(0, length) + '...'
}

/** 'create_request' -> 'Create Request' */
export function formatConstant(str: string): string {
  return str
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ')
}
