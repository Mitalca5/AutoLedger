import { describe, expect, it } from 'vitest'
import { groupNavItems, NAV_SECTIONS } from './navigation'

const item = (name: string) => ({ name })
const all = NAV_SECTIONS.flatMap((s) => s.names).map(item)

describe('groupNavItems', () => {
  it('spreads every page over the sections, in the order of the sections', () => {
    const grouped = groupNavItems(all)
    expect(grouped.map((g) => g.headingKey)).toEqual([undefined, 'shell.navSections.tracking', 'shell.navSections.costs', 'shell.navSections.management'])
    expect(grouped.flatMap((g) => g.items.map((i) => i.name))).toEqual(all.map((i) => i.name))
  })

  it('drops the pages the account cannot open and the sections left empty', () => {
    const noDrives = all.filter((i) => !['drives', 'carpools', 'fleet', 'odometer', 'energy'].includes(i.name))
    const grouped = groupNavItems(noDrives)
    expect(grouped.map((g) => g.headingKey)).not.toContain('shell.navSections.tracking')
    expect(grouped.flatMap((g) => g.items.map((i) => i.name))).not.toContain('drives')
  })

  it('keeps a page that no section names, after the others', () => {
    const grouped = groupNavItems([...all, item('reports')])
    const last = grouped[grouped.length - 1]
    expect(last.headingKey).toBeUndefined()
    expect(last.items.map((i) => i.name)).toEqual(['reports'])
  })

  it('returns nothing for no page', () => {
    expect(groupNavItems([])).toEqual([])
  })
})
