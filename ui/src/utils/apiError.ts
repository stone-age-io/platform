/**
 * The message worth showing for a failed PocketBase request.
 *
 * The SDK's `err.message` is the response's top-level message, which for a
 * refused write is usually generic ("Failed to create record."). When the server
 * explains itself, it does so in the per-field errors under `response.data`, and
 * a form that toasts only `err.message` drops the explanation on the floor. A NATS
 * limit of 0 is the case that prompted this: the server says what 0 would do and
 * what to write instead, and the form said "Failed to save role".
 *
 * Field errors are listed in the order the server sent them, as `field: message`.
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
