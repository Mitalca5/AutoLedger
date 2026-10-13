import { afterEach, describe, expect, it } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { i18n, setLocale } from '@/i18n'
import { setDistanceUnit } from '@/units'
import TireWearComparison from './TireWearComparison.vue'

const stat = {
  wear_rate_source: 'measured', wear_rate_confidence: 0.7,
  wear_comparison: { measured_lifespan_km: 26667, expected_lifespan_km: 40000, delta_pct: -33.3, reference: 'declared' },
}
const render = (value: any) => renderToString(createSSRApp(TireWearComparison, { stat: value }).use(i18n))

afterEach(() => { setLocale('fr'); setDistanceUnit('km') })

describe('measured tire wear display', () => {
  it('puts the two lifespans side by side and the gap in a pill', async () => {
    setLocale('en')
    const html = await render(stat)
    expect(html).toContain('Measured vs expected wear')
    expect(html).toContain('26,667')
    expect(html).toContain('40,000')
    expect(html).toContain('Lifespan −33%')
    expect(html).toContain('border-warning-500/30')
  })
  it('signs a longer lifespan and colours it green', async () => {
    setLocale('en')
    const html = await render({ ...stat, wear_comparison: { ...stat.wear_comparison, measured_lifespan_km: 60000, expected_lifespan_km: 40000, delta_pct: 50 } })
    expect(html).toContain('Lifespan +50%')
    expect(html).toContain('border-success-500/30')
  })
  it('says the rate is on target instead of a zero gap', async () => {
    setLocale('en')
    expect(await render({ ...stat, wear_comparison: { ...stat.wear_comparison, delta_pct: 0 } })).toContain('On the expected rate')
  })
  it('keeps the gap neutral and carries the caveat in its footnote while the measure is thin', async () => {
    setLocale('en')
    const html = await render({ ...stat, wear_rate_confidence: 0.3 })
    expect(html).toContain('Lifespan −33%')
    expect(html).toContain('Measure still thin')
    expect(html).not.toContain('border-warning-500/30')
    expect(await render(stat)).not.toContain('Measure still thin')
  })
  it('converts the lifespans to miles and translates the labels', async () => {
    setLocale('en'); setDistanceUnit('mi')
    const miles = await render(stat)
    expect(miles).toContain('16,570')
    expect(miles).toContain('24,855')
    expect(miles).toContain('mi')
    setLocale('fr'); setDistanceUnit('km')
    expect(await render(stat)).toContain('Usure mesurée et attendue')
  })
  it('hides an unavailable comparison in old responses or without a measurement', async () => {
    expect(await render({ wear_rate_source: 'measured' })).not.toContain('data-testid')
    expect(await render({ ...stat, wear_rate_source: 'default' })).not.toContain('data-testid')
  })
})
