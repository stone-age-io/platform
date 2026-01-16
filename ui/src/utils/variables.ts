/**
 * Variable Resolution Utilities
 * 
 * Grug say: Find {{curly_braces}} and replace with value.
 * Simple string replacement.
 */

/**
 * Resolve a string template with variable values
 * e.g. "sensors.{{device_id}}.temp" -> "sensors.truck-1.temp"
 */
export function resolveTemplate(template: string | undefined, variables: Record<string, string>): string {
  if (!template) return ''
  if (!variables || Object.keys(variables).length === 0) return template

  // Regex to find {{ variable_name }}
  // Allowed chars: alphanumeric, underscore, dot (for JSON paths), dash (for slugs)
  return template.replace(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g, (match, varName) => {
    // If variable exists, use it. If not, keep original {{tag}} so user sees error/missing.
    return variables[varName] !== undefined ? variables[varName] : match
  })
}

/**
 * Check if a string contains variable placeholders
 */
export function hasVariables(template: string): boolean {
  return /\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/.test(template)
}
