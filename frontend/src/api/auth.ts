import { request } from './http'

export interface User {
  id: string
  username: string
  email: string
  displayName: string
  isAdmin: boolean
  createdAt: string
}

export interface RegisterInput {
  username: string
  email: string
  displayName: string
  password: string
}

type UserBody = { user: User }

export const authApi = {
  register: (input: RegisterInput) => request<undefined>('POST', '/api/auth/register', input),
  verifyEmail: (token: string) => request<undefined>('POST', '/api/auth/verify-email', { token }),
  resendVerification: (email: string) => request<undefined>('POST', '/api/auth/resend-verification', { email }),
  login: async (login: string, password: string) => (await request<UserBody>('POST', '/api/auth/login', { login, password })).user,
  logout: () => request<undefined>('POST', '/api/auth/logout'),
  logoutAll: () => request<undefined>('POST', '/api/auth/logout-all'),
  forgotPassword: (email: string) => request<undefined>('POST', '/api/auth/password/forgot', { email }),
  resetPassword: (token: string, password: string) => request<undefined>('POST', '/api/auth/password/reset', { token, password }),
  me: async () => (await request<UserBody>('GET', '/api/me')).user,
  updateMe: async (displayName: string) => (await request<UserBody>('PATCH', '/api/me', { displayName })).user,
  changePassword: (currentPassword: string, newPassword: string) =>
    request<undefined>('POST', '/api/me/password', { currentPassword, newPassword }),
}
