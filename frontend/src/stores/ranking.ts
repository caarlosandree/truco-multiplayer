import { defineStore } from 'pinia'
import { socialApi, type Metric, type Period, type RankingEntry, type RankingResult } from '@/api/social'
import { useToastsStore } from './toasts'

export type { Metric, Period, RankingEntry, RankingResult }

export type RankingStatus = 'idle' | 'loading' | 'ok' | 'error'

let seq = 0

export const useRankingStore = defineStore('ranking', {
  state: () => ({
    period: 'all' as Period,
    metric: 'wins' as Metric,
    result: null as RankingResult | null,
    status: 'idle' as RankingStatus,
  }),
  actions: {
    /** Carrega o filtro pedido; resposta ou erro de um pedido que já foi superado por outro é descartado. */
    async load(period: Period, metric: Metric) {
      this.period = period
      this.metric = metric
      this.result = null
      this.status = 'loading'
      const id = ++seq
      try {
        const res = await socialApi.ranking(period, metric)
        if (id !== seq) return
        this.result = res
        this.status = 'ok'
      } catch (err) {
        if (id !== seq) return
        this.status = 'error'
        useToastsStore().fromError(err)
      }
    },
  },
})
