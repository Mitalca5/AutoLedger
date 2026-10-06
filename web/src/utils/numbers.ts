import { intlLocale } from '@/i18n'

/** A number with a fixed count of decimals in the current language ("0,0198", "18.7"), for display only. */
export function formatNumber(value: number, digits = 1): string {
  return value.toLocaleString(intlLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

/** A percentage in the current language, with the language's own spacing ("18,7 %" in French). */
export function formatPercent(value: number, digits = 1): string {
  return (value / 100).toLocaleString(intlLocale(), { style: 'percent', minimumFractionDigits: digits, maximumFractionDigits: digits })
}

/** What a decimal field accepts: "16,5", "16.5", "1 234,5" and "1.234,5" all read as numbers; anything else is NaN. */
export function parseDecimal(raw: string): number {
  let s = raw.replace(/[\s  ]/g, '')
  if (s === '' || s === '-' || s === '+') return Number.NaN
  const comma = s.lastIndexOf(',')
  const dot = s.lastIndexOf('.')
  if (comma >= 0 && dot >= 0) {
    const decimal = comma > dot ? ',' : '.'
    const group = decimal === ',' ? /\./g : /,/g
    s = s.replace(group, '').replace(decimal, '.')
  } else {
    s = s.replace(',', '.')
  }
  return /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(s) ? Number(s) : Number.NaN
}

/** A number as the text of a decimal field: the language's decimal separator, no thousands grouping, no forced decimals. */
export function formatDecimalInput(value: number): string {
  return value.toLocaleString(intlLocale(), { useGrouping: false, maximumFractionDigits: 12 })
}
