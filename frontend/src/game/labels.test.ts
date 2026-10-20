import { endReasonText, errorText, eventAnnouncement, nextStake, signalText, stakeName, SIGNALS } from './labels'

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

it('anuncia pedidos de aumento e ignora eventos internos', () => {
  const nome = (s: number) => ['Ana', 'Bia', 'Caio', 'Duda'][s]!
  expect(eventAnnouncement({ kind: 'raise_requested', seat: 1, stake: 3 }, nome)).toBe('Bia pediu Truco!')
  // contra-aumento (voto "raise") vem sem seat, só com a dupla
  expect(eventAnnouncement({ kind: 'raise_requested', team: 0, stake: 6 }, nome)).toBe('A dupla 1 pediu Seis!')
  expect(eventAnnouncement({ kind: 'cut_offered', seat: 1 }, nome)).toBeNull()
})
