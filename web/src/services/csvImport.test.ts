import { describe, expect, it } from 'vitest'
import { csvImportForm, profileColumns, profileMapping } from './csvImport'

const file = new File(['a,b\n1,2\n'], 'x.csv', { type: 'text/csv' })

describe('csvImportForm', () => {
  it('sends only what the user chose', () => {
    const form = csvImportForm(file, { skipDuplicates: true })
    expect(form.get('skip_duplicates')).toBe('true')
    expect(form.has('type')).toBe(false)
    expect(form.has('mapping')).toBe(false)
    expect(form.has('date_order')).toBe(false)
    expect(form.has('decimal_separator')).toBe(false)
  })

  it('serialises the mapping and formats', () => {
    const form = csvImportForm(file, {
      type: 'CHARGES',
      skipDuplicates: false,
      mapping: { 1: 'kwh', 2: '' },
      dateOrder: 'mdy',
      decimalSeparator: ',',
    })
    expect(form.get('type')).toBe('CHARGES')
    expect(form.get('skip_duplicates')).toBe('false')
    expect(JSON.parse(String(form.get('mapping')))).toEqual({ '1': 'kwh', '2': '' })
    expect(form.get('date_order')).toBe('mdy')
    expect(form.get('decimal_separator')).toBe(',')
  })
})

describe('import profiles', () => {
  const mapping = [
    { index: 0, header: 'Jour', field: 'date', detected: true },
    { index: 1, header: 'Truc', field: 'kwh', detected: false },
    { index: 2, header: 'Note', field: '', detected: false },
  ]

  it('keeps the field of every header', () => {
    expect(profileColumns(mapping)).toEqual({ Jour: 'date', Truc: 'kwh', Note: '' })
  })

  it('maps the columns of another file by header, whatever their order', () => {
    expect(profileMapping(['Note', 'Truc', 'Autre'], { Jour: 'date', Truc: 'kwh', Note: '' })).toEqual({ 0: '', 1: 'kwh' })
  })
})
