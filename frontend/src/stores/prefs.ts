import { defineStore } from 'pinia'
import type { DeckStyle } from '@/game/cards'

const KEY = 'truco.prefs'

interface Prefs {
  deck: DeckStyle
  volume: number
  muted: boolean
  /** Notificações do navegador para convites (desligar não revoga a permissão; só para de notificar). */
  notifications: boolean
  /** A permissão de notificação já foi pedida uma vez: recusa nunca é pedida de novo. */
  notifyAsked: boolean
}

const DEFAULTS: Prefs = { deck: 'frances', volume: 0.7, muted: false, notifications: true, notifyAsked: false }

function read(): Prefs {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { ...DEFAULTS }
    const data = JSON.parse(raw) as Partial<Record<keyof Prefs, unknown>>
    return {
      deck: data.deck === 'espanhol' || data.deck === 'frances' ? data.deck : DEFAULTS.deck,
      volume: typeof data.volume === 'number' && data.volume >= 0 && data.volume <= 1 ? data.volume : DEFAULTS.volume,
      muted: typeof data.muted === 'boolean' ? data.muted : DEFAULTS.muted,
      notifications: typeof data.notifications === 'boolean' ? data.notifications : DEFAULTS.notifications,
      notifyAsked: typeof data.notifyAsked === 'boolean' ? data.notifyAsked : DEFAULTS.notifyAsked,
    }
  } catch {
    return { ...DEFAULTS }
  }
}

export const usePrefsStore = defineStore('prefs', {
  state: (): Prefs => read(),
  actions: {
    setDeck(deck: DeckStyle) {
      this.deck = deck
      this.save()
    },
    setVolume(volume: number) {
      this.volume = Math.min(1, Math.max(0, volume))
      this.save()
    },
    setMuted(muted: boolean) {
      this.muted = muted
      this.save()
    },
    setNotifications(on: boolean) {
      this.notifications = on
      this.save()
    },
    markNotifyAsked() {
      this.notifyAsked = true
      this.save()
    },
    save() {
      try {
        localStorage.setItem(
          KEY,
          JSON.stringify({
            deck: this.deck,
            volume: this.volume,
            muted: this.muted,
            notifications: this.notifications,
            notifyAsked: this.notifyAsked,
          }),
        )
      } catch {
        // storage indisponível: a preferência vale só nesta aba
      }
    },
  },
})
