import { JSONPath } from 'jsonpath-plus'

// No path (a widget bound to the whole payload) or '$' returns the data as is.
// Kept out of the subscription manager so pure chart utilities can use it.
export function extractJsonPath(data: any, path?: string): any {
  if (!path || path === '$') return data
  try { return JSONPath({ path, json: data, wrap: false }) } catch { return null }
}
