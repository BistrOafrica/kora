import { describe, expect, it } from 'vitest'
import { applyComponentFilters } from './view-filters'

describe('applyComponentFilters', () => {
  it('filters a shared resource result without changing the source metadata', () => {
    const result = applyComponentFilters(
      {
        data: [
          { name: 'PROD-1', is_active: 1 },
          { name: 'PROD-2', is_active: 0 },
        ],
        meta: { doctype: 'Product', total: 2 },
      },
      [{ field: 'is_active', op: 'equals', value: true }],
    )

    expect(result.data).toEqual([{ name: 'PROD-1', is_active: 1 }])
    expect(result.meta).toEqual({ doctype: 'Product', total: 1 })
  })

  it('requires every component filter to match', () => {
    const result = applyComponentFilters(
      { data: [{ category: 'Dairy', stock_qty: 4 }, { category: 'Dairy', stock_qty: 0 }, { category: 'Bakery', stock_qty: 8 }] },
      [
        { field: 'category', op: 'equals', value: 'Dairy' },
        { field: 'stock_qty', op: 'gt', value: 0 },
      ],
    )

    expect(result.data).toEqual([{ category: 'Dairy', stock_qty: 4 }])
  })
})
