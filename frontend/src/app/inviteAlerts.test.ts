import { createPinia, setActivePinia } from 'pinia'
import { installInviteAlerts } from './inviteAlerts'
import { useInvitesStore } from '@/stores/invites'
import type { InviteReceivedPayload } from '@/protocol/messages'

const convite = (inviteId: string, sala = 'Mesa do RH'): InviteReceivedPayload => ({
  inviteId,
  room: { id: 'r1', name: sala, visibility: 'private' },
  from: { displayName: 'Ana' },
  freeSeats: 2,
})

let stopAlerts: () => void
beforeEach(() => {
  vi.useFakeTimers()
  setActivePinia(createPinia())
  document.title = 'Truco'
})
afterEach(() => {
  stopAlerts?.()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

it('aba oculta: toca o som, pisca o título e notifica uma vez por convite', async () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  const play = vi.fn()
  const notify = vi.fn()
  stopAlerts = installInviteAlerts({ hidden: () => true, play, notify })
  const s = useInvitesStore()
  s.onReceived(convite('a'))
  expect(play).toHaveBeenCalledWith('invite')
  expect(notify).toHaveBeenCalledTimes(1)
  expect(notify.mock.calls[0]?.[0]).toMatchObject({ inviteId: 'a' })
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Convite: Mesa do RH')
  // reenvio na reconexão (mesmo id) não repete nada
  s.onReceived(convite('a'))
  expect(play).toHaveBeenCalledTimes(1)
  expect(notify).toHaveBeenCalledTimes(1)
})

it('aba em foco: só o som', () => {
  const play = vi.fn()
  const notify = vi.fn()
  vi.spyOn(document, 'hasFocus').mockReturnValue(true)
  stopAlerts = installInviteAlerts({ hidden: () => false, play, notify })
  useInvitesStore().onReceived(convite('a'))
  expect(play).toHaveBeenCalledWith('invite')
  expect(notify).not.toHaveBeenCalled()
  vi.advanceTimersByTime(3000)
  expect(document.title).toBe('Truco')
})

it('o título para quando o convite sai (resposta ou expiração)', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  stopAlerts = installInviteAlerts({ hidden: () => true, play: vi.fn(), notify: vi.fn() })
  const s = useInvitesStore()
  s.onReceived(convite('a'))
  vi.advanceTimersByTime(1000)
  expect(document.title).toBe('Convite: Mesa do RH')
  s.onExpired({ inviteId: 'a' })
  expect(document.title).toBe('Truco')
  vi.advanceTimersByTime(5000)
  expect(document.title).toBe('Truco')
})

it('o aviso que volta (invitee_playing) não alerta de novo', () => {
  const play = vi.fn()
  stopAlerts = installInviteAlerts({ hidden: () => false, play, notify: vi.fn() })
  const s = useInvitesStore()
  s.onReceived(convite('a'))
  s.received = []
  s.onReceived(convite('a'))
  expect(play).toHaveBeenCalledTimes(1)
})

it('clicar na notificação foca a janela e o Entrar do aviso', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  const focar = vi.spyOn(window, 'focus').mockImplementation(() => {})
  const notify = vi.fn()
  stopAlerts = installInviteAlerts({ hidden: () => true, play: vi.fn(), notify })
  useInvitesStore().onReceived(convite('a'))
  document.body.innerHTML = '<div data-invite-id="a"><button data-acao="entrar">Entrar</button></div>'
  const onClick = notify.mock.calls[0]?.[1] as () => void
  onClick()
  expect(focar).toHaveBeenCalled()
  expect(document.activeElement).toBe(document.querySelector('[data-acao="entrar"]'))
})

it('fecha a notificação do sistema quando o convite expira ou é respondido', () => {
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  const fechar = vi.fn()
  const notify = vi.fn(() => ({ close: fechar }))
  stopAlerts = installInviteAlerts({ hidden: () => true, play: vi.fn(), notify })
  const s = useInvitesStore()
  s.onReceived(convite('a'))
  s.onReceived(convite('b'))
  s.onExpired({ inviteId: 'a' })
  expect(fechar).toHaveBeenCalledTimes(1)
  s.received = []
  expect(fechar).toHaveBeenCalledTimes(2)
})
