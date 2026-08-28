import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('renders required-field completion with an accessible percentage', () => {
    const markup = renderToStaticMarkup(<ProgressBar filled={3} total={5} />)

    expect(markup).toContain('3/5 required fields')
    expect(markup).toContain('60%')
    expect(markup).toContain('width:60%')
  })

  it('renders nothing when there are no required fields', () => {
    expect(renderToStaticMarkup(<ProgressBar filled={0} total={0} />)).toBe('')
  })
})
