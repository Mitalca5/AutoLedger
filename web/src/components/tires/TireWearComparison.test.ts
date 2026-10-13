import { afterEach, describe, expect, it } from 'vitest'
import { createSSRApp } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { i18n, setLocale } from '@/i18n'
import { setDistanceUnit } from '@/units'
import TireWearComparison from './TireWearComparison.vue'

const stat = {
  wear_rate_source: 'measured', wear_rate_confidence: 0.7,
  wear_comparison: { measured_rate_per_10k_km: 2.4, expected_rate_per_10k_km: 1.6, measured_lifespan_km: 26667, expected_lifespan_km: 40000, delta_pct: 50, reference: 'declared' },
}
const render = (value: any) => renderToString(createSSRApp(TireWearComparison, { stat: value }).use(i18n))

afterEach(() => { setLocale('fr'); setDistanceUnit('km') })

describe('measured tire wear display', () => {
  it('names measured and expected wear without changing the recorded depth', async () => {
    setLocale('en')
    const html = await render(stat)
    expect(html).toContain('Measured vs expected wear')
    expect(html).toContain('2.40 mm / 10,000 km')
    expect(html).toContain('Lifespan: 26,667 km')
    expect(html).toContain('50.0% faster')
    expect(html).toContain('Measurement weight in the forecast: 70.0%')
    expect(html).toContain('text-warning-400')
  })
  it('converts both the rate and lifespan to miles and translates the labels', async () => {
    setLocale('en'); setDistanceUnit('mi')
    const miles = await render(stat)
    expect(miles).toContain('3.86 mm / 10,000 mi')
    expect(miles).toContain('Lifespan: 16,570 mi')
    setLocale('fr'); setDistanceUnit('km')
    expect(await render(stat)).toContain('Usure mesurée et attendue')
  })
  it('hides an unavailable comparison in old responses or without a measurement', async () => {
    expect(await render({ wear_rate_source: 'measured' })).not.toContain('data-testid')
    expect(await render({ ...stat, wear_rate_source: 'default' })).not.toContain('data-testid')
  })
})
