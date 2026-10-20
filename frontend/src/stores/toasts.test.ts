import { createPinia, setActivePinia } from 'pinia'
import { useToastsStore } from './toasts'
import { ApiError } from '@/api/http'
import { WsSendError } from '@/ws/client'
import { errorText } from '@/game/labels'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.useFakeTimers()
})
afterEach(() => vi.useRealTimers())

it('traduz o código e some sozinho em 5 s', () => {
  const toasts = useToastsStore()
  toasts.error('room_full')
  expect(toasts.items.map((t) => t.text)).toEqual(['A sala está cheia.'])
  vi.advanceTimersByTime(5000)
  expect(toasts.items).toEqual([])
})

it('fromError mapeia ApiError, erro WS e desconhecidos', () => {
  const toasts = useToastsStore()
  toasts.fromError(new ApiError(409, 'room_full', 'x'))
  toasts.fromError(new WsSendError('seat_taken', 'x'))
  toasts.fromError({ code: 'room_not_found', message: 'x' })
  toasts.fromError(new Error('boom'))
  toasts.fromError('texto')
  expect(toasts.items.map((t) => t.text)).toEqual([
    errorText('room_full'),
    errorText('seat_taken'),
    errorText('room_not_found'),
    errorText('internal'),
    errorText('internal'),
  ])
  expect(toasts.items.every((t) => t.kind === 'error')).toBe(true)
})

it('fromError usa a mensagem do servidor quando o código é desconhecido', () => {
  const toasts = useToastsStore()
  toasts.fromError(new ApiError(400, 'codigo_novo', 'Mensagem do servidor'))
  expect(toasts.items[0]?.text).toBe('Mensagem do servidor')
})
