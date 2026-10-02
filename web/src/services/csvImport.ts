import type { ApiErrorBody } from '@/services/apiError'

export type CSVImportType = 'CHARGES' | 'DRIVES' | 'FUEL' | 'ODOMETER'

export interface CSVRowError extends ApiErrorBody {
  code: string
  message: string
}

export interface CSVColumnMapping {
  index: number
  header: string
  field: string
  detected: boolean
}

export type CSVDateOrder = '' | 'dmy' | 'mdy' | 'ymd'
export type CSVDecimalSeparator = '' | '.' | ','

export interface CSVImportOptions {
  type?: CSVImportType | ''
  skipDuplicates: boolean
  // column index -> field; an empty field ignores the column
  mapping?: Record<number, string>
  dateOrder?: CSVDateOrder
  decimalSeparator?: CSVDecimalSeparator
}

export function csvImportForm(file: File, opts: CSVImportOptions): FormData {
  const form = new FormData()
  form.append('file', file)
  if (opts.type) form.append('type', opts.type)
  form.append('skip_duplicates', opts.skipDuplicates ? 'true' : 'false')
  if (opts.mapping && Object.keys(opts.mapping).length > 0) form.append('mapping', JSON.stringify(opts.mapping))
  if (opts.dateOrder) form.append('date_order', opts.dateOrder)
  if (opts.decimalSeparator) form.append('decimal_separator', opts.decimalSeparator)
  return form
}

export interface CSVPreviewResult {
  type: CSVImportType | 'UNKNOWN'
  total_rows: number
  valid_rows: number
  invalid_rows: number
  duplicate_rows: number
  headers: string[]
  mapping: CSVColumnMapping[]
  fields: string[]
  sample_rows: Record<string, string>[]
  errors?: CSVRowError[]
  errors_truncated?: boolean
}

export interface CSVExecuteResult {
  type: CSVImportType
  total_rows: number
  imported_count: number
  skipped_count: number
  error_count: number
  committed: boolean
  errors?: CSVRowError[]
  errors_truncated?: boolean
}

export interface CSVImportProfile {
  id: string
  name: string
  import_type: CSVImportType
  columns: Record<string, string>
  date_order: CSVDateOrder
  decimal_separator: CSVDecimalSeparator
}

// What a profile holds: the field each header feeds, whether the header or the user chose it.
export function profileColumns(mapping: CSVColumnMapping[]): Record<string, string> {
  const columns: Record<string, string> = {}
  for (const col of mapping) columns[col.header] = col.field
  return columns
}

// Applies a profile to the headers of a file: the columns it names, by header text, keep its field.
export function profileMapping(headers: string[], columns: Record<string, string>): Record<number, string> {
  const mapping: Record<number, string> = {}
  headers.forEach((header, index) => {
    if (header in columns) mapping[index] = columns[header]
  })
  return mapping
}
