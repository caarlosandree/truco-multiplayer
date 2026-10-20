import { defineStore } from 'pinia'
import { ApiError } from '@/api/http'
import { errorText } from '@/game/labels'

export interface Toast {
  id: number
  text: string
  kind: 'error' | 'info'
}

const TTL_MS = 5000
let nextId = 1

function hasCodeAndMessage(err: unknown): err is { code: string; message: string } {
  return (
    typeof err === 'object' &&
    err !== null &&
    typeof (err as { code?: unknown }).code === 'string' &&
    typeof (err as { message?: unknown }).message === 'string'
  )
}

export const useToastsStore = defineStore('toasts', {
  state: () => ({ items: [] as Toast[] }),
  actions: {
    push(text: string, kind: Toast['kind'] = 'info') {
      const id = nextId++
      this.items.push({ id, text, kind })
      setTimeout(() => this.dismiss(id), TTL_MS)
    },
    error(code: string, serverMessage?: string) {
      this.push(errorText(code, serverMessage), 'error')
    },
    /** Traduz ApiError, erro WS ({code,message}, ex.: WsSendError) ou qualquer outro (→ 'internal'). */
    fromError(err: unknown) {
      if (err instanceof ApiError || hasCodeAndMessage(err)) this.error(err.code, err.message)
      else this.error('internal')
    },
    dismiss(id: number) {
      this.items = this.items.filter((t) => t.id !== id)
    },
  },
})
