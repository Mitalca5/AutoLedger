import { describe, expect, it } from 'vitest'
import type { TariffPlan } from '@/services/api'
import { emptyPlanForm, formToPayload, groupVersions, nextVersionForm, planProblem, planToForm, removeBand, renameBand } from './tariffPlans'

const plan = (over: Partial<TariffPlan> = {}): TariffPlan => ({
  id: 'p1',
  user_id: 'u',
  name: 'Heures creuses',
  plan_type: 'BANDS',
  currency: 'EUR',
  bands: [
    { name: 'peak', rate_cents: 0.25 },
    { name: 'offpeak', rate_cents: 0.17 },
  ],
  rules: [{ days: [], start: '22:00', end: '06:00', band: 'offpeak' }],
  default_band: 'peak',
  standing_charge_cents: 12.5,
  valid_from: '2026-01-01',
  valid_to: '2026-06-30',
  is_default: true,
  created_at: '',
  updated_at: '',
  ...over,
})

const valid = () => planToForm(plan())

describe('tariff plan form', () => {
  it('round-trips a banded plan through the form', () => {
    const payload = formToPayload(planToForm(plan()))
    expect(payload).toMatchObject({
      name: 'Heures creuses',
      plan_type: 'BANDS',
      bands: [
        { name: 'peak', rate_cents: 0.25 },
        { name: 'offpeak', rate_cents: 0.17 },
      ],
      rules: [{ days: [], start: '22:00', end: '06:00', band: 'offpeak' }],
      default_band: 'peak',
      standing_charge_cents: 12.5,
      valid_from: '2026-01-01',
      valid_to: '2026-06-30',
      is_default: true,
    })
  })

  it('sends a flat plan with its single rate and no bands', () => {
    const form = emptyPlanForm('EUR')
    form.name = 'Base'
    form.planType = 'FLAT'
    form.flatRate = 0.2
    expect(planProblem(form)).toBeNull()
    const payload = formToPayload(form)
    expect(payload).toMatchObject({ plan_type: 'FLAT', flat_rate_cents: 0.2, valid_from: null, valid_to: null })
    expect(payload.bands).toBeUndefined()
  })

  it('treats a plan stored as peak / off-peak like a banded one', () => {
    const form = planToForm(plan({ plan_type: 'TIME_OF_USE', bands: [], rules: [] }))
    expect(form.planType).toBe('BANDS')
    expect(form.bands).toHaveLength(1)
  })

  it('sends a rule that covers all seven days as every day', () => {
    const form = valid()
    form.rules[0].days = [0, 1, 2, 3, 4, 5, 6]
    expect(formToPayload(form).rules?.[0].days).toEqual([])
    form.rules[0].days = [6, 0]
    expect(formToPayload(form).rules?.[0].days).toEqual([0, 6])
  })

  it('defaults to the first band when none is chosen', () => {
    const form = valid()
    form.defaultBand = ''
    expect(formToPayload(form).default_band).toBe('peak')
  })

  describe('validation', () => {
    it('accepts a complete plan', () => {
      expect(planProblem(valid())).toBeNull()
    })

    it.each([
      ['name', (f: ReturnType<typeof valid>) => (f.name = '  ')],
      ['bandsRequired', (f: ReturnType<typeof valid>) => (f.bands = [])],
      ['bandName', (f: ReturnType<typeof valid>) => (f.bands[1].name = ' ')],
      ['bandDuplicate', (f: ReturnType<typeof valid>) => (f.bands[1].name = 'peak')],
      ['bandRate', (f: ReturnType<typeof valid>) => (f.bands[0].rate = null)],
      ['bandRate', (f: ReturnType<typeof valid>) => (f.bands[0].rate = -1)],
      ['defaultBand', (f: ReturnType<typeof valid>) => (f.defaultBand = 'red')],
      ['ruleBand', (f: ReturnType<typeof valid>) => (f.rules[0].band = 'red')],
      ['ruleTime', (f: ReturnType<typeof valid>) => (f.rules[0].start = '25:00')],
      ['ruleTime', (f: ReturnType<typeof valid>) => (f.rules[0].end = '')],
      ['ruleDays', (f: ReturnType<typeof valid>) => (f.rules[0].days = [7])],
      ['validity', (f: ReturnType<typeof valid>) => (f.validTo = '2025-12-31')],
    ])('reports %s', (problem, mutate) => {
      const form = valid()
      mutate(form)
      expect(planProblem(form)).toBe(problem)
    })

    it('accepts 24:00 as the end of a rule and open-ended validity', () => {
      const form = valid()
      form.rules[0].end = '24:00'
      form.validTo = ''
      expect(planProblem(form)).toBeNull()
    })

    it('requires a rate for a flat plan', () => {
      const form = valid()
      form.planType = 'FLAT'
      form.flatRate = null
      expect(planProblem(form)).toBe('flatRate')
    })
  })

  describe('band edits', () => {
    it('renaming a band keeps its rules and default pointing at it', () => {
      const form = valid()
      renameBand(form, 1, 'night')
      renameBand(form, 0, 'day')
      expect(form.rules[0].band).toBe('night')
      expect(form.defaultBand).toBe('day')
      expect(planProblem(form)).toBeNull()
    })

    it('a band just added (no name yet) renames without touching rules', () => {
      const form = valid()
      form.bands.push({ name: '', rate: null })
      form.rules.push({ days: [], start: '00:00', end: '01:00', band: '' })
      renameBand(form, 2, 'red')
      expect(form.rules[1].band).toBe('')
    })

    it('removing a band drops its rules and resets the default', () => {
      const form = valid()
      removeBand(form, 1)
      expect(form.rules).toHaveLength(0)
      removeBand(form, 0)
      expect(form.defaultBand).toBe('')
    })
  })

  describe('versions', () => {
    it('a next version starts the day after the previous one ends', () => {
      const form = nextVersionForm(plan())
      expect(form).toMatchObject({ validFrom: '2026-07-01', validTo: '', isDefault: false, name: 'Heures creuses' })
      expect(nextVersionForm(plan({ valid_to: '2026-12-31' })).validFrom).toBe('2027-01-01')
      expect(nextVersionForm(plan({ valid_to: null })).validFrom).toBe('')
    })

    it('groups plans by name regardless of case', () => {
      const groups = groupVersions([plan({ id: 'a' }), plan({ id: 'b', name: 'heures creuses ' }), plan({ id: 'c', name: 'Base' })])
      expect(groups.map((g) => [g.name, g.versions.map((v) => v.id)])).toEqual([
        ['Heures creuses', ['a', 'b']],
        ['Base', ['c']],
      ])
    })
  })
})
