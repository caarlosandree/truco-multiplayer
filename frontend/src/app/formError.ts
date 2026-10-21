import { ApiError } from '@/api/http'
import { errorText } from '@/game/labels'

export interface FormError {
  code: string
  message: string
  fields: Record<string, string>
  retryAfterSec?: number
}

export function formError(err: unknown): FormError {
  if (err instanceof ApiError) {
    // com tempo de espera conhecido, a frase "aguarde um pouco" seria redundante: o aviso mostra os segundos
    const limitado = err.code === 'too_many_attempts' && err.retryAfterSec !== undefined
    const base: FormError = { code: err.code, message: limitado ? 'Muitas tentativas.' : errorText(err.code, err.message), fields: err.fields }
    return err.retryAfterSec === undefined ? base : { ...base, retryAfterSec: err.retryAfterSec }
  }
  return { code: 'internal', message: errorText('internal'), fields: {} }
}

/** Leva o foco ao primeiro campo inválido do formulário (ou ao alerta, se não houver campo). */
export function focusFirstInvalid(root: ParentNode | null | undefined): void {
  const target = root?.querySelector<HTMLElement>('[aria-invalid="true"]') ?? root?.querySelector<HTMLElement>('[role="alert"]')
  target?.focus({ preventScroll: false })
}
