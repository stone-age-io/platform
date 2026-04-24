import type { ScannerRule } from '@/types/dashboard'

function getByPath(obj: any, path: string): any {
  if (!obj || !path) return undefined
  return path.split('.').reduce((acc, k) => (acc == null ? acc : acc[k]), obj)
}

function evalRule(rec: any, r: ScannerRule): boolean {
  const v = getByPath(rec, r.field)
  switch (r.op) {
    case 'truthy':     return !!v
    case 'falsy':      return !v
    case 'equals':     return v === r.value
    case 'not_equals': return v !== r.value
    case 'in':         return Array.isArray(r.value) && r.value.includes(v)
    case 'not_in':     return Array.isArray(r.value) && !r.value.includes(v)
    case 'exists':     return v !== undefined && v !== null
    case 'missing':    return v === undefined || v === null
    case 'future': {
      // Absent value ≡ no expiry → pass. Mirrors legacy behavior for expires_at.
      if (v == null) return true
      const t = Date.parse(String(v))
      return !Number.isNaN(t) && t > Date.now()
    }
    case 'past': {
      if (v == null) return false
      const t = Date.parse(String(v))
      return !Number.isNaN(t) && t < Date.now()
    }
  }
}

export interface RuleResult {
  ok: boolean
  reason: string
}

export function evaluateRules(rec: any, rules: ScannerRule[] | undefined): RuleResult {
  if (!rules || rules.length === 0) return { ok: true, reason: 'ok' }
  for (const r of rules) {
    if (!evalRule(rec, r)) return { ok: false, reason: r.reason || `${r.field} ${r.op}` }
  }
  return { ok: true, reason: 'ok' }
}
