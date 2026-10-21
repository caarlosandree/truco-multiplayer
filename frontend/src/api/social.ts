import type { EndReason } from '@/protocol/messages'
import { request } from './http'

export interface DirectoryEntry {
  username: string
  displayName: string
}

export interface DirectoryPage {
  items: DirectoryEntry[]
  nextCursor?: string
}

export interface MatchPlayer {
  seat: number
  username: string
  displayName: string
}

export type MatchStatus = 'in_progress' | 'finished' | 'abandoned'

export interface HistoryItem {
  id: string
  status: MatchStatus
  team: number
  won: boolean
  winnerTeam: number | null
  score: [number, number] | null
  createdAt: string
  finishedAt: string | null
  players: MatchPlayer[]
  endReason: EndReason
}

export interface HistoryPage {
  items: HistoryItem[]
  nextCursor?: string
}

export interface Partner {
  username: string
  displayName: string
  matches: number
  wins: number
  winRate: number
}

export interface PositionPair {
  wins: number | null
  winrate: number | null
}

export interface Positions {
  all: PositionPair
  month: PositionPair
  week: PositionPair
}

export interface Totals {
  matchesPlayed: number
  wins: number
  losses: number
  winRate: number
  pointsFor: number
  pointsAgainst: number
  handsPlayed: number
  handsWon: number
  raisesCalled: number
  raisesFolded: number
  raisesWonByFold: number
  elevenHands: number
  elevenRuns: number
  elevenWon: number
  ironHands: number
  ironWon: number
  punished: number
  timeouts: number
}

export interface Profile {
  username: string
  displayName: string
  createdAt: string
  stats: Totals
  recentMatches: HistoryItem[]
  partners: Partner[]
  positions: Positions
}

export type Period = 'all' | 'month' | 'week'
export type Metric = 'wins' | 'winrate'

export interface RankingEntry {
  position: number
  username: string
  displayName: string
  matchesPlayed: number
  wins: number
  losses: number
  winRate: number
}

export interface RankingResult {
  items: RankingEntry[]
  me: RankingEntry | null
  minMatches: number
}

function qs(params: Record<string, string | number | undefined>): string {
  const parts = Object.entries(params).flatMap(([k, v]) => (v === undefined || v === '' ? [] : [`${k}=${encodeURIComponent(String(v))}`]))
  return parts.length === 0 ? '' : `?${parts.join('&')}`
}

export const socialApi = {
  directory: (q: string, cursor?: string) => request<DirectoryPage>('GET', `/api/users${qs({ q, cursor })}`),
  profile: (username: string) => request<Profile>('GET', `/api/users/${encodeURIComponent(username)}`),
  history: (username: string, cursor?: string) => request<HistoryPage>('GET', `/api/matches${qs({ user: username, cursor })}`),
  ranking: (period: Period, metric: Metric, limit?: number) => request<RankingResult>('GET', `/api/ranking${qs({ period, metric, limit })}`),
}
