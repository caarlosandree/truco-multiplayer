import { createPinia, setActivePinia } from 'pinia'
import { askNotificationPermissionOnce, showInviteNotification } from './notifications'
import { usePrefsStore } from '@/stores/prefs'
import type { InviteReceivedPayload } from '@/protocol/messages'

const inv: InviteReceivedPayload = {
  inviteId: 'a', room: { id: 'r', name: 'Mesa do RH', visibility: 'private' }, from: { displayName: 'Ana' }, freeSeats: 2,
}

let permission: string
let requestPermission: ReturnType<typeof vi.fn<(...a: unknown[]) => Promise<string>>>
let criadas: { title: string; opts: NotificationOptions; close: ReturnType<typeof vi.fn>; onclick: (() => void) | null }[]

beforeEach(() => {
  setActivePinia(createPinia())
  permission = 'default'
  criadas = []
  requestPermission = vi.fn<(...a: unknown[]) => Promise<string>>(() => Promise.resolve("granted"))
  class FakeNotification {
    static get permission() { return permission }
    static requestPermission = (...a: unknown[]) => requestPermission(...a)
    close = vi.fn()
    onclick: (() => void) | null = null
    constructor(public title: string, public opts: NotificationOptions) {
      criadas.push(this as never)
    }
  }
  vi.stubGlobal('Notification', FakeNotification)
})
afterEach(() => vi.unstubAllGlobals())

describe('askNotificationPermissionOnce', () => {
  it('marca notifyAsked ANTES de pedir e a segunda chamada não pede de novo', async () => {
    const prefs = usePrefsStore()
    let marcadoAoPedir: boolean | null = null
    requestPermission.mockImplementation(() => {
      marcadoAoPedir = prefs.notifyAsked
      return Promise.resolve('denied')
    })
    await askNotificationPermissionOnce()
    expect(requestPermission).toHaveBeenCalledTimes(1)
    expect(marcadoAoPedir).toBe(true)
    await askNotificationPermissionOnce()
    expect(requestPermission).toHaveBeenCalledTimes(1)
  })

  it('não pede com a preferência desligada', async () => {
    usePrefsStore().setNotifications(false)
    await askNotificationPermissionOnce()
    expect(requestPermission).not.toHaveBeenCalled()
  })

  it('não pede se já foi perguntado', async () => {
    usePrefsStore().markNotifyAsked()
    await askNotificationPermissionOnce()
    expect(requestPermission).not.toHaveBeenCalled()
  })

  it('não pede com a permissão já negada', async () => {
    permission = 'denied'
    await askNotificationPermissionOnce()
    expect(requestPermission).not.toHaveBeenCalled()
  })

  it('não pede sem Notification em window', async () => {
    vi.unstubAllGlobals()
    Reflect.deleteProperty(window, 'Notification')
    await expect(askNotificationPermissionOnce()).resolves.toBeUndefined()
    expect(usePrefsStore().notifyAsked).toBe(false)
  })
})

describe('showInviteNotification', () => {
  it('só cria com permissão concedida e preferência ligada', () => {
    showInviteNotification(inv, () => {})
    expect(criadas).toHaveLength(0) // default
    permission = 'granted'
    usePrefsStore().setNotifications(false)
    showInviteNotification(inv, () => {})
    expect(criadas).toHaveLength(0)
    usePrefsStore().setNotifications(true)
    showInviteNotification(inv, () => {})
    expect(criadas).toHaveLength(1)
    expect(criadas[0]?.title).toBe('Ana te chamou pra mesa!')
    expect(criadas[0]?.opts.body).toBe('Mesa do RH')
  })

  it('onclick chama o callback e fecha; o retorno fecha por fora', () => {
    permission = 'granted'
    const cb = vi.fn()
    const h = showInviteNotification(inv, cb)
    criadas[0]?.onclick?.()
    expect(cb).toHaveBeenCalled()
    expect(criadas[0]?.close).toHaveBeenCalledTimes(1)
    h?.close()
    expect(criadas[0]?.close).toHaveBeenCalledTimes(2)
  })
})
