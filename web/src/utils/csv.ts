import { currentLocale } from '@/i18n'

/** How a CSV file is written: the field separator and the decimal mark of its numbers. */
export interface CsvDialect {
  separator: ',' | ';'
  decimal: '.' | ','
}

const POINT: CsvDialect = { separator: ',', decimal: '.' }
const FRENCH: CsvDialect = { separator: ';', decimal: ',' }

/** The dialect a spreadsheet of the interface language opens in one column per field (French Excel splits on `;`). */
export function csvDialect(): CsvDialect {
  return currentLocale() === 'fr' ? FRENCH : POINT
}

const NUMERIC_TEXT = /^[+-]?\d+(?:\.\d+)?$/

/**
 * Utility to trigger a CSV file download from tabular data.
 */
export function csvCell(value: string | number, dialect: CsvDialect = POINT): string {
  // Keep numeric cells numeric; protect user-provided text the same way as backend exports.
  const raw = String(value)
  const numericText = NUMERIC_TEXT.test(raw)
  if (typeof value === 'number' || numericText) return dialect.decimal === ',' ? raw.replace('.', ',') : raw
  const text = /^[=+\-@\t\r]/.test(raw) ? `'${raw}` : raw
  return text.includes(dialect.separator) || /["\r\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text
}

export function csvContent(headers: (string | number)[], rows: (string | number)[][], dialect: CsvDialect = POINT): string {
  return [headers, ...rows].map((row) => row.map((cell) => csvCell(cell, dialect)).join(dialect.separator)).join('\n')
}

export function downloadCsv(filename: string, headers: (string | number)[], rows: (string | number)[][]) {
  // The byte order mark makes Excel read the file as UTF-8 (accents) instead of the system code page.
  const content = `﻿${csvContent(headers, rows, csvDialect())}`
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename.endsWith('.csv') ? filename : `${filename}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
