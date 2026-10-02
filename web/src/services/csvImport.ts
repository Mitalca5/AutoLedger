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
