import { useMemo } from 'react'
import type { ViewRule } from '@/lib/api/views'

export interface RuleResult {
  hidden: boolean
  disabled: boolean
  readonly: boolean
}

/**
 * Evaluates visibility/disabled/readonly rules for a view component.
 * Phase 1 supports structured conditions (field + op + value).
 */
export function useComponentRules(
  rules: ViewRule[] | undefined,
  data: any,
): RuleResult {
  return useMemo(() => {
    const result: RuleResult = { hidden: false, disabled: false, readonly: false }

    if (!rules || rules.length === 0) return result

    for (const rule of rules) {
      const { field, op, value } = rule.condition
      const fieldValue = data?.data?.[field] ?? data?.[field]

      let matches = false
      switch (op) {
        case 'equals':
          matches = fieldValue == value  // loose equality for cross-type matching
          break
        case 'not_equals':
          matches = fieldValue != value
          break
        case 'in':
          matches = Array.isArray(value) && value.includes(fieldValue)
          break
        case 'is_set':
          matches = fieldValue != null && fieldValue !== ''
          break
        case 'is_not_set':
          matches = fieldValue == null || fieldValue === ''
          break
        case 'gt':
          matches = Number(fieldValue) > Number(value)
          break
        case 'gte':
          matches = Number(fieldValue) >= Number(value)
          break
        case 'lt':
          matches = Number(fieldValue) < Number(value)
          break
        case 'lte':
          matches = Number(fieldValue) <= Number(value)
          break
        default:
          matches = false
      }

      if (matches) {
        switch (rule.target) {
          case 'visible':
            result.hidden = false
            break
          case 'hidden':
            result.hidden = true
            break
          case 'disabled':
            result.disabled = true
            break
          case 'readonly':
            result.readonly = true
            break
        }
      }
    }

    return result
  }, [rules, data])
}
