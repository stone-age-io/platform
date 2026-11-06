/**
 * The message worth showing for a failed PocketBase request.
 *
 * The top-level `err.message` is usually generic ("Failed to create record.");
 * the server's explanation is in the per-field errors under `response.data`.
 * Those are listed in server order as `field: message`.
 */
export function apiErrorMessage(err: unknown, fallback: string): string {
  const e = err as { message?: unknown; response?: { data?: unknown } } | null
  const fields = e?.response?.data
  if (fields && typeof fields === 'object' && !Array.isArray(fields)) {
    const parts = Object.entries(fields as Record<string, unknown>)
      .map(([name, detail]) => {
        const message = (detail as { message?: unknown } | null)?.message
        return typeof message === 'string' && message ? `${name}: ${message}` : ''
      })
      .filter(Boolean)
    if (parts.length) return parts.join(' ')
  }
  return typeof e?.message === 'string' && e.message ? e.message : fallback
}
