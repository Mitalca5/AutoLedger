/** A mileage the page lists: a reading entered on the odometer page, or the mileage typed on a fill-up. */
export interface OdometerHistoryItem {
  key: string
  kind: 'READING' | 'FUEL'
  id: string
  date: string
  odometer: number
  notes?: string | null
  source?: string | null
}

interface ReadingLike {
  id: string
  date: string
  odometer: number
  notes?: string | null
  source?: string | null
}

interface FuelLogLike {
  id: string
  date: string
  odometer?: number | null
}

/**
 * The readings and the fill-ups that carry a mileage, oldest first. A fill-up keeps its mileage on its own record (it
 * is the single source), so it is listed here without being copied; only the mileage typed on the fill-up counts, never
 * the one estimated from the readings around it.
 */
export function mergeOdometerHistory(readings: ReadingLike[], fuelLogs: FuelLogLike[]): OdometerHistoryItem[] {
  const items: OdometerHistoryItem[] = readings.map((r) => ({
    key: `reading-${r.id}`,
    kind: 'READING',
    id: r.id,
    date: r.date,
    odometer: r.odometer,
    notes: r.notes,
    source: r.source,
  }))
  for (const f of fuelLogs) {
    if (typeof f.odometer !== 'number') continue
    items.push({ key: `fuel-${f.id}`, kind: 'FUEL', id: f.id, date: f.date, odometer: f.odometer })
  }
  return items.sort((a, b) => Date.parse(a.date) - Date.parse(b.date) || a.odometer - b.odometer)
}
