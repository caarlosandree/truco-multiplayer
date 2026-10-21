import { endReasonText, errorText, nextStake, signalText, stakeName, SIGNALS } from './labels'

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
