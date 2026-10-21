import { ref } from 'vue'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import { formError, type FormError } from './formError'

/** Reenvio do e-mail de confirmação: estado de envio e, no rate limit, espera com contagem (spec §7.2). */
export function useReenvio() {
  const session = useSessionStore()
  const toasts = useToastsStore()
  const enviando = ref(false)
  const limite = ref<FormError | null>(null)
  const bloqueado = ref(false)

  async function enviar(email: string): Promise<boolean> {
    enviando.value = true
    limite.value = null
    try {
      await session.resendVerification(email)
      return true
    } catch (err) {
      const f = formError(err)
      if (f.code === 'too_many_attempts') {
        limite.value = f
        bloqueado.value = f.retryAfterSec !== undefined
      } else {
        toasts.fromError(err)
      }
      return false
    } finally {
      enviando.value = false
    }
  }

  function fimEspera() {
    bloqueado.value = false
    limite.value = null
  }

  return { enviando, limite, bloqueado, enviar, fimEspera }
}
