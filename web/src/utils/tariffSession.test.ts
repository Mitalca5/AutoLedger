import { describe, expect, it } from 'vitest'
import { sessionPriceRequest } from './tariffSession'

describe('sessionPriceRequest', () => {
  it('sends the instants of the session with its energy', () => {
    const req = sessionPriceRequest('v1', '2026-07-10T22:00', '2026-07-11T06:00', '10,5')
    expect(req).toEqual({
      vehicle_id: 'v1',
      start_time: new Date('2026-07-10T22:00').toISOString(),
      end_time: new Date('2026-07-11T06:00').toISOString(),
      kwh: 10.5,
    })
  })

  it('prices at the start when the end is missing or not after it', () => {
    const start = new Date('2026-07-10T22:00').toISOString()
    expect(sessionPriceRequest('v1', '2026-07-10T22:00', '', '5')?.end_time).toBe(start)
    expect(sessionPriceRequest('v1', '2026-07-10T22:00', '2026-07-10T21:00', '5')?.end_time).toBe(start)
  })

  it('cannot price without a vehicle, a valid start or some energy', () => {
    expect(sessionPriceRequest('', '2026-07-10T22:00', '', '5')).toBeNull()
    expect(sessionPriceRequest('v1', '', '', '5')).toBeNull()
    expect(sessionPriceRequest('v1', 'nope', '', '5')).toBeNull()
    expect(sessionPriceRequest('v1', '2026-07-10T22:00', '', '')).toBeNull()
    expect(sessionPriceRequest('v1', '2026-07-10T22:00', '', '0')).toBeNull()
  })
})
