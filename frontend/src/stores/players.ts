import { defineStore } from 'pinia'
import { ApiError } from '@/api/http'
import { socialApi, type DirectoryEntry, type HistoryItem, type HistoryPage, type Profile } from '@/api/social'

export type { DirectoryEntry, HistoryItem, HistoryPage, Profile }

/** 404 do perfil/histórico (jogador inexistente ou desativado): é estado da página, não erro. */
export function isNotFound(err: unknown): boolean {
  return err instanceof ApiError && err.status === 404
}

export const usePlayersStore = defineStore('players', {
  actions: {
    /** Só a primeira página do diretório; o seletor de convite refina digitando. */
    async searchDirectory(q: string): Promise<DirectoryEntry[]> {
      return (await socialApi.directory(q)).items
    },
    loadProfile(username: string): Promise<Profile> {
      return socialApi.profile(username)
    },
    loadHistory(username: string, cursor?: string): Promise<HistoryPage> {
      return socialApi.history(username, cursor)
    },
  },
})
