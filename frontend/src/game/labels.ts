import type { Event, HandKind, Phase, VoteChoice } from '@/protocol/engine'
import type { EndReason, SignalID } from '@/protocol/messages'

const ERRORS: Record<string, string> = {
  // HTTP / contas
  network: 'Sem conexão com o servidor.',
  internal: 'Erro no servidor, tente de novo.',
  invalid_request: 'Requisição inválida.',
  validation_failed: 'Verifique os campos destacados.',
  invalid_credentials: 'Usuário ou senha incorretos.',
  email_not_verified: 'Confirme seu e-mail antes de entrar.',
  account_disabled: 'Esta conta está desativada.',
  password_reset_required: 'Você precisa redefinir sua senha. Use "Esqueci minha senha".',
  username_taken: 'Este nome de usuário já está em uso.',
  email_domain_not_allowed: 'O cadastro só é permitido com o e-mail da empresa.',
  invalid_token: 'Link inválido ou expirado.',
  unauthenticated: 'Faça login para continuar.',
  too_many_attempts: 'Muitas tentativas. Aguarde um pouco e tente de novo.',
  forbidden_origin: 'Origem da requisição não permitida.',
  // Sala / lobby
  bad_request: 'Ação inválida.',
  already_in_room: 'Você já está em uma sala.',
  room_full: 'A sala está cheia.',
  room_not_found: 'Sala não encontrada.',
  seat_taken: 'Esse lugar já está ocupado.',
  not_host: 'Só o anfitrião pode fazer isso.',
  not_in_room: 'Você não está em uma sala.',
  match_in_progress: 'A partida está em andamento.',
  forfeit_unavailable: 'O W.O. não está disponível agora.',
  too_many_rooms: 'O servidor atingiu o limite de salas. Tente mais tarde.',
  rate_limited: 'Calma! Espere um pouco antes de repetir.',
  // Regras do engine
  invalid_phase: 'Essa ação não vale nesta fase.',
  not_your_turn: 'Não é a sua vez.',
  invalid_slot: 'Carta inválida.',
  invalid_choice: 'Escolha inválida.',
  illegal_raise: 'Não dá para aumentar agora.',
  already_voted: 'Você já votou.',
  not_voting_team: 'Sua dupla não está votando.',
  covered_not_allowed: 'Não é permitido jogar coberta agora.',
  burn_not_four: 'Só dá para queimar cartas 4.',
  game_over: 'A partida já terminou.',
  already_decided: 'Você já decidiu.',
  invalid_seat: 'Lugar inválido.',
}

export function errorText(code: string, serverMessage?: string): string {
  return ERRORS[code] ?? serverMessage ?? 'Algo deu errado. Tente de novo.'
}

const PHASES: Record<Phase, string> = {
  waiting: 'Aguardando',
  cut_choice: 'Corte',
  dealer_choice: 'Escolha do pé',
  burn_fours: 'Queima dos 4',
  eleven_decision: 'Mão de 11',
  playing: 'Jogando',
  awaiting_raise_response: 'Resposta ao aumento',
  hand_end: 'Fim da mão',
  game_over: 'Fim de jogo',
}

export function phaseText(phase: Phase): string {
  return PHASES[phase] ?? phase
}

export function handKindText(kind: HandKind | undefined): string {
  if (kind === 'eleven') return 'Mão de 11'
  if (kind === 'iron') return 'Mão de Ferro'
  return 'Mão normal'
}

const STAKE_NAMES: Record<number, string> = { 3: 'Truco', 6: 'Seis', 9: 'Nove', 12: 'Doze' }
const NEXT_STAKE: Record<number, number> = { 1: 3, 3: 6, 6: 9, 9: 12 }

export function stakeName(stake: number): string {
  return STAKE_NAMES[stake] ?? String(stake)
}

export function nextStake(stake: number): number | null {
  return NEXT_STAKE[stake] ?? null
}

const VOTES: Record<VoteChoice, string> = { accept: 'Aceito', run: 'Corro', raise: 'Aumento', play: 'Jogo' }

export function voteText(choice: VoteChoice): string {
  return VOTES[choice] ?? choice
}

export const SIGNALS: readonly { id: SignalID; text: string }[] = [
  { id: 'truco', text: 'Truco!' },
  { id: 'boa', text: 'Boa!' },
  { id: 'vamos', text: 'Vamos!' },
  { id: 'calma', text: 'Calma…' },
  { id: 'desculpa', text: 'Desculpa!' },
  { id: 'valeu', text: 'Valeu!' },
]

export function signalText(id: SignalID): string {
  return SIGNALS.find((s) => s.id === id)?.text ?? id
}

const END_REASONS: Record<EndReason, string> = {
  game_over: 'Fim de jogo',
  forfeit: 'Encerrada por W.O.',
  aborted: 'Partida abortada pelo administrador',
  abandoned: 'Partida abandonada',
}

export function endReasonText(reason: EndReason): string {
  return END_REASONS[reason] ?? reason
}

export function eventAnnouncement(ev: Event, nameOf: (seat: number) => string): string | null {
  const who = ev.seat !== undefined ? nameOf(ev.seat) : ev.team !== undefined ? `A dupla ${ev.team + 1}` : 'Alguém'
  switch (ev.kind) {
    case 'raise_requested':
      return `${who} pediu ${stakeName(ev.stake ?? 0)}!`
    case 'raise_resolved':
      return ev.choice === 'run' ? 'A dupla correu.' : `Aceito: a mão vale ${ev.stake ?? ''}.`
    case 'raise_punished':
      return 'Pedido proibido: a dupla perdeu a partida.'
    case 'round_ended':
      return ev.team === undefined ? 'Rodada empatada.' : `Rodada para a dupla ${ev.team + 1}.`
    case 'hand_ended':
      return ev.score ? `Fim da mão. Placar ${ev.score[0]} a ${ev.score[1]}.` : 'Fim da mão.'
    case 'vira_revealed':
      return 'Vira revelada.'
    default:
      return null
  }
}
