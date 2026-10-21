import { ApiError } from '@/api/http'
import { formError } from './formError'

it('traduz o código e repassa campos e espera', () => {
  const f = formError(new ApiError(429, 'too_many_attempts', 'x', { login: 'ruim' }, 12))
  expect(f).toEqual({ code: 'too_many_attempts', message: 'Muitas tentativas.', fields: { login: 'ruim' }, retryAfterSec: 12 })
})

it('erro desconhecido vira internal', () => {
  expect(formError(new Error('boom'))).toEqual({ code: 'internal', message: 'Erro no servidor, tente de novo.', fields: {} })
})
