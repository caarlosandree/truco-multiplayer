import { endReasonText, errorText, inviteStateText, nextStake, signalText, stakeName, SIGNALS } from './labels'

it('traduz códigos conhecidos de HTTP, sala e regra', () => {
  expect(errorText('invalid_credentials')).toBe('Usuário ou senha incorretos.')
  expect(errorText('room_full')).toBe('A sala está cheia.')
  expect(errorText('not_your_turn')).toBe('Não é a sua vez.')
})

it('cai na mensagem do servidor para código desconhecido', () => {
  expect(errorText('codigo_novo', 'Mensagem do servidor.')).toBe('Mensagem do servidor.')
  expect(errorText('codigo_novo')).toBe('Algo deu errado. Tente de novo.')
})

it('nomeia a escada de apostas', () => {
  expect([1, 3, 6, 9, 12].map(nextStake)).toEqual([3, 6, 9, 12, null])
  expect([3, 6, 9, 12].map(stakeName)).toEqual(['Truco', 'Seis', 'Nove', 'Doze'])
})

it('cobre os 6 sinais e os motivos de fim', () => {
  expect(SIGNALS.map((s) => s.id)).toEqual(['truco', 'boa', 'vamos', 'calma', 'desculpa', 'valeu'])
  expect(signalText('valeu')).toBe('Valeu!')
  expect(endReasonText('forfeit')).toBe('Encerrada por W.O.')
})

describe('inviteStateText', () => {
  const base = { inviteId: 'i', username: 'bia', displayName: 'Bia', emailed: false }
  it('um texto por estado', () => {
    expect(inviteStateText({ ...base, state: 'accepted' })).toBe('aceitou')
    expect(inviteStateText({ ...base, state: 'pending' })).toBe('aguardando…')
    expect(inviteStateText({ ...base, state: 'pending', emailed: true })).toBe('e-mail enviado')
    expect(inviteStateText({ ...base, state: 'declined' })).toBe('agora não')
  })
})
