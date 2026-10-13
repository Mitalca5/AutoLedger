import { describe, expect, it } from 'vitest'
import {
  OUTSIDE_COLOR,
  UNCLASSIFIED_COLOR,
  donutItems,
  estimatedSeries,
  groupColors,
  groupLabel,
  monthlyDatasets,
  monthlyPrices,
  placeCategoryRows,
  categoryColor,
  savingsView,
  totalsWithEstimate,
  unclassifiedShare,
  type HomeSavings,
  type PlaceStats,
  type PlaceStatsGroup,
} from './chargingInsights'

const group = (over: Partial<PlaceStatsGroup>): PlaceStatsGroup => ({
  key: 'home', name: 'Home', category: 'HOME', sessions: 2, kwh_added: 100, cost: 20, priced_kwh: 100, price_per_kwh: 0.2,
  free_sessions: 0, unknown_sessions: 0, unknown_kwh: 0, ...over,
})

const stats = (groups: PlaceStatsGroup[]): PlaceStats => ({
  months: 12,
  summary: { sessions: 4, kwh_added: 150, cost: 30, priced_kwh: 150, price_per_kwh: 0.2, free_sessions: 0, unknown_sessions: 0, unknown_kwh: 0, zero_kwh_sessions: 0 },
  groups,
  series: [
    { month: '2026-02', kwh_added: 100, cost: 20, priced_kwh: 100, price_per_kwh: 0.2, groups: [{ key: 'home', kwh_added: 100, cost: 20, priced_kwh: 100 }] },
    { month: '2026-03', kwh_added: 50, cost: 10, priced_kwh: 0, price_per_kwh: null, groups: [{ key: '', kwh_added: 50, cost: 10, priced_kwh: 0 }] },
  ],
  home_savings: { home_price_per_kwh: 0.2, home_priced_kwh: 100, away_sessions: 0, away_priced_kwh: 0, away_cost: 0, hypothetical_cost: 0, savings: 0, unclassified_kwh: 50 },
})

describe('groupColors', () => {
  it('gives each place of a category its own shade and unclassified a neutral one', () => {
    const colors = groupColors([group({}), group({ key: 'home2', name: 'Parents' }), group({ key: '', name: undefined, category: undefined })])
    expect(colors.home).not.toBe(colors.home2)
    expect(colors['']).toBe(UNCLASSIFIED_COLOR)
  })

  it('gives the outside group its own colour, apart from the classified categories', () => {
    const colors = groupColors([group({}), group({ key: 'outside', name: undefined, category: 'OUTSIDE' })])
    expect(colors.outside).toBe(OUTSIDE_COLOR)
  })
})

describe('groupLabel', () => {
  it('names a place, and falls back on the unclassified wording', () => {
    expect(groupLabel(group({}), 'Unclassified')).toBe('Home')
    expect(groupLabel(group({ key: '' }), 'Unclassified')).toBe('Unclassified')
  })

  it('labels the derived outside group, even though it has no place name', () => {
    const outside = group({ key: 'outside', name: undefined, category: 'OUTSIDE' })
    expect(groupLabel(outside, 'Unclassified', 'Outside')).toBe('Outside')
    expect(groupLabel(outside, 'Unclassified')).toBe('Unclassified')
  })
})

describe('donutItems', () => {
  it('switches between energy and cost', () => {
    const groups = [group({})]
    const colors = groupColors(groups)
    expect(donutItems(groups, 'energy', colors, 'U')[0].amount).toBe(100)
    expect(donutItems(groups, 'cost', colors, 'U')[0].amount).toBe(20)
  })
})

describe('monthlyDatasets and monthlyPrices', () => {
  const s = stats([group({}), group({ key: '', name: undefined, category: undefined, kwh_added: 50, cost: 10 })])
  it('aligns every group on the months, with zero where it did not charge', () => {
    const sets = monthlyDatasets(s, groupColors(s.groups), 'U', 'energy')
    expect(sets.find((d) => d.key === 'home')!.data).toEqual([100, 0])
    expect(sets.find((d) => d.key === '')!.data).toEqual([0, 50])
  })
  it('switches the same layout to the recorded cost', () => {
    const sets = monthlyDatasets(s, groupColors(s.groups), 'U', 'cost')
    expect(sets.find((d) => d.key === 'home')!.data).toEqual([20, 0])
    expect(sets.find((d) => d.key === '')!.data).toEqual([0, 10])
  })
  it('keeps a month without a known cost as a gap', () => {
    expect(monthlyPrices(s)).toEqual([0.2, null])
  })
})

describe('savingsView', () => {
  const base: HomeSavings = { home_price_per_kwh: 0.2, home_priced_kwh: 100, away_sessions: 3, away_priced_kwh: 50, away_cost: 30, hypothetical_cost: 10, savings: 20, unclassified_kwh: 5 }
  it('reports the saving and its basis', () => {
    expect(savingsView(base)).toMatchObject({ state: 'ok', amount: 20, better: true, awayKwh: 50, homePrice: 0.2 })
    // 20 saved on the 30 the away charges cost
    expect(savingsView(base)).toMatchObject({ pct: (20 / 30) * 100 })
  })
  it('reports a cheaper away charging as a negative saving', () => {
    expect(savingsView({ ...base, savings: -10 })).toMatchObject({ state: 'ok', amount: 10, better: false, pct: (10 / 30) * 100 })
    expect(savingsView({ ...base, away_cost: 0, savings: 0 })).toMatchObject({ state: 'ok', pct: null })
  })
  it('explains why there is no figure', () => {
    expect(savingsView({ ...base, reason: 'no_home', home_price_per_kwh: null })).toEqual({ state: 'none', reasonKey: 'expenses.chargingInsights.savings.noHome' })
    expect(savingsView({ ...base, reason: 'no_away' })).toEqual({ state: 'none', reasonKey: 'expenses.chargingInsights.savings.noAway' })
  })
})

describe('unclassifiedShare', () => {
  it('is the energy of the unclassified group over the total', () => {
    const s = stats([group({}), group({ key: '', kwh_added: 50 })])
    expect(unclassifiedShare(s)).toBeCloseTo(50 / 150)
  })
  it('is zero without energy or without an unclassified group', () => {
    expect(unclassifiedShare(stats([group({})]))).toBe(0)
  })
})

describe('placeCategoryRows', () => {
  const group = (key: string, category: string | undefined, kwh: number, cost: number, priced: number, unknown = 0) => ({
    key, category, sessions: 1, kwh_added: kwh, cost, priced_kwh: priced, price_per_kwh: priced ? cost / priced : null, free_sessions: 0, unknown_sessions: unknown, unknown_kwh: 0,
  })
  const stats = {
    months: 12,
    summary: { sessions: 5, kwh_added: 100, cost: 0, priced_kwh: 0, price_per_kwh: null, free_sessions: 0, unknown_sessions: 0, unknown_kwh: 0, zero_kwh_sessions: 0 },
    groups: [
      group('h1', 'HOME', 40, 8, 40),
      group('h2', 'HOME', 20, 6, 20),
      group('p1', 'PUBLIC', 30, 15, 20, 1),
      group('', undefined, 10, 0, 0, 2),
    ],
    series: [],
    home_savings: { home_price_per_kwh: null, home_priced_kwh: 0, away_sessions: 0, away_priced_kwh: 0, away_cost: 0, hypothetical_cost: 0, savings: 0, unclassified_kwh: 0 },
  } as any

  it('folds the places of a category, the busiest first', () => {
    const rows = placeCategoryRows(stats)
    expect(rows.map((r) => r.category)).toEqual(['HOME', 'PUBLIC', ''])
    expect(rows[0].kwh).toBe(60)
    expect(rows[0].sharePct).toBe(60)
    expect(rows[0].pricePerKwh).toBeCloseTo(14 / 60, 6)
  })

  it('prices over the priced energy only and leaves unclassified charges unclassified', () => {
    const rows = placeCategoryRows(stats)
    expect(rows[1].pricePerKwh).toBeCloseTo(15 / 20, 6)
    expect(rows[1].unknownSessions).toBe(1)
    expect(rows[2].category).toBe('')
    expect(rows[2].pricePerKwh).toBeNull()
  })

  it('is empty without energy', () => {
    expect(placeCategoryRows({ ...stats, groups: [] })).toEqual([])
  })
})

describe('estimated energy before tracking', () => {
  const withEstimate = (): PlaceStats => {
    const s = stats([group({})])
    s.summary.estimated_km = 400
    s.summary.estimated_kwh = 64
    s.summary.estimated_cost = 16
    s.series[0].estimated_kwh = 40
    s.series[0].estimated_cost = 10
    return s
  }

  it('adds the estimate to the recorded totals without altering them', () => {
    const s = withEstimate()
    expect(totalsWithEstimate(s.summary)).toEqual({ kwh: 214, cost: 46, estimatedKwh: 64, estimatedCost: 16, estimatedKm: 400 })
    expect(s.summary.kwh_added).toBe(150)
  })

  it('keeps the totals recorded when the backend reports no estimate', () => {
    expect(totalsWithEstimate(stats([]).summary)).toEqual({ kwh: 150, cost: 30, estimatedKwh: 0, estimatedCost: 0, estimatedKm: 0 })
  })

  it('gives one value per month, in kWh or cost, and nothing without an estimate', () => {
    const s = withEstimate()
    expect(estimatedSeries(s, 'energy')).toEqual([40, 0])
    expect(estimatedSeries(s, 'cost')).toEqual([10, 0])
    expect(estimatedSeries(stats([]), 'energy')).toBeNull()
  })
})

describe('categoryColor', () => {
  it('gives each category its own hue, with a neutral colour for the unclassified ones', () => {
    expect(categoryColor('HOME')).toBe('#34d399')
    expect(categoryColor('PUBLIC')).toBe('#fbbf24')
    expect(categoryColor('OUTSIDE')).toBe(OUTSIDE_COLOR)
    expect(categoryColor('')).toBe(UNCLASSIFIED_COLOR)
    expect(categoryColor('UNKNOWN')).toBe(UNCLASSIFIED_COLOR)
  })
})
