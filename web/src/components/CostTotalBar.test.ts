import { describe, expect, it } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import CostTotalBar from './CostTotalBar.vue'

async function render(props: Record<string, unknown>, slot?: string) {
  const app = createSSRApp({ render: () => h(CostTotalBar, props as never, slot ? { default: () => h('p', { class: 'extra' }, slot) } : undefined) })
  return renderToString(app)
}

const base = { total: '12,40 €', perDistance: '0,155 €/km', totalLabel: 'Total', perDistanceLabel: 'Cost per km' }

describe('CostTotalBar', () => {
  it('shows the total and the per-distance figure with their labels', async () => {
    const html = await render(base)
    expect(html).toContain('12,40 €')
    expect(html).toContain('0,155 €/km')
    expect(html).toContain('Total')
    expect(html).toContain('Cost per km')
    expect(html).toContain('border-success-500/30')
  })

  it('brightens the border when strong', async () => {
    expect(await render({ ...base, strong: true })).toContain('border-success-500/40')
  })

  it('renders the slot below the figures', async () => {
    const html = await render(base, 'detail')
    expect(html.indexOf('0,155')).toBeLessThan(html.indexOf('class="extra"'))
  })
})
