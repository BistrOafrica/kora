import { renderToStaticMarkup } from 'react-dom/server'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DashboardGrid from './DashboardGrid'
import MetricCard from './MetricCard'
import PublicForm from './PublicForm'

const useQueryMock = vi.hoisted(() => vi.fn())
const apiPostMock = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-query', () => ({
  useQuery: useQueryMock,
}))

vi.mock('@/lib/api/client', () => ({
  api: {
    post: apiPostMock,
  },
}))

vi.mock('@/components/ui/skeleton', () => ({
  Skeleton: (props: { className?: string }) => <div {...props} />,
}))

vi.mock('@/components/ui/button', () => ({
  Button: (props: Record<string, any>) => <button {...props} />,
}))

vi.mock('@/components/ui/input', () => ({
  Input: (props: Record<string, any>) => <input {...props} />,
}))

vi.mock('@/components/ui/label', () => ({
  Label: (props: Record<string, any>) => <label {...props} />,
}))

vi.mock('@/components/ui/textarea', () => ({
  Textarea: (props: Record<string, any>) => <textarea {...props} />,
}))

describe('accessibility and mobile contract checks', () => {
  beforeEach(() => {
    useQueryMock.mockReset()
    apiPostMock.mockReset()
  })

  it('renders dashboard grids with responsive structure', () => {
    const markup = renderToStaticMarkup(
      <DashboardGrid
        config={{ id: 'grid', type: 'dashboard_grid', region: 'main', position: 0, label: 'Overview' } as any}
        onAction={vi.fn()}
      />,
    )

    expect(markup).toContain('grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4')
    expect(markup).toContain('Overview')
  })

  it('renders accessible metric error states', () => {
    useQueryMock.mockReturnValue({ data: null, isLoading: false, isError: true })

    const markup = renderToStaticMarkup(
      <MetricCard
        config={{ id: 'metric', type: 'metric_card', region: 'main', position: 0, label: 'Sales', bindings: {} } as any}
        onAction={vi.fn()}
      />,
    )

    expect(markup).toContain('role="status"')
    expect(markup).toContain('Metric needs a data source')
  })

  it('renders public forms with labels and submit controls', () => {
    const markup = renderToStaticMarkup(
      <PublicForm
        config={{
          id: 'form',
          type: 'public_form',
          region: 'main',
          position: 0,
          label: 'Contact us',
          bindings: undefined,
        } as any}
        onAction={vi.fn()}
      />,
    )

    expect(markup).toContain('Contact us')
    expect(markup).toContain('Submit')
    expect(markup).toContain('aria-label="name"')
    expect(markup).toContain('aria-label="email"')
  })
})
