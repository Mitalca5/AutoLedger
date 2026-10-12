import { kmToDisplayDistance } from '@/units'

interface ComparisonCsvRow {
  name: string
  mode: string
  years: number
  km: number
  tracked: number
  ice: number
  savings: number
  breakEven: string
}

// Text stays raw: the shared CSV encoder quotes each cell exactly once.
export function comparisonCsvRows(rows: ComparisonCsvRow[]): (string | number)[][] {
  return rows.map(row => [
    row.name,
    row.mode,
    row.years,
    Math.round(kmToDisplayDistance(row.km)),
    Number(row.tracked).toFixed(2),
    Number(row.ice).toFixed(2),
    Number(row.savings).toFixed(2),
    row.breakEven,
  ])
}
