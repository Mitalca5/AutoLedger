// Categories of a month whose total can be traced back to the entries that make it up.
const CONTRIBUTION_CATEGORIES: Record<string, string> = {
  energy: 'ENERGY',
  insurance: 'INSURANCE',
}

/** API category of a monthly breakdown row, or null when the row has no entry drill-down. */
export function contributionCategory(key: string): string | null {
  return Object.hasOwn(CONTRIBUTION_CATEGORIES, key) ? CONTRIBUTION_CATEGORIES[key] : null
}

/** Page listing the records of a ledger source table. */
export function contributionRoute(sourceTable: string): string {
  return sourceTable === 'maintenance_expenses' ? '/expenses' : '/energy'
}

/** Wording of a recurrence interval: the translation key and its parameters. */
export function recurrenceLabel(months: number): { key: string; params: { n: number } } {
  if (months === 1) return { key: 'dashboard.monthDetailModal.contributions.recurringMonthly', params: { n: 1 } }
  if (months === 12) return { key: 'dashboard.monthDetailModal.contributions.recurringYearly', params: { n: 12 } }
  return { key: 'dashboard.monthDetailModal.contributions.recurring', params: { n: months } }
}
