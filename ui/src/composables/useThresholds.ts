import type { ThresholdRule } from '@/types/dashboard'

export function useThresholds() {

  /** The colour of the first matching rule, or undefined. */
  function evaluateThresholds(value: any, rules: ThresholdRule[] | undefined): string | undefined {
    return matchThreshold(value, rules)?.color
  }

  /** The first matching rule, for callers that need its label too (the state timeline). */
  function matchThreshold(value: any, rules: ThresholdRule[] | undefined): ThresholdRule | undefined {
    if (!rules || rules.length === 0) return undefined
    if (value === null || value === undefined) return undefined
    return rules.find(rule => checkRule(value, rule))
  }

  function checkRule(value: any, rule: ThresholdRule): boolean {
    const valNum = Number(value)
    const ruleValNum = Number(rule.value)
    
    const isNumeric = !isNaN(valNum) && !isNaN(ruleValNum) && String(rule.value).trim() !== ''

    if (isNumeric) {
      switch (rule.operator) {
        case '>': return valNum > ruleValNum
        case '>=': return valNum >= ruleValNum
        case '<': return valNum < ruleValNum
        case '<=': return valNum <= ruleValNum
        case '==': return valNum === ruleValNum
        case '!=': return valNum !== ruleValNum
      }
    }

    // Text and boolean values.
    const valStr = String(value).trim()
    const ruleValStr = String(rule.value).trim()

    switch (rule.operator) {
      // Ordering is for numbers only: as text, letters sort after digits, so
      // `> 40` would match every word.
      case '>':
      case '>=':
      case '<':
      case '<=':
        return false
      case '==': return valStr === ruleValStr || valStr.toLowerCase() === ruleValStr.toLowerCase()
      case '!=': return valStr !== ruleValStr && valStr.toLowerCase() !== ruleValStr.toLowerCase()
      default: return false
    }
  }

  return { evaluateThresholds, matchThreshold }
}
