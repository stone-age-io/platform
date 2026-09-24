import { describe, it, expect } from 'vitest'
import { useThresholds } from './useThresholds'
import type { ThresholdRule } from '@/types/dashboard'

// matchThreshold was split out of evaluateThresholds so the state timeline can
// read a rule's label as well as its colour. Text, stat and kv still call
// evaluateThresholds and must see exactly what they saw before.

const rule = (id: string, operator: ThresholdRule['operator'], value: string, color: string, label?: string): ThresholdRule =>
  ({ id, operator, value, color, label })

const RULES = [
  rule('a', '==', 'true', 'green', 'running'),
  rule('b', '==', 'false', 'grey', 'stopped'),
  rule('c', '>', '40', 'red'),
]


describe('useThresholds', () => {
  const { matchThreshold, evaluateThresholds } = useThresholds()

  it('returns the whole first matching rule, label included', () => {
    expect(matchThreshold(true, RULES)?.label).toBe('running')
    expect(matchThreshold('FALSE', RULES)?.id).toBe('b')
    expect(matchThreshold(45, RULES)?.id).toBe('c')
  })

  it('returns undefined when nothing matches, or there is nothing to match', () => {
    expect(matchThreshold('idle', RULES)).toBeUndefined()
    expect(matchThreshold(null, RULES)).toBeUndefined()
    expect(matchThreshold(true, [])).toBeUndefined()
    expect(matchThreshold(true, undefined)).toBeUndefined()
  })

  it('first match wins', () => {
    const rules = [rule('x', '>', '10', 'amber'), rule('y', '>', '40', 'red')]
    expect(evaluateThresholds(50, rules)).toBe('amber')
  })

  it('evaluateThresholds is still the colour of that rule', () => {
    expect(evaluateThresholds(true, RULES)).toBe('green')
    expect(evaluateThresholds(45, RULES)).toBe('red')
    expect(evaluateThresholds('idle', RULES)).toBeUndefined()
  })
})

// > >= < <= used to fall back to comparing TEXT, and letters sort after digits,
// so `> 40` coloured "idle", "fault" and every other word red.
describe('ordering operators', () => {
  const { matchThreshold } = useThresholds()
  const hot = [rule('h', '>', '40', 'red')]

  it('never match a value that is not a number', () => {
    for (const v of ['idle', 'fault', 'Z', 'true']) expect(matchThreshold(v, hot)).toBeUndefined()
    expect(matchThreshold('abc', [rule('l', '<', 'b', 'red')])).toBeUndefined()
  })

  it('still compare numbers, including numeric strings', () => {
    expect(matchThreshold(41, hot)?.id).toBe('h')
    expect(matchThreshold('41.5', hot)?.id).toBe('h')
    expect(matchThreshold(40, hot)).toBeUndefined()
  })
})
