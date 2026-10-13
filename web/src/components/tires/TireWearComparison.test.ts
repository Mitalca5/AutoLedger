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
  it('names the lifespans and the gap without changing the recorded depth', async () => {
    setLocale('en')
    const html = await render(stat)
    expect(html).toContain('Measured vs expected wear')
    expect(html).toContain('26,667 km')
    expect(html).toContain('40,000 km')
    expect(html).toContain('Lifespan 33.3% shorter than expected')
    expect(html).toContain('text-warning-400')
  })
  it('does not colour a gap while the measure is thin', async () => {
    setLocale('en')
    const html = await render({ ...stat, wear_rate_confidence: 0.3 })
    expect(html).toContain('shorter than expected')
    expect(html).not.toContain('text-warning-400')
  })
  it('converts the lifespans to miles and translates the labels', async () => {
    setLocale('en'); setDistanceUnit('mi')
    const miles = await render(stat)
    expect(miles).toContain('16,570 mi')
    expect(miles).toContain('24,855 mi')
    setLocale('fr'); setDistanceUnit('km')
    expect(await render(stat)).toContain('Usure mesurée et attendue')
  })
  it('hides an unavailable comparison in old responses or without a measurement', async () => {
    expect(await render({ wear_rate_source: 'measured' })).not.toContain('data-testid')
    expect(await render({ ...stat, wear_rate_source: 'default' })).not.toContain('data-testid')
  })
})
