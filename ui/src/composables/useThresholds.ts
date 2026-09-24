import type { ThresholdRule } from '@/types/dashboard'

export function useThresholds() {

  /**
   * Evaluate a value against a list of rules
   * Returns the color string of the first matching rule, or undefined
   */
  function evaluateThresholds(value: any, rules: ThresholdRule[] | undefined): string | undefined {
    return matchThreshold(value, rules)?.color
  }

  /**
   * The first rule that matches, or undefined. The state timeline needs the
   * whole rule (colour AND label); everything else only wants the colour.
   */
  function matchThreshold(value: any, rules: ThresholdRule[] | undefined): ThresholdRule | undefined {
    if (!rules || rules.length === 0) return undefined
    if (value === null || value === undefined) return undefined
    return rules.find(rule => checkRule(value, rule))
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
      // Ordering is for numbers only. Comparing text made `> 40` true for
      // "idle", "fault" and every other word, since letters sort after digits.
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
