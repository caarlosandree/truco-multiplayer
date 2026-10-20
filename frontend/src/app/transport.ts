import { WsSendError, type Outgoing } from '@/ws/client'

export type Sender = (m: Outgoing) => Promise<void>

let sender: Sender | null = null

export function setSender(s: Sender | null): void {
  sender = s
}

export function send(m: Outgoing): Promise<void> {
  if (!sender) return Promise.reject(new WsSendError('network', 'Sem conexão com o servidor.'))
  return sender(m)
}
