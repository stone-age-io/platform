import { fillTemplate } from './templates'

/**
 * Fill a dashboard's variables into a template.
 * e.g. "sensors.{device_id}.temp" -> "sensors.truck-1.temp".
 * An unknown variable keeps its {tag} so the user sees what is missing.
 */
export function resolveTemplate(template: string | undefined, variables: Record<string, string>): string {
  if (!template) return ''
  return fillTemplate(template, variables || {})
}
