export function csvText(headers: string[], rows: unknown[][]): string {
  const cell = (value: unknown) => {
    let text = typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value ?? '')
    if (/^[=+\-@]/.test(text)) text = "'" + text
    return '"' + text.replaceAll('"', '""') + '"'
  }
  return '\uFEFF' + [headers, ...rows].map(row => row.map(cell).join(';')).join('\r\n')
}
export function downloadCSV(filename: string, headers: string[], rows: unknown[][]): void {
  const url = URL.createObjectURL(new Blob([csvText(headers, rows)], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url; link.download = filename; link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
