export interface CompletenessGap {
  key: string
  amount: number | null
  count: number
  quantity: number
  target?: string
}

export interface GapAction {
  key: string
}

// The action key that removes each gap of the completeness report
export const GAP_ACTION_KEYS: Record<string, string> = {
  depreciation: 'acquisition',
  unpriced_charges: 'charges',
  untracked_distance: 'consumption',
  unconverted_expenses: 'unconverted',
}

// Actions ordered as the gaps are (largest first), those without a gap keeping their order after them, each with the
// gap it removes.
export function orderActionsByGap<T extends GapAction>(actions: T[], gaps: CompletenessGap[] | null | undefined): (T & { gap: CompletenessGap | null })[] {
  const byAction = new Map<string, CompletenessGap>()
  const order: string[] = []
  for (const g of gaps ?? []) {
    const action = GAP_ACTION_KEYS[g.key]
    if (action && !byAction.has(action)) {
      byAction.set(action, g)
      order.push(action)
    }
  }
  const rank = (key: string) => {
    const i = order.indexOf(key)
    return i === -1 ? order.length : i
  }
  return actions
    .map((a, i) => ({ a, i }))
    .sort((x, y) => rank(x.a.key) - rank(y.a.key) || x.i - y.i)
    .map(({ a }) => ({ ...a, gap: byAction.get(a.key) ?? null }))
}
