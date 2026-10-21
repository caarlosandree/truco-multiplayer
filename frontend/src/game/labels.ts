import type { HandKind, VoteChoice } from '@/protocol/engine'
import type { EndReason, PresenceEntry, SignalID } from '@/protocol/messages'

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
  invite_not_found: 'Convite não encontrado.',
  invite_expired: 'O convite expirou.',
  invite_not_yours: 'Este convite é para outra pessoa.',
  invitee_playing: 'Você está numa partida.',
  cannot_invite_self: 'Você não pode convidar a si mesmo.',
  user_not_found: 'Usuário não encontrado.',
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

/** Linha de status da pessoa na lista de online: "no lobby", "esperando · Mesa do RH", "jogando · Truco da TI". */
export function presenceText(e: PresenceEntry): string {
  if (e.status === 'lobby') return 'no lobby'
  const where = e.privateRoom ? 'sala privada' : (e.roomName ?? '')
  const verb = e.status === 'waiting' ? 'esperando' : 'jogando'
  return where ? `${verb} · ${where}` : verb
}
