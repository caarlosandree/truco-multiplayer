import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { setSender, type Sender } from '@/app/transport'
import SairPartida from './SairPartida.vue'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'

enableAutoUnmount(afterEach)
let send: ReturnType<typeof vi.fn<Sender>>

function roomState(bots: boolean, ranked: boolean | null) {
  return {
    id: 'r', code: 'ABC234', name: 'Sala', visibility: 'public' as const, host: 0, status: 'playing' as const, matchId: 'm',
    seats: [0, 1, 2, 3].map((s) => ({
      userId: `u${s}`, displayName: ['Ana', 'Bia', 'Caio', 'Duda'][s]!, ready: true, connected: true, absent: false,
      bot: bots && s !== 0, botPlaying: false,
    })),
    yourSeat: 0, ranked,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  useSessionStore().connection = 'open'
  send = vi.fn<Sender>().mockResolvedValue(undefined)
  setSender(send)
})
afterEach(() => {
  setSender(null)
  document.body.innerHTML = ''
})

async function abrir() {
  const w = mount(SairPartida, { attachTo: document.body })
  await w.get('[data-acao="sair-partida"]').trigger('click')
  await flushPromises()
  return w
}

const dialogo = () => document.body.querySelector('[role="dialog"]')

describe('SairPartida', () => {
  it('sozinho com bots: avisa que a partida termina e não conta', async () => {
    useRoomStore().onRoomState(roomState(true, false))
    await abrir()
    expect(dialogo()?.textContent).toContain('A partida termina agora e não conta no ranking.')
    expect(dialogo()?.textContent).not.toContain('bot assume')
  })

  it('com outros humanos em partida que conta: bot assume e a derrota conta', async () => {
    useRoomStore().onRoomState(roomState(false, true))
    await abrir()
    expect(dialogo()?.textContent).toContain('Um bot assume o seu lugar')
    expect(dialogo()?.textContent).toContain('a derrota conta para você no ranking')
  })

  it('cancelar fecha sem enviar nada', async () => {
    useRoomStore().onRoomState(roomState(true, false))
    await abrir()
    document.body.querySelector<HTMLElement>('[data-acao="cancelar-saida"]')!.click()
    await flushPromises()
    expect(dialogo()).toBeNull()
    expect(send).not.toHaveBeenCalled()
  })

  it('confirmar envia leave_room', async () => {
    useRoomStore().onRoomState(roomState(true, false))
    await abrir()
    document.body.querySelector<HTMLElement>('[data-acao="confirmar-saida"]')!.click()
    await flushPromises()
    expect(send).toHaveBeenCalledWith(expect.objectContaining({ type: 'leave_room' }))
  })
})
