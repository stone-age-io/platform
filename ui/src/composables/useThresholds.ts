import type { ThresholdRule } from '@/types/dashboard'

export function useThresholds() {

  /**
   * Evaluate a value against a list of rules
   * Returns the color string of the first matching rule, or undefined
   */
  function evaluateThresholds(value: any, rules: ThresholdRule[] | undefined): string | undefined {
    if (!rules || rules.length === 0) return undefined
    if (value === null || value === undefined) return undefined

    for (const rule of rules) {
      if (checkRule(value, rule)) {
        return rule.color
      }
    }

    return undefined
  }

  function checkRule(value: any, rule: ThresholdRule): boolean {
    // 1. Handle Numeric Comparison
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

    // 2. Handle String/Boolean Comparison
    const valStr = String(value).trim()
    const ruleValStr = String(rule.value).trim()

    switch (rule.operator) {
      case '>': return valStr > ruleValStr
      case '>=': return valStr >= ruleValStr
      case '<': return valStr < ruleValStr
      case '<=': return valStr <= ruleValStr
      case '==': return valStr === ruleValStr || valStr.toLowerCase() === ruleValStr.toLowerCase()
      case '!=': return valStr !== ruleValStr && valStr.toLowerCase() !== ruleValStr.toLowerCase()
      default: return false
    }
  }

  return { evaluateThresholds }
}
