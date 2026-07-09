import type { Constraint, Field } from '@/types/kora'

export interface FormSectionGroup {
  title: string
  fields: Field[]
}

export function buildFormSections(fields: Field[]): FormSectionGroup[] {
  const sections: FormSectionGroup[] = []
  let currentTitle = 'Details'
  let currentFields: Field[] = []

  const pushCurrent = () => {
    if (currentFields.length === 0) return
    sections.push({ title: currentTitle, fields: currentFields })
    currentFields = []
  }

  for (const field of fields) {
    if (field.fieldtype === 'Section Break' || field.fieldtype === 'Heading') {
      pushCurrent()
      currentTitle = field.label || 'Details'
      continue
    }
    if (field.fieldtype === 'Column Break') {
      continue
    }
    currentFields.push(field)
  }

  pushCurrent()
  return sections
}

export function isFieldVisible(field: Field, formData: Record<string, any>): boolean {
  if (field.hidden) return false
  return evaluateDependency(field.depends_on, formData, true)
}

export function isFieldRequired(field: Field, formData: Record<string, any>): boolean {
  if (field.reqd) return true
  return evaluateDependency(field.mandatory_depends_on, formData, false)
}

export function getFieldConstraintHint(
  field: Field,
  value: unknown,
  formData: Record<string, any>,
): string | null {
  if (value == null || value === '') return null
  if (!field.constraints?.length) return null

  for (const constraint of field.constraints) {
    if (!constraintApplies(constraint, formData)) continue
    const message = validateConstraint(constraint, value)
    if (message) return message
  }

  return null
}

function evaluateDependency(
  expression: string | undefined,
  formData: Record<string, any>,
  fallback: boolean,
): boolean {
  if (!expression) return fallback

  const tokens = expression
    .split(',')
    .map((token) => token.trim())
    .filter(Boolean)

  if (tokens.length === 0) return fallback

  return tokens.every((token) => evaluateToken(token, formData))
}

function evaluateToken(token: string, formData: Record<string, any>): boolean {
  let expr = token
  if (expr.startsWith('eval:')) expr = expr.slice(5).trim()
  if (expr.startsWith('doc.')) expr = expr.slice(4)

  if (expr.startsWith('!')) {
    return !truthy(formData[expr.slice(1)])
  }

  const comparators = ['==', '!=', '>=', '<=', '>', '<']
  for (const comparator of comparators) {
    const index = expr.indexOf(comparator)
    if (index === -1) continue
    const left = expr.slice(0, index).trim().replace(/^doc\./, '')
    const right = expr.slice(index + comparator.length).trim()
    return compare(formData[left], parseLiteral(right), comparator)
  }

  return truthy(formData[expr])
}

function compare(left: unknown, right: unknown, comparator: string): boolean {
  switch (comparator) {
    case '==':
      return left == right
    case '!=':
      return left != right
    case '>':
      return Number(left) > Number(right)
    case '<':
      return Number(left) < Number(right)
    case '>=':
      return Number(left) >= Number(right)
    case '<=':
      return Number(left) <= Number(right)
    default:
      return false
  }
}

function parseLiteral(raw: string): unknown {
  const trimmed = raw.replace(/^['"]|['"]$/g, '')
  if (trimmed === 'true') return true
  if (trimmed === 'false') return false
  if (trimmed === 'null') return null
  const numeric = Number(trimmed)
  return Number.isNaN(numeric) ? trimmed : numeric
}

function truthy(value: unknown): boolean {
  if (Array.isArray(value)) return value.length > 0
  return !!value
}

function constraintApplies(constraint: Constraint, formData: Record<string, any>): boolean {
  if (!constraint.condition) return true
  return evaluateToken(constraint.condition.trim(), formData)
}

function validateConstraint(constraint: Constraint, value: unknown): string | null {
  const type = constraint.type.toLowerCase()
  const text = String(value)
  const numberValue = Number(value)

  switch (type) {
    case 'min':
      if (!Number.isNaN(numberValue) && numberValue < Number(constraint.value)) return constraint.message
      return null
    case 'max':
      if (!Number.isNaN(numberValue) && numberValue > Number(constraint.value)) return constraint.message
      return null
    case 'min_length':
      if (text.length < Number(constraint.value)) return constraint.message
      return null
    case 'max_length':
      if (text.length > Number(constraint.value)) return constraint.message
      return null
    case 'regex':
    case 'pattern': {
      const pattern = constraint.pattern || String(constraint.value || '')
      if (!pattern) return null
      return new RegExp(pattern).test(text) ? null : constraint.message
    }
    case 'one_of': {
      const values = constraint.values || (Array.isArray(constraint.value) ? constraint.value : [constraint.value])
      return values.includes(value as never) ? null : constraint.message
    }
    case 'not_one_of': {
      const values = constraint.values || (Array.isArray(constraint.value) ? constraint.value : [constraint.value])
      return values.includes(value as never) ? constraint.message : null
    }
    default:
      return null
  }
}
