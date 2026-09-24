import { format, formatDistanceToNow } from 'date-fns'
import { parseTimestamp } from './timestamp'

/**
 * Format bytes to human-readable string
 * @param bytes - Number of bytes
 * @param decimals - Number of decimal places (default: 2)
 */
export function formatBytes(bytes: number, decimals = 2): string {
  if (!+bytes) return '0 Bytes'

  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB']

  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`
}

/**
 * Format date to local string
 * @param date - Date string or Date object
 * @param formatStr - date-fns format string (default: 'PPpp')
 */
export function formatDate(date: string | Date, formatStr = 'PPpp'): string {
  try {
    const d = typeof date === 'string' ? new Date(date) : date
    return format(d, formatStr)
  } catch (error) {
    return 'Invalid date'
  }
}

/**
 * Format date to relative time (e.g., "2 hours ago", "in 5 days")
 * Uses date-fns to correctly handle both past and future dates.
 */
export function formatRelativeTime(date: string | Date): string {
  try {
    const d = typeof date === 'string' ? new Date(date) : date
    // addSuffix: true adds "ago" for past dates or "in" for future dates
    return formatDistanceToNow(d, { addSuffix: true })
  } catch (error) {
    return 'Invalid date'
  }
}

/**
 * A table cell's value as epoch ms, or null when it is not a timestamp.
 *
 * This used to have its own parser, which disagreed with the one messages are
 * stamped with: a nanosecond epoch (Go's UnixNano) threw a RangeError inside
 * the table's computed and blanked the whole table, microseconds rendered tens
 * of thousands of years out, and 0, "", a seconds value sent as a string, or
 * junk all showed as "less than a minute ago" because they fell back to now.
 * A cell now shows "-" for anything that is not a time.
 */
function cellTimestamp(val: unknown): number | null {
  if (val instanceof Date) return Number.isNaN(val.getTime()) ? null : val.getTime()
  return parseTimestamp(val)
}

/**
 * Format a KV table column value based on its configured format.
 */
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
 * Truncate string with ellipsis
 */
export function truncate(str: string, length: number): string {
  if (str.length <= length) return str
  return str.substring(0, length) + '...'
}

/**
 * Format enum/constant to readable string
 * Example: 'create_request' -> 'Create Request'
 */
export function formatConstant(str: string): string {
  return str
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ')
}
