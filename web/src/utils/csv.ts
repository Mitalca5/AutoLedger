/**
 * Utility to trigger a CSV file download from tabular data.
 */
export function csvCell(value: string | number): string {
  // Keep numeric cells numeric; protect user-provided text the same way as backend exports.
  const raw = String(value)
  const numericText = /^[+-]?\d+(?:\.\d+)?$/.test(raw)
  const text = typeof value === 'string' && !numericText && /^[=+\-@\t\r]/.test(raw) ? `'${raw}` : raw
  return /[",\r\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text
}

export function csvContent(headers: (string | number)[], rows: (string | number)[][]): string {
  return [headers, ...rows].map((row) => row.map(csvCell).join(',')).join('\n')
}

export function downloadCsv(filename: string, headers: (string | number)[], rows: (string | number)[][]) {
  const content = csvContent(headers, rows)
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename.endsWith('.csv') ? filename : `${filename}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
