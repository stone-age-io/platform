/**
 * e.g. "sensors.{{device_id}}.temp" -> "sensors.truck-1.temp".
 * An unknown variable keeps its {{tag}} so the user sees what is missing.
 */
export function resolveTemplate(template: string | undefined, variables: Record<string, string>): string {
  if (!template) return ''
  if (!variables || Object.keys(variables).length === 0) return template

  return template.replace(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g, (match, varName) => {
    return variables[varName] !== undefined ? variables[varName] : match
  })
}

export function hasVariables(template: string): boolean {
  return /\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/.test(template)
}
