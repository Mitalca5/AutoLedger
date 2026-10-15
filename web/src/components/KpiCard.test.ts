import { afterEach, describe, expect, it } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { i18n, setLocale } from '@/i18n'
import KpiCard from './KpiCard.vue'

const render = (props: Record<string, unknown>, slot?: string) =>
  renderToString(createSSRApp({ render: () => h(KpiCard as any, props, slot ? { default: () => h('p', slot) } : undefined) }).use(i18n))

afterEach(() => setLocale('fr'))

describe('KpiCard', () => {
  it('shows the label, the value with its unit and the period', async () => {
    const html = await render({ label: 'Economic cost', value: '0,142', unit: '/km', period: 'last 12 months' })
    expect(html).toContain('Economic cost')
    expect(html).toContain('0,142')
    expect(html).toContain('/km')
    expect(html).toContain('last 12 months')
  })

  it('reads an unknown value as a dash, without its unit', async () => {
    const html = await render({ label: 'Cost per km', value: null, unit: '/km' })
    expect(html).toContain('—')
    expect(html).not.toContain('/km<')
    expect(html).toContain('text-slate-400')
  })

  it('adds the definition tip when a glossary term is given', async () => {
    setLocale('en')
    const html = await render({ label: 'Cash paid', value: '1 200 €', term: 'cashPaid' })
    expect(html).toContain('role="note"')
  })

  it('renders the delta with its tone and the secondary lines from the slot', async () => {
    const html = await render({ label: 'Month', value: '90 €', delta: { text: '+12 %', tone: 'bad' } }, 'of which tolls')
    expect(html).toContain('+12 %')
    expect(html).toContain('text-danger-400')
    expect(html).toContain('of which tolls')
  })

  it('opens the detail through a button when there is no target', async () => {
    const html = await render({ label: 'Energy', value: '300 €', actionLabel: 'Open panel' })
    expect(html).toContain('<button')
    expect(html).toContain('Open panel')
  })
})
