import { intlLocale } from '@/i18n'

/** A number with a fixed count of decimals in the current language ("0,0198", "18.7"), for display only. */
export function formatNumber(value: number, digits = 1): string {
  return value.toLocaleString(intlLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

/** A percentage in the current language, with the language's own spacing ("18,7 %" in French). */
export function formatPercent(value: number, digits = 1): string {
  return (value / 100).toLocaleString(intlLocale(), { style: 'percent', minimumFractionDigits: digits, maximumFractionDigits: digits })
}
