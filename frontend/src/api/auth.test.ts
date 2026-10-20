import { authApi } from './auth'

it('usa os caminhos e corpos do backend', async () => {
  const calls: [string, string | undefined][] = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: RequestInit) => {
      calls.push([`${init.method} ${url}`, init.body as string | undefined])
      return new Response(JSON.stringify({ user: { id: '1' } }), { status: 200 })
    }),
  )
  await authApi.login('ana', 'segredo-longo')
  await authApi.verifyEmail('tok')
  await authApi.resetPassword('tok', 'nova-senha-123')
  await authApi.updateMe('Ana')
  await authApi.changePassword('a', 'b')
  expect(calls).toEqual([
    ['POST /api/auth/login', '{"login":"ana","password":"segredo-longo"}'],
    ['POST /api/auth/verify-email', '{"token":"tok"}'],
    ['POST /api/auth/password/reset', '{"token":"tok","password":"nova-senha-123"}'],
    ['PATCH /api/me', '{"displayName":"Ana"}'],
    ['POST /api/me/password', '{"currentPassword":"a","newPassword":"b"}'],
  ])
})
