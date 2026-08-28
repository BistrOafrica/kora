import { describe, expect, it } from 'vitest'
import { buildDefaultFormData } from './form-runtime'
import type { Field } from '@/types/kora'

const field = (fieldname: string, fieldtype: Field['fieldtype'], defaultValue: string): Field => ({
  fieldname,
  fieldtype,
  label: fieldname,
  options: '',
  reqd: false,
  unique: false,
  default: defaultValue,
  hidden: false,
  read_only: false,
  bold: false,
  in_list_view: false,
  in_standard_filter: false,
  search_index: false,
  description: '',
  depends_on: '',
  mandatory_depends_on: '',
  constraints: null,
  renamed_from: '',
})

describe('buildDefaultFormData', () => {
  it('coerces schema defaults into form values', () => {
    expect(buildDefaultFormData([
      field('status', 'Select', 'Draft'),
      field('tax_rate', 'Percent', '16'),
      field('active', 'Check', '1'),
    ])).toEqual({ status: 'Draft', tax_rate: 16, active: true })
  })

  it('supports a date default without hard-coding a date', () => {
    const value = buildDefaultFormData([field('posting_date', 'Date', 'today')]).posting_date
    expect(value).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})
