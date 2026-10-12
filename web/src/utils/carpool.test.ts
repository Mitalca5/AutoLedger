import { describe, expect, it } from 'vitest'
import {
  allocate,
  carpoolCsvRows,
  carpoolCoverage,
  cents,
  clampPassengerStops,
  DRIVE_WINDOW_DAYS,
  driveWindow,
  earliestSelectedDriveDate,
  emptyLeg,
  estimateTitle,
  legTotalCents,
  pickerDrives,
  shiftDay,
  legsFromEstimate,
  legsFromTrip,
  newPassenger,
  passengersFromTrip,
  remapPassengerStops,
  stopKeys,
  stopNames,
  toDateInputString,
  type LegForm,
  type PassengerForm,
} from './carpool'

const leg = (over: Partial<LegForm> = {}): LegForm => ({ ...emptyLeg(), ...over })
const pax = (over: Partial<PassengerForm> = {}): PassengerForm => ({ passenger_name: 'P', seats: 1, amount_paid: 0, board_stop_index: 0, alight_stop_index: 1, notes: '', ...over })

describe('money helpers', () => {
  it('converts euros to whole cents', () => {
    expect(cents(12.34)).toBe(1234)
    expect(cents('0.1')).toBe(10)
    expect(cents('')).toBe(0)
    expect(cents(0.005)).toBe(1)
  })

  it('sums the six cost fields of a leg in cents', () => {
    expect(legTotalCents({ electricity_cost: 8.4, tolls_cost: 15.2, tires_cost: 2.1, maintenance_cost: 1.5, insurance_cost: 1.2, other_cost: 0 })).toBe(2840)
    expect(legTotalCents({})).toBe(0)
  })
})

describe('toDateInputString', () => {
  it('formats local dates and rejects blanks and garbage', () => {
    expect(toDateInputString(new Date(2026, 4, 2))).toBe('2026-05-02')
    expect(toDateInputString('')).toBe('')
    expect(toDateInputString(null)).toBe('')
    expect(toDateInputString('nope')).toBe('')
  })
})

describe('stopNames', () => {
  it('has a start and an end without legs', () => {
    expect(stopNames([])).toEqual(['Départ', 'Arrivée'])
  })

  it('takes stop names from the leg labels, falling back on the previous leg end', () => {
    expect(stopNames([leg({ start_label: 'Paris', end_label: 'Dijon' }), leg({ end_label: 'Lyon' }), leg({})])).toEqual(['Paris', 'Dijon', 'Lyon', 'Arrivée'])
    expect(stopNames([leg({ start_label: 'A' }), leg({})])).toEqual(['A', 'Arrêt 1', 'Arrivée'])
    expect(stopNames([leg({})])).toEqual(['Départ', 'Arrivée'])
  })
})

describe('allocate', () => {
  it('leaves the whole cost to the driver without passengers', () => {
    const a = allocate([leg({ electricity_cost: 10 })], [])
    expect(a.total).toBe(1000)
    expect(a.passengersShare).toBe(0)
    expect(a.driverShare).toBe(1000)
  })

  it('splits a leg between the driver and the people on board', () => {
    const a = allocate([leg({ electricity_cost: 9 })], [pax(), pax()])
    expect(a.legDetails[0]).toEqual({ total: 900, seats: 2, perPerson: 300 })
    expect(a.shares).toEqual([300, 300])
    expect(a.driverShare).toBe(300)
  })

  it('counts seats and only the legs a passenger rides', () => {
    const legs = [leg({ electricity_cost: 10 }), leg({ electricity_cost: 20 })]
    const a = allocate(legs, [pax({ alight_stop_index: 2 }), pax({ seats: 2, board_stop_index: 1, alight_stop_index: 2 })])
    // leg 0: driver + 1 seat; leg 1: driver + 1 + 2 seats
    expect(a.legDetails[0].seats).toBe(1)
    expect(a.legDetails[1].seats).toBe(3)
    expect(a.shares[0]).toBe(500 + 500)
    expect(a.shares[1]).toBe(500 * 2)
    expect(a.total).toBe(3000)
    expect(a.driverShare).toBe(3000 - a.passengersShare)
  })

  it('rounds each person share down to the cent, like the backend', () => {
    const a = allocate([leg({ electricity_cost: 10 })], [pax(), pax()])
    expect(a.legDetails[0].perPerson).toBe(333)
    expect(a.driverShare).toBe(1000 - 666)
  })
})

describe('stopKeys', () => {
  it('identifies stops by the drive they start, and the last one by the drive it ends', () => {
    expect(stopKeys([leg({ drive_id: 'a' }), leg({ drive_id: 'b' })])).toEqual(['start:a', 'start:b', 'end:b'])
    expect(stopKeys([leg({}), leg({})])).toEqual(['index:0', 'index:1', 'index:2'])
    expect(stopKeys([])).toEqual(['index:0'])
  })
})

describe('clampPassengerStops', () => {
  it('keeps stops inside the legs, boarding before alighting', () => {
    const p = [pax({ board_stop_index: 5, alight_stop_index: 9 })]
    clampPassengerStops(p, 3, 3)
    expect(p[0].alight_stop_index).toBe(3)
    expect(p[0].board_stop_index).toBe(2)
  })

  it('sends passengers who rode to the end (or came before the legs) to the new last stop', () => {
    const p = [pax({ alight_stop_index: 2 }), pax({ alight_stop_index: 1 })]
    clampPassengerStops(p, 4, 2)
    expect(p.map((x) => x.alight_stop_index)).toEqual([4, 1])
    const q = [pax({ alight_stop_index: 1 })]
    clampPassengerStops(q, 3, 0)
    expect(q[0].alight_stop_index).toBe(3)
  })

  it('does nothing without legs', () => {
    const p = [pax({ board_stop_index: 4, alight_stop_index: 7 })]
    clampPassengerStops(p, 0, 2)
    expect(p[0].board_stop_index).toBe(4)
  })
})

describe('remapPassengerStops', () => {
  const drive = (id: string) => leg({ drive_id: id })

  it('keeps a passenger on the same drives when one is added before', () => {
    const previous = [drive('a'), drive('b'), drive('c')]
    const next = [drive('z'), drive('a'), drive('b'), drive('c')]
    const p = [pax({ board_stop_index: 1, alight_stop_index: 2 })]
    remapPassengerStops(previous, next, p)
    expect(p[0].board_stop_index).toBe(2)
    expect(p[0].alight_stop_index).toBe(3)
  })

  it('moves a passenger who rode to the end to the new end', () => {
    const p = [pax({ board_stop_index: 0, alight_stop_index: 2 })]
    remapPassengerStops([drive('a'), drive('b')], [drive('a'), drive('b'), drive('c')], p)
    expect(p[0].alight_stop_index).toBe(3)
  })

  it('falls back to the first stop and the end when their drive was removed', () => {
    const p = [pax({ board_stop_index: 1, alight_stop_index: 1 })]
    remapPassengerStops([drive('a'), drive('b')], [drive('c')], p)
    expect(p[0].board_stop_index).toBe(0)
    expect(p[0].alight_stop_index).toBe(1)
  })

  it('leaves passengers alone when there were no legs before', () => {
    const p = [pax({ board_stop_index: 0, alight_stop_index: 5 })]
    remapPassengerStops([], [drive('a')], p)
    expect(p[0].alight_stop_index).toBe(5)
  })
})

describe('forms from data', () => {
  it('starts a passenger on the whole trip', () => {
    expect(newPassenger(1, 3)).toMatchObject({ passenger_name: 'Passager 2', seats: 1, board_stop_index: 0, alight_stop_index: 3 })
    expect(newPassenger(0, 0).alight_stop_index).toBe(1)
  })

  it('maps estimate and trip legs, defaulting the missing fields', () => {
    const l = legsFromEstimate({ legs: [{ drive_id: 'a', distance_km: 12, electricity_cost: 3 }] })[0]
    expect(l).toMatchObject({ drive_id: 'a', start_label: '', distance_km: 12, electricity_cost: 3, other_cost: 0 })
    expect(legsFromEstimate({})).toEqual([])
    expect(legsFromTrip({ legs: [{ start_label: 'A', end_label: 'B', distance_km: 5 }] })[0].drive_id).toBeNull()
  })

  it('maps trip passengers with defaults for seats, amount and stops', () => {
    const p = passengersFromTrip({ passengers: [{ passenger_name: 'Alice', board_stop_index: 0 }, { passenger_name: 'Bob', seats: 2, amount_paid: 15, board_stop_index: 1, alight_stop_index: 1 }] }, 4)
    expect(p[0]).toMatchObject({ seats: 1, amount_paid: 0, alight_stop_index: 4, notes: '' })
    expect(p[1]).toMatchObject({ seats: 2, amount_paid: 15, board_stop_index: 1, alight_stop_index: 1 })
  })
})

describe('estimate helpers', () => {
  it('titles a trip first stop to last stop, with the number of legs when there are several', () => {
    expect(estimateTitle(['Paris', 'Dijon'], 1)).toBe('Paris → Dijon')
    expect(estimateTitle(['Paris', 'Dijon', 'Lyon'], 2)).toBe('Paris → Lyon (2 étapes)')
  })

  it('finds the date of the earliest selected drive', () => {
    const drives = [
      { id: 'a', start_time: '2026-05-03T08:00:00' },
      { id: 'b', start_time: '2026-05-01T08:00:00' },
      { id: 'c', start_time: '2026-04-01T08:00:00' },
    ]
    expect(earliestSelectedDriveDate(drives, ['a', 'b'])).toBe('2026-05-01')
    expect(earliestSelectedDriveDate(drives, [])).toBe('')
  })
})

describe('carpoolCsvRows', () => {
  it('returns the raw title and computes the coverage, capped at 100 %', () => {
    const rows = carpoolCsvRows([
      { id: 't1', date: '2026-05-02T10:00:00', title: 'Paris "Est" → Lyon', distance_km: 230, passengers: [{}, {}], total_cost: 60.5, total_revenue: 40, net_cost: 20.5 },
      { id: 't2', title: '', distance_km: 110, passenger_count: 1, total_cost: 10, total_revenue: 25, net_cost: -15 },
      { id: 't3', total_cost: 0, total_revenue: 12 },
    ])
    expect(rows[0][2]).toBe('Paris "Est" → Lyon')
    expect(rows[0][4]).toBe(2)
    expect(rows[0][5]).toBe('60.50')
    expect(rows[0][8]).toBe(66)
    expect(rows[1][8]).toBe(100)
    expect(rows[2][8]).toBe(0)
    expect(rows[2][4]).toBe(0)
  })
})

describe('carpoolCoverage', () => {
  it('gives the paid and fair shares as a percentage of the cost', () => {
    const c = carpoolCoverage({ total_cost: 34.06, total_revenue: 27, passengers_cost_share: 22.71 })
    expect(c.paidPct).toBeCloseTo(79.3, 1)
    expect(c.fairPct).toBeCloseTo(66.7, 1)
    expect(c.status).toBe('above')
  })

  it('says when the passengers paid less than their share, or exactly it', () => {
    expect(carpoolCoverage({ total_cost: 30, total_revenue: 10, passengers_cost_share: 20 }).status).toBe('below')
    expect(carpoolCoverage({ total_cost: 30, total_revenue: 20, passengers_cost_share: 19.995 }).status).toBe('fair')
  })

  it('does not divide by zero', () => {
    const c = carpoolCoverage({ total_cost: 0, total_revenue: 5 })
    expect(c.paidPct).toBe(0)
    expect(c.fairPct).toBe(0)
    expect(carpoolCoverage(undefined).total).toBe(0)
  })
})

describe('drive window of the carpool form', () => {
  it('moves a date across month and year ends', () => {
    expect(shiftDay('2026-06-14', 7)).toBe('2026-06-21')
    expect(shiftDay('2026-06-14', -14)).toBe('2026-05-31')
    expect(shiftDay('2026-01-03', -7)).toBe('2025-12-27')
    expect(shiftDay('2028-02-25', 7)).toBe('2028-03-03')
  })

  it('spans the days before and after the saved date', () => {
    expect(DRIVE_WINDOW_DAYS).toBe(7)
    expect(driveWindow('2026-06-14')).toEqual({ from: '2026-06-07', to: '2026-06-21' })
    expect(driveWindow('2026-06-14', 2)).toEqual({ from: '2026-06-12', to: '2026-06-16' })
  })

  it('keeps the selected drives outside the window listed, most recent first', () => {
    const d = (id: string, start: string) => ({ id, start_time: start })
    const known = new Map([
      ['old', d('old', '2026-03-01T10:00:00Z')],
      ['a', d('a', '2026-06-14T08:00:00Z')],
    ])
    const list = pickerDrives([d('a', '2026-06-14T08:00:00Z'), d('b', '2026-06-15T09:00:00Z')], known, ['old', 'a', 'unknown'])
    expect(list.map((x) => x.id)).toEqual(['b', 'a', 'old'])
  })

  it('does not list a selected drive twice or an unselected known one', () => {
    const known = new Map([['x', { id: 'x', start_time: '2026-01-01T00:00:00Z' }]])
    expect(pickerDrives([], known, [])).toEqual([])
  })
})
