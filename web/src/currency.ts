import { intlLocale } from '@/i18n'

// A vehicle's currency is fixed at creation (see CLAUDE.md) and travels with its own data —
// unlike the language or distance unit, it is not an account-wide preference, so every call site
// passes the currency of the vehicle the amount belongs to rather than reading a global setting.

/** Formats a cents amount as currency, e.g. formatMoney(154000, "EUR") -> "1 540,00 €". */
export function formatMoney(cents: number, currency: string): string {
  return formatAmount(cents / 100, currency)
}

/** Formats a currency amount already in main units (not cents), e.g. a €/km rate. */
export function formatAmount(amount: number, currency: string, maximumFractionDigits = 2): string {
  return new Intl.NumberFormat(intlLocale(), { style: 'currency', currency, maximumFractionDigits }).format(amount)
}

/** Just the symbol ("€", "$"...), for a chart axis or dataset label that cannot show a full amount. */
export function currencySymbol(currency: string): string {
  const part = new Intl.NumberFormat(intlLocale(), { style: 'currency', currency }).formatToParts(0).find((p) => p.type === 'currency')
  return part?.value ?? currency
}

/** A money axis tick without decimals, shortened above a thousand ("400 k€"). */
export function formatAxisAmount(amount: number, currency: string): string {
  return new Intl.NumberFormat(intlLocale(), { style: 'currency', currency, notation: 'compact', maximumFractionDigits: Math.abs(amount) >= 1000 ? 1 : 0 }).format(amount)
}
