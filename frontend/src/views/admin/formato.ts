const data = new Intl.DateTimeFormat('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })

/** "21/10/2026 12:00"; valor inválido vira "—". */
export function quando(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : data.format(d).replace(',', '')
}
