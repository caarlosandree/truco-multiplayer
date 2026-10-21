import { defineStore } from 'pinia'
import { socialApi, type DirectoryEntry } from '@/api/social'

export type { DirectoryEntry }

export const usePlayersStore = defineStore('players', {
  actions: {
    /** Só a primeira página do diretório; o seletor de convite refina digitando. */
    async searchDirectory(q: string): Promise<DirectoryEntry[]> {
      return (await socialApi.directory(q)).items
    },
  },
})
