import type { ViewComponent } from './api/views'

export function applyComponentFilters(data: any, filters?: ViewComponent['filters']) {
  if (!data?.data || !Array.isArray(data.data) || !filters?.length) return data

  const rows = data.data.filter((row: Record<string, any>) =>
    filters.every((filter) => matchesComponentFilter(row[filter.field], filter.op, filter.value)),
  )

  return {
    ...data,
    data: rows,
    meta: data.meta ? { ...data.meta, total: rows.length } : data.meta,
  }
}

function sameFilterValue(actual: any, expected: any): boolean {
  if (typeof expected === 'boolean') return Boolean(actual) === expected || Number(actual) === (expected ? 1 : 0)
  return actual === expected || String(actual) === String(expected)
}

function matchesComponentFilter(actual: any, op: string, expected: any): boolean {
  switch (op) {
    case 'equals': return sameFilterValue(actual, expected)
    case 'not_equals': return !sameFilterValue(actual, expected)
    case 'contains': return String(actual ?? '').toLowerCase().includes(String(expected ?? '').toLowerCase())
    case 'starts_with': return String(actual ?? '').toLowerCase().startsWith(String(expected ?? '').toLowerCase())
    case 'ends_with': return String(actual ?? '').toLowerCase().endsWith(String(expected ?? '').toLowerCase())
    case 'gt': return actual > expected
    case 'gte': return actual >= expected
    case 'lt': return actual < expected
    case 'lte': return actual <= expected
    case 'is_set': return actual !== null && actual !== undefined && actual !== ''
    case 'is_not_set': return actual === null || actual === undefined || actual === ''
    case 'not_in': return !Array.isArray(expected) || !expected.some((value) => sameFilterValue(actual, value))
    case 'in': return Array.isArray(expected) && expected.some((value) => sameFilterValue(actual, value))
    default: return true
  }
}
