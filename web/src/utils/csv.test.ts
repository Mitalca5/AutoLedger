import { describe, expect, it } from 'vitest'
import { csvCell, csvContent } from './csv'

describe('CSV encoding', () => {
  it('quotes commas, escapes quotes and keeps line breaks in one field', () => {
    expect(csvCell('Family, weekend')).toBe('"Family, weekend"')
    expect(csvCell('A "quoted" name')).toBe('"A ""quoted"" name"')
    expect(csvCell('a\nb')).toBe('"a\nb"')
    expect(csvCell('a\rb')).toBe('"a\rb"')
  })

  it('protects spreadsheet formulas without changing negative amounts', () => {
    expect(csvCell('=SUM(A1:A2)')).toBe("'=SUM(A1:A2)")
    expect(csvCell(-12.5)).toBe('-12.5')
    expect(csvCell('-15.00')).toBe('-15.00')
    expect(csvCell('-1+2')).toBe("'-1+2")
  })

  it('keeps column counts for headers and rows without changing numbers', () => {
    expect(csvContent(['Name', 'Cost, EUR'], [['Family, weekend', 12.5], ['Plain', 0]]))
      .toBe('Name,"Cost, EUR"\n"Family, weekend",12.5\nPlain,0')
  })
})
