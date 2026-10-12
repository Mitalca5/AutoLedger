import { describe, expect, it } from 'vitest'
import { classifiedShare, placeCategoryKey, unclassifiedLocations, type ChargingLocation } from './chargingPlaces'

const locations: ChargingLocation[] = [
  { text: 'Maison', sessions: 8, kwh_added: 300, place_id: 'p1' },
  { text: 'Valence Supercharger, Valence', sessions: 3, kwh_added: 100, place_id: null },
]

describe('placeCategoryKey', () => {
  it('maps a category to its catalog key and falls back on other', () => {
    expect(placeCategoryKey('HOME')).toBe('expenses.chargingPlaces.category.HOME')
    expect(placeCategoryKey('OUTSIDE')).toBe('expenses.chargingPlaces.category.OUTSIDE')
    expect(placeCategoryKey('SPA')).toBe('expenses.chargingPlaces.category.OTHER')
  })
})

describe('unclassifiedLocations', () => {
  it('keeps the locations without a place', () => {
    expect(unclassifiedLocations(locations).map((l) => l.text)).toEqual(['Valence Supercharger, Valence'])
  })
})

describe('classifiedShare', () => {
  it('weights by energy', () => {
    expect(classifiedShare(locations)).toBeCloseTo(0.75)
  })
  it('is complete when nothing is reported', () => {
    expect(classifiedShare([])).toBe(1)
  })
})
