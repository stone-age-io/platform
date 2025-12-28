import { format, formatDistanceToNow } from 'date-fns'

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
