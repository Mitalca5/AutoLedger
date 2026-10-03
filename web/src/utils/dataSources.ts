import type { Powertrain } from './onboarding'
import { canRefuel } from './vehicles'

export type CsvTemplateType = 'CHARGES' | 'DRIVES' | 'FUEL' | 'ODOMETER'

export type TemplateLanguage = 'en' | 'fr'

const CSV_TEMPLATES: Record<TemplateLanguage, Record<CsvTemplateType, { headers: string[]; example: string[] }>> = {
  en: {
    CHARGES: { headers: ['date', 'kwh', 'cost', 'currency', 'odometer_km', 'location'], example: ['2026-01-15 18:30', '32.5', '6.50', 'EUR', '15200', 'Home'] },
    DRIVES: { headers: ['start_time', 'end_time', 'distance_km', 'kwh', 'tag'], example: ['2026-01-15 08:10', '2026-01-15 08:55', '42.3', '7.4', 'commute'] },
    FUEL: { headers: ['date', 'liters', 'price_per_liter', 'amount', 'odometer_km'], example: ['2026-01-15 12:00', '41.2', '1.789', '73.71', '15200'] },
    ODOMETER: { headers: ['date', 'odometer_km', 'notes'], example: ['2026-01-15', '15200', 'Service'] },
  },
  fr: {
    CHARGES: { headers: ['date', 'kwh', 'cout', 'devise', 'kilometrage_km', 'lieu'], example: ['2026-01-15 18:30', '32.5', '6.50', 'EUR', '15200', 'Maison'] },
    DRIVES: { headers: ['debut', 'fin', 'distance_km', 'kwh', 'etiquette'], example: ['2026-01-15 08:10', '2026-01-15 08:55', '42.3', '7.4', 'trajet quotidien'] },
    FUEL: { headers: ['date', 'litres', 'prix_par_litre', 'montant', 'kilometrage_km'], example: ['2026-01-15 12:00', '41.2', '1.789', '73.71', '15200'] },
    ODOMETER: { headers: ['date', 'kilometrage_km', 'notes'], example: ['2026-01-15', '15200', 'Entretien'] },
  },
}

// Columns the import reads, by their canonical name; the others are ignored. Aliases and accents are accepted.
export const CSV_COLUMNS: Record<CsvTemplateType, { required: string[]; optional: string[] }> = {
  CHARGES: { required: ['date', 'kwh', 'cost'], optional: ['currency', 'fx_rate', 'odometer_km', 'location'] },
  DRIVES: { required: ['start_time', 'distance_km'], optional: ['end_time', 'kwh', 'start_address', 'end_address', 'tag'] },
  FUEL: { required: ['date', 'liters or amount'], optional: ['price_per_liter', 'fuel_type', 'is_full_tank', 'odometer_km', 'notes'] },
  ODOMETER: { required: ['date', 'odometer_km'], optional: ['notes'] },
}

// What each kind of vehicle records: an electric one logs charges and drives, a combustion one fill-ups.
export function csvTemplateTypes(powertrain: Powertrain | null | undefined): CsvTemplateType[] {
  return canRefuel(powertrain) ? ['FUEL', 'ODOMETER'] : ['CHARGES', 'DRIVES', 'ODOMETER']
}

// Header and one example row, in the shape downloadCsv takes.
export function csvTemplate(type: CsvTemplateType, language: TemplateLanguage = 'en'): { headers: string[]; rows: string[][] } {
  const { headers, example } = CSV_TEMPLATES[language][type]
  return { headers, rows: [example] }
}

export function csvTemplateFilename(type: CsvTemplateType): string {
  return `${type.toLowerCase()}-template.csv`
}

// A ready-to-run odometer reading for the webhook endpoint, built from a token the user has just created.
export function webhookSnippet(origin: string, token: string, vehicleId = 'VEHICLE_ID'): string {
  const body = JSON.stringify({ vehicle_id: vehicleId, event_type: 'odometer_update', data: { odometer_km: 15200 } })
  return [
    `curl -X POST ${origin}/api/integrations/homeassistant/event \\`,
    `  -H "Authorization: Bearer ${token}" \\`,
    `  -H "Content-Type: application/json" \\`,
    `  -d '${body}'`,
  ].join('\n')
}
