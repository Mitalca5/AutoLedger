import { expect, it } from 'vitest'
import { comparisonCsvRows } from './comparisonCsv'
import { csvContent } from './csv'

it('encodes a comparison name once, including quotes and line breaks', () => {
  const name = 'Family, "weekend"\ntrip'
  const rows = comparisonCsvRows([{ name, mode: 'Projection', years: 5, km: 12000, tracked: 123.5, ice: 130, savings: 6.5, breakEven: 'Not reached' }])
  expect(rows[0][0]).toBe(name)
  expect(csvContent([], rows).split('\n').slice(1).join('\n')).toBe('"Family, ""weekend""\ntrip",Projection,5,12000,123.50,130.00,6.50,Not reached')
})
