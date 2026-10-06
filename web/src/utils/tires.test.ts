import { describe, expect, it } from 'vitest'
import {
  copiedSessionFromSession,
  defaultTargetTireIds,
  getConditionBadge,
  getLastDismountInfo,
  getSeasonIcon,
  getTireSelectLabel,
  isMountedPosition,
  positionOnTire,
  sessionFormFromCopy,
  sessionFormFromSession,
  validateTireForm,
  validateTreadDepth,
  wearTone,
} from './tires'

const entry = (id: string, brand: string, model: string, sessions: any[] = []) => ({
  tire: { id, brand, model, dimension: '235/40 R19', current_position: 'STORAGE' },
  sessions,
})

describe('isMountedPosition', () => {
  it('accepts the four wheels only', () => {
    expect(['FL', 'FR', 'RL', 'RR'].every(isMountedPosition)).toBe(true)
    expect(isMountedPosition('STORAGE')).toBe(false)
    expect(isMountedPosition('DISPOSED')).toBe(false)
  })
})

describe('getConditionBadge / getSeasonIcon', () => {
  it('labels each condition and falls back to unknown', () => {
    expect(getConditionBadge('GOOD').label).toBe('Bon état')
    expect(getConditionBadge('WARNING').label).toBe('À surveiller')
    expect(getConditionBadge('CRITICAL').label).toBe('Usure critique')
    expect(getConditionBadge('???').label).toBe('Inconnu')
  })

  it('treats anything but winter and all-season as summer', () => {
    expect(getSeasonIcon('WINTER').label).toBe('Hiver')
    expect(getSeasonIcon('ALL_SEASON').label).toBe('4 Saisons')
    expect(getSeasonIcon('SUMMER').label).toBe('Été')
    expect(getSeasonIcon('').label).toBe('Été')
  })
})

describe('getLastDismountInfo', () => {
  it('returns the most recent dismount, ignoring sessions still mounted', () => {
    const tires = [
      entry('a', 'M', 'PS4', [
        { dismounted_date: '2025-03-01T00:00:00Z', dismounted_odometer: 100 },
        { dismounted_date: '2025-09-15T00:00:00Z', dismounted_odometer: 250 },
        { dismounted_date: null, dismounted_odometer: null },
      ]),
    ]
    expect(getLastDismountInfo(tires, 'a')).toEqual({ date: '2025-09-15', odometer: 250 })
  })

  it('gives null when the tire was never dismounted or is unknown', () => {
    const tires = [entry('a', 'M', 'PS4', [{ dismounted_date: null }])]
    expect(getLastDismountInfo(tires, 'a')).toBeNull()
    expect(getLastDismountInfo(tires, 'missing')).toBeNull()
  })

  it('reports a zero odometer as unknown', () => {
    const tires = [entry('a', 'M', 'PS4', [{ dismounted_date: '2025-03-01T00:00:00Z', dismounted_odometer: 0 }])]
    expect(getLastDismountInfo(tires, 'a')).toEqual({ date: '2025-03-01', odometer: null })
  })
})

describe('defaultTargetTireIds', () => {
  const tires = [entry('a', 'Michelin', 'PS4'), entry('b', 'Michelin', 'PS4'), entry('c', 'Nokian', 'WR')]

  it('prefers tires of the same brand and model, never the source itself', () => {
    expect(defaultTargetTireIds(tires, { id: 'a', brand: 'Michelin', model: 'PS4' })).toEqual(['b'])
  })

  it('falls back to every other tire when the source has no sibling', () => {
    expect(defaultTargetTireIds(tires, { id: 'c', brand: 'Nokian', model: 'WR' })).toEqual(['a', 'b'])
  })
})

describe('getTireSelectLabel', () => {
  it('describes position, distance, sessions and DOT', () => {
    const label = getTireSelectLabel({
      tire: { brand: 'Michelin', model: 'PS4', dimension: '235/40 R19', current_position: 'FL', dot_code: '1224' },
      total_distance_km: 12345.6,
      sessions: [{}, {}],
    })
    expect(label).toContain('Michelin PS4 (235/40 R19) — Roue FL')
    expect(label).toContain('2 sessions')
    expect(label).toContain('DOT 1224')
  })

  it('uses the singular for one session and names stored and disposed tires', () => {
    const base = { brand: 'B', model: 'M', dimension: 'D' }
    expect(getTireSelectLabel({ tire: { ...base, current_position: 'STORAGE' }, sessions: [{}] })).toContain('Au garage')
    expect(getTireSelectLabel({ tire: { ...base, current_position: 'STORAGE' }, sessions: [{}] })).toContain('1 session')
    expect(getTireSelectLabel({ tire: { ...base, current_position: 'DISPOSED' }, sessions: [] })).toContain('Au rebut')
  })

  it('is empty without a tire', () => {
    expect(getTireSelectLabel(null)).toBe('')
    expect(getTireSelectLabel({})).toBe('')
  })
})

describe('session forms', () => {
  const session = {
    position: 'RL',
    mounted_date: '2025-03-05T00:00:00Z',
    mounted_odometer: 120000,
    dismounted_date: '2025-10-01T00:00:00Z',
    dismounted_odometer: 130000,
    distance_km: 10000,
    notes: null,
  }

  it('turns a stored session into an editable form', () => {
    expect(sessionFormFromSession(session)).toEqual({
      position: 'RL',
      mounted_date: '2025-03-05',
      mounted_odometer: 120000,
      is_dismounted: true,
      dismounted_date: '2025-10-01',
      dismounted_odometer: 130000,
      distance_km: 10000,
      notes: '',
    })
  })

  it('marks a session without a dismount date as still mounted', () => {
    const form = sessionFormFromSession({ ...session, dismounted_date: null, dismounted_odometer: null })
    expect(form.is_dismounted).toBe(false)
    expect(form.dismounted_odometer).toBe(0)
  })

  it('keeps blanks in a copied session instead of defaulting to today', () => {
    const copied = copiedSessionFromSession({ ...session, mounted_date: null, dismounted_date: null })
    expect(copied.mounted_date).toBe('')
    expect(copied.dismounted_date).toBe('')
    expect(copied.is_dismounted).toBe(false)
  })

  it('pastes on the wheel a mounted tire sits on, and on the copied position for a tire in storage', () => {
    const copied = copiedSessionFromSession(session)
    expect(sessionFormFromCopy(copied, { current_position: 'FR' }).position).toBe('FR')
    expect(sessionFormFromCopy(copied, { current_position: 'STORAGE' }).position).toBe('RL')
    expect(sessionFormFromCopy(copied, { current_position: 'DISPOSED' }).position).toBe('RL')
    expect(sessionFormFromCopy({ ...copied, position: '' }, null).position).toBe('FL')
  })
})

describe('positionOnTire', () => {
  it('keeps the wheel of a mounted tire, whatever the copied position', () => {
    expect(positionOnTire('FL', { current_position: 'RR' })).toBe('RR')
  })

  it('uses the copied position for a stored or disposed tire, falling back on FL', () => {
    expect(positionOnTire('RL', { current_position: 'STORAGE' })).toBe('RL')
    expect(positionOnTire('RL', { current_position: 'DISPOSED' })).toBe('RL')
    expect(positionOnTire('', undefined)).toBe('FL')
  })
})

describe('validateTreadDepth', () => {
  it('accepts worn and deep treads', () => {
    expect(validateTreadDepth(0.8)).toBeNull()
    expect(validateTreadDepth(12)).toBeNull()
    expect(validateTreadDepth(20)).toBeNull()
  })

  it('refuses empty, zero, negative and above 20 mm', () => {
    for (const v of ['', null, 0, -1, 20.1]) expect(validateTreadDepth(v)).not.toBeNull()
  })
})

describe('wearTone', () => {
  it('follows the tread condition', () => {
    expect(wearTone(10, 'GOOD')).toBe('ok')
    expect(wearTone(10, 'WARNING')).toBe('warning')
    expect(wearTone(10, 'CRITICAL')).toBe('danger')
  })

  it('is danger past 80 % of the lifespan whatever the tread says', () => {
    expect(wearTone(81, 'GOOD')).toBe('danger')
    expect(wearTone(80, 'GOOD')).toBe('ok')
  })

  it('falls back on the lifespan alone without a condition', () => {
    expect(wearTone(50)).toBe('ok')
  })
})

describe('validateTireForm', () => {
  const valid = { brand: 'B', model: 'M', dimension: '205/55 R16', price: '', initial_depth_mm: 8, min_legal_depth_mm: 1.6, estimated_lifespan_km: 45000 }

  it('accepts a form with an empty price', () => {
    expect(validateTireForm(valid)).toEqual({})
  })

  it('flags blank identity fields, ignoring whitespace', () => {
    expect(Object.keys(validateTireForm({ ...valid, brand: '  ', model: '', dimension: ' ' })).sort()).toEqual(['brand', 'dimension', 'model'])
  })

  it('refuses a negative price but not zero', () => {
    expect(validateTireForm({ ...valid, price: -5 }).price).toBeDefined()
    expect(validateTireForm({ ...valid, price: 0 }).price).toBeUndefined()
  })

  it('requires a tread depth above the legal wear indicator', () => {
    expect(validateTireForm({ ...valid, initial_depth_mm: 1.6 }).initial_depth_mm).toBeDefined()
    expect(validateTireForm({ ...valid, initial_depth_mm: 0 }).initial_depth_mm).toBeDefined()
  })

  it('requires a positive lifespan', () => {
    expect(validateTireForm({ ...valid, estimated_lifespan_km: 0 }).estimated_lifespan_km).toBeDefined()
  })
})
