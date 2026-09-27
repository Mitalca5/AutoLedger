import { beforeEach, describe, expect, it } from 'vitest'
import {
  currentDistanceUnit,
  displayDistanceToKm,
  DEFAULT_DISTANCE_UNIT,
  distanceUnit,
  formatDistance,
  formatDistanceValue,
  formatSpeed,
  kmToDisplayDistance,
  perDistance,
  perDistanceToPerKm,
  setDistanceUnit,
  speedUnit,
} from './units'

describe('setDistanceUnit', () => {
  beforeEach(() => setDistanceUnit(null))

  it('applies a supported value', () => {
    setDistanceUnit('mi')
    expect(currentDistanceUnit()).toBe('mi')
  })

  it('falls back to the default for a missing or unsupported value', () => {
    setDistanceUnit('mi')
    setDistanceUnit(undefined)
    expect(currentDistanceUnit()).toBe(DEFAULT_DISTANCE_UNIT)

    setDistanceUnit('mi')
    setDistanceUnit('furlong')
    expect(currentDistanceUnit()).toBe(DEFAULT_DISTANCE_UNIT)
  })
})

describe('distance conversion', () => {
  it('is a no-op in km', () => {
    setDistanceUnit('km')
    expect(kmToDisplayDistance(100)).toBe(100)
    expect(displayDistanceToKm(100)).toBe(100)
  })

  it('converts both ways in miles', () => {
    setDistanceUnit('mi')
    expect(kmToDisplayDistance(160.9344)).toBeCloseTo(100, 5)
    expect(displayDistanceToKm(100)).toBeCloseTo(160.9344, 5)
  })

  it('round-trips without drift', () => {
    setDistanceUnit('mi')
    expect(displayDistanceToKm(kmToDisplayDistance(42000))).toBeCloseTo(42000, 6)
  })
})

describe('formatDistance', () => {
  it('rounds and appends the unit in km', () => {
    setDistanceUnit('km')
    expect(formatDistance(42000.4)).toBe(`${Math.round(42000.4).toLocaleString('fr-FR')} km`)
  })

  it('converts, rounds and appends the unit in miles', () => {
    setDistanceUnit('mi')
    expect(formatDistance(160.9344)).toBe(`${(100).toLocaleString('fr-FR')} mi`)
  })
})

describe('per-distance figures', () => {
  beforeEach(() => setDistanceUnit(null))

  it('keep their value in km', () => {
    expect(perDistance(0.2)).toBe(0.2)
    expect(perDistanceToPerKm(0.2)).toBe(0.2)
    expect(distanceUnit()).toBe('km')
    expect(speedUnit()).toBe('km/h')
  })

  it('grow by the mile ratio in miles and convert back', () => {
    setDistanceUnit('mi')
    expect(perDistance(0.1)).toBeCloseTo(0.1609344)
    expect(perDistanceToPerKm(perDistance(0.37))).toBeCloseTo(0.37)
    expect(distanceUnit()).toBe('mi')
    expect(formatSpeed(100)).toBe('62 mph')
  })

  it('format a distance with decimals when asked', () => {
    setDistanceUnit('mi')
    expect(formatDistanceValue(16.09344, 1)).toBe((10).toLocaleString('fr-FR', { minimumFractionDigits: 1 }))
    expect(formatDistance(1.609344, 1)).toBe(`${(1).toLocaleString('fr-FR', { minimumFractionDigits: 1 })} mi`)
  })

  it('formats consumption range bounds in the current unit', () => {
    setDistanceUnit('km')
    expect(perDistance(1).toLocaleString('fr-FR', { maximumFractionDigits: 1 })).toBe('1')
    expect(perDistance(100).toLocaleString('fr-FR', { maximumFractionDigits: 1 })).toBe('100')

    setDistanceUnit('mi')
    expect(perDistance(1).toLocaleString('fr-FR', { maximumFractionDigits: 1 })).toBe('1,6')
    expect(perDistance(100).toLocaleString('fr-FR', { maximumFractionDigits: 1 })).toBe('160,9')
  })
})
