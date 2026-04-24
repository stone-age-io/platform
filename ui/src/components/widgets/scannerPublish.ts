import type { ScannerWidgetConfig } from '@/types/dashboard'

export interface ScanContext {
  value: string
  passed: boolean
  reason: string
  scanner: string
  scannerKind: 'user' | 'thing'
  record: Record<string, any> | null
}

export interface ScanPublish {
  subject: string
  payload: Record<string, any>
}

export function buildScanPublish(cfg: ScannerWidgetConfig, ctx: ScanContext): ScanPublish {
  const ts = new Date().toISOString()
  const template = cfg.publishSubjectTemplate || cfg.publishSubject || ''
  const subjectTokens: Record<string, string> = {
    value: ctx.value,
    scanner: ctx.scanner,
    scanner_kind: ctx.scannerKind,
    device_label: cfg.deviceLabel || '',
    purpose: cfg.scanPurpose || 'other',
    location: cfg.location || '',
    passed: ctx.passed ? 'true' : 'false',
    reason: ctx.reason,
    ts,
  }
  const subject = template.replace(/\{(\w+)\}/g, (_m, tok) => subjectTokens[tok] ?? '')

  const payload = {
    value: ctx.value,
    passed: ctx.passed,
    reason: ctx.reason,
    scanner: ctx.scanner,
    scanner_kind: ctx.scannerKind,
    device_label: cfg.deviceLabel || '',
    purpose: cfg.scanPurpose || 'other',
    location: cfg.location || '',
    record: ctx.record,
    ts,
  }

  return { subject, payload }
}
