import { describe, expect, it } from 'vitest'
import { buildTireTimeline, groupTimelineTires, type TimelineTireInfo } from './tireTimeline'

const tire = (id: string, model: string, season: string, sessions: any[]) => ({ tire: { id, brand: 'Hankook', model, dimension: '255/45 R19', season }, sessions })
const session = (position: string, from: number, to: number | null, mountedDate = '2025-04-28', dismountedDate: string | null = '2025-11-16T00:00:00Z') => ({
  position,
  mounted_odometer: from,
  dismounted_odometer: to,
  mounted_date: mountedDate,
  dismounted_date: to == null ? null : dismountedDate,
})
const allWheels = (idPrefix: string, model: string, season: string, from: number, to: number | null) =>
  ['FL', 'FR', 'RL', 'RR'].map((w) => tire(`${idPrefix}-${w}`, model, season, [session(w, from, to)]))

describe('buildTireTimeline', () => {
  it('uses one lane when every wheel had the same tires over the same km', () => {
    const tl = buildTireTimeline([...allWheels('a', 'Ventus', 'SUMMER', 0, 30000), ...allWheels('b', 'Icept', 'WINTER', 30000, null)], 40000)
    expect(tl.lanes).toHaveLength(1)
    expect(tl.lanes[0].shortLabel).toBe('Tous')
    expect(tl.lanes[0].segments.map((s) => s.endKm)).toEqual([30000, 40000])
  })

  it('splits by axle, with a short label that fits a narrow column', () => {
    const front = ['FL', 'FR'].map((w) => tire(`f-${w}`, 'Ventus', 'SUMMER', [session(w, 0, 40000)]))
    const rear = ['RL', 'RR'].map((w) => tire(`r-${w}`, 'Leboncoin', 'WINTER', [session(w, 0, 40000)]))
    const tl = buildTireTimeline([...front, ...rear], 40000)
    expect(tl.lanes.map((l) => l.shortLabel)).toEqual(['AV', 'AR'])
    expect(tl.lanes[0].label).not.toBe(tl.lanes[0].shortLabel)
  })

  it('uses the wheel code for a lane of one wheel', () => {
    const tires = [
      tire('fl', 'Ventus', 'SUMMER', [session('FL', 0, 40000)]),
      tire('fr', 'Ventus', 'SUMMER', [session('FR', 0, 20000)]),
      tire('rl', 'Ventus', 'SUMMER', [session('RL', 0, 40000)]),
      tire('rr', 'Ventus', 'SUMMER', [session('RR', 0, 40000)]),
    ]
    expect(buildTireTimeline(tires, 40000).lanes.map((l) => l.shortLabel)).toContain('FR')
  })

  it("keeps each session's season and dates for the detail", () => {
    const tl = buildTireTimeline(allWheels('a', 'Icept', 'WINTER', 0, 30000), 30000)
    const info = tl.lanes[0].segments[0].tires[0]
    expect(info.season).toBe('WINTER')
    expect(info.mountedDate).toBe('2025-04-28')
    expect(info.dismountedDate).toBe('2025-11-16T00:00:00Z')
  })

  it('leaves the removal date empty while the tire is still fitted', () => {
    const tl = buildTireTimeline(allWheels('a', 'Icept', 'WINTER', 10000, null), 25000)
    const info = tl.lanes[0].segments[0].tires[0]
    expect(info.ongoing).toBe(true)
    expect(info.dismountedDate).toBeNull()
    expect(info.endKm).toBe(25000)
  })
})

describe('groupTimelineTires', () => {
  const tire = (tireId: string, position: string): TimelineTireInfo => ({
    tireId, label: 'B M', dimension: '195/55R16', position, startKm: 0, endKm: 10, ongoing: false, season: 'WINTER', mountedDate: '2024-11-01', dismountedDate: '2025-04-01',
  })

  it('counts identical tires fitted without a wheel instead of repeating them', () => {
    const groups = groupTimelineTires([tire('a', 'STORAGE'), tire('b', 'STORAGE'), tire('c', 'STORAGE'), tire('d', 'STORAGE')])
    expect(groups).toHaveLength(1)
    expect(groups[0].count).toBe(4)
  })

  it('keeps one line per wheel', () => {
    expect(groupTimelineTires([tire('a', 'FL'), tire('b', 'FR')]).map((g) => g.count)).toEqual([1, 1])
  })
})
