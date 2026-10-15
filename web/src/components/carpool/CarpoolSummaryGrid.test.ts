import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createSSRApp } from 'vue'
import { createPinia } from 'pinia'
import { renderToString } from '@vue/server-renderer'
import { i18n, setLocale } from '@/i18n'
import CarpoolSummaryGrid from './CarpoolSummaryGrid.vue'

const summary = {
  total_trips: 12, total_passengers: 20, total_distance_km: 480, total_revenue: 50, total_passengers_share: 80,
  total_real_cost: 100, total_driver_share: 20, net_cost_per_km: 0.05, coverage_rate_pct: 64.1,
}
const render = (value: Record<string, unknown>) =>
  renderToString(createSSRApp(CarpoolSummaryGrid, { summary: { ...summary, ...value } }).use(createPinia()).use(i18n))

beforeEach(() => {
  vi.stubGlobal('localStorage', { getItem: () => null, setItem: () => undefined })
})

afterEach(() => {
  vi.unstubAllGlobals()
  setLocale('fr')
})

describe('carpool summary grid', () => {
  it('shows the recovery rate in the account language', async () => {
    setLocale('en')
    expect(await render({})).toContain('64.1')
  })

  it('reads an unknown recovery rate as a dash, not as 0 %', async () => {
    const html = await render({ coverage_rate_pct: null })
    expect(html).toContain('—')
    expect(html).not.toContain('0 %')
  })

  it('says how much is missing while the revenue is under the fair share', async () => {
    setLocale('en')
    const html = await render({})
    expect(html).toContain('text-warning-400')
    expect(await render({ total_revenue: 80 })).toContain('text-success-400')
  })

  it('uses the singular for one trip and one passenger', async () => {
    setLocale('en')
    const one = await render({ total_trips: 1, total_passengers: 1 })
    expect(one).toMatch(/1<[^>]*>\s*trip</)
    expect(one).toMatch(/1<[^>]*>\s*person</)
    const many = await render({})
    expect(many).toMatch(/12<[^>]*>\s*trips</)
    expect(many).toMatch(/20<[^>]*>\s*people</)
  })
})
