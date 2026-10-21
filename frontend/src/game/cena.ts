import { inject, provide, reactive, watch, type InjectionKey } from 'vue'
import type { CardID, Event, TableCardView, Team } from '@/protocol/engine'
import { useGameStore } from '@/stores/game'
import { stakeName } from './labels'

/**
 * O diretor só aplica o `view` ao fim do lote de eventos, então durante a cena o que já aconteceu
 * (carta jogada, vira, placar novo…) vem do próprio evento. Este estado guarda esses adiantamentos e
 * zera quando o `view` novo chega: a partir daí a verdade é o `view`.
 */
export interface Cena {
  /** cartas jogadas durante a cena que o `view` ainda não mostra */
  flights: TableCardView[]
  vira: CardID | undefined
  /** placar já com os pontos da mão que acabou */
  score: number[] | null
  /** valor da mão já aceito */
  stake: number | null
  /** rodada que acabou de fechar (team indefinido = empate) */
  round: { team?: Team } | null
}

const KEY: InjectionKey<Cena> = Symbol('cena')

function vazia(): Cena {
  return { flights: [], vira: undefined, score: null, stake: null, round: null }
}

export function aplicarEvento(cena: Cena, ev: Event): void {
  switch (ev.kind) {
    case 'card_played':
      if (ev.seat !== undefined && !cena.flights.some((f) => f.seat === ev.seat)) {
        cena.flights.push({ seat: ev.seat, card: ev.card, covered: ev.card === undefined })
      }
      break
    case 'vira_revealed':
      cena.vira = ev.card
      break
    case 'hand_ended':
      cena.score = ev.score ?? null
      break
    case 'raise_resolved':
      if (ev.choice !== 'run' && ev.stake !== undefined) cena.stake = ev.stake
      break
    case 'round_ended':
      cena.round = ev.team === undefined ? {} : { team: ev.team }
      break
    default:
      break
  }
}

/** Chamado uma vez pela Mesa; as peças filhas leem com `useCena()`. */
export function provideCena(): Cena {
  const game = useGameStore()
  const cena = reactive<Cena>(vazia())
  watch(
    () => game.view,
    () => Object.assign(cena, vazia()),
  )
  watch(
    () => game.scene.key,
    () => {
      if (game.scene.event) aplicarEvento(cena, game.scene.event)
    },
  )
  provide(KEY, cena)
  return cena
}

export function useCena(): Cena {
  return inject(KEY, () => reactive<Cena>(vazia()), true)
}

/** Texto para a região `aria-live` da mesa (relativo a quem joga: "nós"/"eles"). */
export function anuncioDoEvento(ev: Event, nameOf: (seat: number) => string, myTeam: Team | null): string | null {
  const quem = ev.seat !== undefined ? nameOf(ev.seat) : 'Alguém'
  switch (ev.kind) {
    case 'raise_requested':
      return `${quem} pediu ${stakeName(ev.stake ?? 0)}.`
    case 'raise_resolved':
      return ev.choice === 'run' ? 'Correu.' : 'Aceito.'
    case 'raise_punished':
      return 'Pedido proibido: derrota.'
    case 'round_ended':
      return ev.seat === undefined ? 'Rodada empatada.' : `Rodada ganha por ${nameOf(ev.seat)}.`
    case 'hand_ended': {
      if (ev.team === undefined) return 'Mão sem pontos.'
      const de = myTeam === null ? `da dupla ${ev.team + 1}` : ev.team === myTeam ? 'para nós' : 'para eles'
      return `Mão ${de}, mais ${ev.stake ?? 0}.`
    }
    case 'vira_revealed':
      return 'Vira revelada.'
    default:
      return null
  }
}
