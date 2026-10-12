import { describe, expect, it } from 'vitest'
import { orderActionsByGap } from './completenessGaps'

const gap = (key: string, amount: number | null) => ({ key, amount, count: 0, quantity: 0 })

describe('orderActionsByGap', () => {
  const actions = [{ key: 'drives' }, { key: 'charges' }, { key: 'insurance' }, { key: 'acquisition' }, { key: 'consumption' }]

  it('follows the order of the gaps and keeps the other actions after, in their order', () => {
    const ordered = orderActionsByGap(actions, [gap('depreciation', null), gap('untracked_distance', 8000), gap('unpriced_charges', 2500)])
    expect(ordered.map((a) => a.key)).toEqual(['acquisition', 'consumption', 'charges', 'drives', 'insurance'])
  })

  it('attaches the gap an action removes', () => {
    const ordered = orderActionsByGap(actions, [gap('unpriced_charges', 2500)])
    expect(ordered[0].gap?.amount).toBe(2500)
    expect(ordered[1].gap).toBeNull()
  })

  it('leaves the order alone without gaps', () => {
    expect(orderActionsByGap(actions, undefined).map((a) => a.key)).toEqual(actions.map((a) => a.key))
  })
})
