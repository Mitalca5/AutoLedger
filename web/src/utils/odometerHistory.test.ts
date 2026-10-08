import { describe, expect, it } from 'vitest'
import { mergeOdometerHistory } from './odometerHistory'

const reading = (id: string, date: string, odometer: number, extra = {}) => ({ id, date, odometer, ...extra })

describe('mergeOdometerHistory', () => {
  it('lists readings and fill-ups with a mileage in date order', () => {
    const merged = mergeOdometerHistory(
      [reading('r2', '2026-06-01', 30000), reading('r1', '2026-01-01', 10000)],
      [{ id: 'f1', date: '2026-03-15T08:00:00Z', odometer: 20000 }],
    )
    expect(merged.map((m) => `${m.kind}:${m.odometer}`)).toEqual(['READING:10000', 'FUEL:20000', 'READING:30000'])
  })

  it('leaves out the fill-ups entered without a mileage', () => {
    const merged = mergeOdometerHistory([], [
      { id: 'f1', date: '2026-03-15T08:00:00Z', odometer: null },
      { id: 'f2', date: '2026-03-16T08:00:00Z' },
      { id: 'f3', date: '2026-03-17T08:00:00Z', odometer: 0 },
    ])
    expect(merged.map((m) => m.id)).toEqual(['f3'])
  })

  it('keeps what a reading says about itself and gives each item a distinct key', () => {
    const merged = mergeOdometerHistory([reading('x', '2026-01-01', 100, { notes: 'inspection', source: 'HA' })], [{ id: 'x', date: '2026-01-01T10:00:00Z', odometer: 100 }])
    expect(merged).toHaveLength(2)
    expect(new Set(merged.map((m) => m.key)).size).toBe(2)
    expect(merged.find((m) => m.kind === 'READING')).toMatchObject({ notes: 'inspection', source: 'HA' })
  })

  it('orders a same-day pair by mileage', () => {
    const merged = mergeOdometerHistory([reading('r', '2026-01-01', 500)], [{ id: 'f', date: '2026-01-01', odometer: 400 }])
    expect(merged.map((m) => m.kind)).toEqual(['FUEL', 'READING'])
  })

  it('does not change the lists it receives', () => {
    const readings = [reading('b', '2026-02-01', 2), reading('a', '2026-01-01', 1)]
    mergeOdometerHistory(readings, [])
    expect(readings.map((r) => r.id)).toEqual(['b', 'a'])
  })
})
