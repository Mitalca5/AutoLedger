export type BudgetStatus = 'ok' | 'warning' | 'over'

export interface BudgetUsage {
  percent: number
  status: BudgetStatus
  remaining: number
}

export const BUDGET_WARNING_PERCENT = 80

/** Share of the monthly budget already spent; `percent` is not capped so an overrun shows (e.g. 125). */
export function budgetUsage(spent: number, budget: number): BudgetUsage {
  const percent = (spent / budget) * 100
  let status: BudgetStatus = 'ok'
  if (spent > budget) status = 'over'
  else if (percent >= BUDGET_WARNING_PERCENT) status = 'warning'
  return { percent, status, remaining: budget - spent }
}

/** Parses a typed budget ("250", "250,5"); null when it is not a positive amount. */
export function parseBudgetInput(raw: string): number | null {
  const value = Number(raw.trim().replace(',', '.'))
  return Number.isFinite(value) && value > 0 ? Math.round(value * 100) / 100 : null
}
