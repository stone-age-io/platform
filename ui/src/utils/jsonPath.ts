// ui/src/utils/jsonPath.ts
import { JSONPath } from 'jsonpath-plus'

// `path` is optional because every caller's is: WidgetListener.jsonPath is
// optional, and a widget bound to a whole payload has none. The body already
// handled undefined; only the signature disagreed.
//
// Lives here rather than inside the subscription manager so the pure chart
// utilities can use it without standing up a manager.
export function extractJsonPath(data: any, path?: string): any {
  if (!path || path === '$') return data
  try { return JSONPath({ path, json: data, wrap: false }) } catch { return null }
}
