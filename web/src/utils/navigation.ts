export interface NavSection<T> {
  /** Translation key of the heading; absent for the section that stands alone at the top. */
  headingKey?: string
  items: T[]
}

/** The sections of the desktop sidebar, each listing the names of its pages in order. */
export const NAV_SECTIONS: { headingKey?: string; names: string[] }[] = [
  { names: ['dashboard'] },
  { headingKey: 'shell.navSections.tracking', names: ['fleet', 'drives', 'carpools', 'odometer', 'energy'] },
  { headingKey: 'shell.navSections.costs', names: ['expenses', 'maintenance', 'tires', 'comparison'] },
  { headingKey: 'shell.navSections.management', names: ['vehicles', 'account'] },
]

/**
 * Spreads the pages the account can open over the sections, in the order the sections list them. A section left without
 * a page is dropped, and a page no section names is kept in a last section without a heading, so no page disappears.
 */
export function groupNavItems<T extends { name: string }>(items: T[], sections = NAV_SECTIONS): NavSection<T>[] {
  const placed = new Set<string>()
  const grouped: NavSection<T>[] = []
  for (const section of sections) {
    const inSection = section.names.map((name) => items.find((item) => item.name === name)).filter((item): item is T => !!item)
    inSection.forEach((item) => placed.add(item.name))
    if (inSection.length > 0) grouped.push({ headingKey: section.headingKey, items: inSection })
  }
  const rest = items.filter((item) => !placed.has(item.name))
  if (rest.length > 0) grouped.push({ items: rest })
  return grouped
}
