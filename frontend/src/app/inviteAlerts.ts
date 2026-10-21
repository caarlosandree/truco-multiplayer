import { watch } from 'vue'
import { sfx } from '@/audio/install'
import type { SoundId } from '@/audio/sfx'
import { startTabTitleBlink } from '@/game/tabTitle'
import { useInvitesStore } from '@/stores/invites'
import { showInviteNotification } from './notifications'

export interface InviteAlertDeps {
  hidden?: () => boolean
  notify?: typeof showInviteNotification
  play?: (id: SoundId) => void
}

const abaOculta = () => document.hidden || !document.hasFocus()

/** Foca o "Entrar" do aviso (ao clicar na notificação do sistema). */
function focarEntrar(inviteId: string) {
  window.focus()
  const alvo = [...document.querySelectorAll<HTMLElement>('[data-invite-id]')].find((e) => e.dataset.inviteId === inviteId)
  alvo?.querySelector<HTMLElement>('[data-acao="entrar"]')?.focus()
}

/**
 * Para cada convite NOVO na store: toca `invite`; com a aba oculta ou sem foco também pisca o título
 * ("Convite: <sala>", compartilhando o intervalo com "Sua vez!") 
 * Reenvio na reconexão (mesmo inviteId) e aviso que volta por `invitee_playing` não alertam de novo.
 * Devolve a função que desfaz a instalação.
 */
export function installInviteAlerts(deps: InviteAlertDeps = {}): () => void {
  const hidden = deps.hidden ?? abaOculta
  const notify = deps.notify ?? showInviteNotification
  const play = deps.play ?? ((id: SoundId) => sfx()?.tocarQuandoLiberado(id))
  const store = useInvitesStore()
  const vistos = new Set<string>(store.received.map((r) => r.inviteId))
  const titulos = new Map<string, () => void>()
  const sistema = new Map<string, { close(): void }>()

  const parar = watch(
    () => store.received.map((r) => r.inviteId),
    (ids) => {
      for (const [id, pararTitulo] of titulos) {
        if (!ids.includes(id)) {
          pararTitulo()
          titulos.delete(id)
          sistema.get(id)?.close()
          sistema.delete(id)
        }
      }
      for (const inv of store.received) {
        if (vistos.has(inv.inviteId)) continue
        vistos.add(inv.inviteId)
        play('invite')
        if (!hidden()) continue
        titulos.set(inv.inviteId, startTabTitleBlink(document.title, `Convite: ${inv.room.name}`))
        const n = notify(inv, () => focarEntrar(inv.inviteId))
        if (n) sistema.set(inv.inviteId, n)
      }
    },
    { flush: 'sync' },
  )
  return () => {
    parar()
    for (const pararTitulo of titulos.values()) pararTitulo()
    titulos.clear()
    for (const n of sistema.values()) n.close()
    sistema.clear()
  }
}
