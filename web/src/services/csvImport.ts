import type { ApiErrorBody } from '@/services/apiError'

export type CSVImportType = 'CHARGES' | 'DRIVES' | 'FUEL' | 'ODOMETER'

export interface CSVRowError extends ApiErrorBody {
  code: string
  message: string
}

export interface CSVPreviewResult {
  type: CSVImportType | 'UNKNOWN'
  total_rows: number
  valid_rows: number
  invalid_rows: number
  duplicate_rows: number
  headers: string[]
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
