import type { InviteReceivedPayload } from '@/protocol/messages'
import { usePrefsStore } from '@/stores/prefs'

function suportado(): boolean {
  return typeof window !== 'undefined' && 'Notification' in window
}

/**
 * Pede a permissão de notificação do navegador uma única vez, no primeiro gesto de resposta a um convite.
 * Marca `notifyAsked` antes de pedir: recusa (ou fechar o pedido) nunca é pedida de novo.
 */
export async function askNotificationPermissionOnce(): Promise<void> {
  if (!suportado()) return
  const prefs = usePrefsStore()
  if (Notification.permission !== 'default' || !prefs.notifications || prefs.notifyAsked) return
  prefs.markNotifyAsked()
  try {
    await Notification.requestPermission()
  } catch {
    // navegador sem suporte ao pedido: segue sem notificação
  }
}

/** Notificação do sistema para um convite (só com permissão concedida e a preferência ligada). Devolve o handle para fechá-la. */
export function showInviteNotification(inv: InviteReceivedPayload, onClick: () => void): { close(): void } | null {
  if (!suportado() || Notification.permission !== 'granted' || !usePrefsStore().notifications) return null
  try {
    const n = new Notification(`${inv.from.displayName} te chamou pra mesa!`, {
      body: inv.room.name,
      tag: `convite-${inv.inviteId}`,
    })
    n.onclick = () => {
      onClick()
      n.close()
    }
    return n
  } catch {
    // alguns navegadores móveis só criam notificações pelo service worker
    return null
  }
}
