export const PLACE_CATEGORIES = ['HOME', 'WORK', 'PUBLIC', 'OTHER'] as const
export type PlaceCategory = (typeof PLACE_CATEGORIES)[number]

export interface ChargingPlace {
  id: string
  name: string
  category: PlaceCategory
  aliases: string[]
}

export interface ChargingLocation {
  text: string
  sessions: number
  kwh_added: number
  place_id: string | null
}

/** Catalog key of a place category. */
export function placeCategoryKey(category: string): string {
  return `expenses.chargingPlaces.category.${PLACE_CATEGORIES.includes(category as PlaceCategory) ? category : 'OTHER'}`
}

/** Locations still to classify, the busiest first (the server already sorts by sessions). */
export function unclassifiedLocations(locations: ChargingLocation[]): ChargingLocation[] {
  return locations.filter((l) => !l.place_id)
}

/** Share of the reported energy whose location is linked to a place, 0..1 (1 when nothing is reported). */
export function classifiedShare(locations: ChargingLocation[]): number {
  const total = locations.reduce((sum, l) => sum + l.kwh_added, 0)
  if (total <= 0) return 1
  const classified = locations.filter((l) => l.place_id).reduce((sum, l) => sum + l.kwh_added, 0)
  return classified / total
}
