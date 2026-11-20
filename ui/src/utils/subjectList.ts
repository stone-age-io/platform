// The NATS role and user forms edit subject lists as one comma-separated input.
// Both forms must split and join them the same way, or they save different
// permissions for the same text.

export function formatForInput(val: unknown): string {
  if (!val) return ''
  if (Array.isArray(val)) return val.join(', ')
  return String(val)
}

export function formatForApi(val: string): string[] {
  if (!val) return []
  return val.split(',')
    .map(s => s.trim())
    .filter(s => s !== '')
}
