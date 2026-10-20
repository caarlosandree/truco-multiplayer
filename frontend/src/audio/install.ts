import { Sfx, SOUND_IDS, soundForEvent, type SoundId } from './sfx'
import { usePrefsStore } from '@/stores/prefs'
import { useGameStore, setGameSoundPlayer } from '@/stores/game'
import { setMatchSoundPlayer } from '@/stores/room'

const ogg = import.meta.glob('@/assets/sfx/*.ogg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>
const mp3 = import.meta.glob('@/assets/sfx/*.mp3', { eager: true, query: '?url', import: 'default' }) as Record<string, string>

function urlsFor(): Record<SoundId, string> {
  const useOgg = typeof Audio !== 'undefined' && new Audio().canPlayType('audio/ogg; codecs="vorbis"') !== ''
  const table = useOgg ? ogg : mp3
  const ext = useOgg ? 'ogg' : 'mp3'
  return Object.fromEntries(
    SOUND_IDS.map((id) => {
      const entry = Object.entries(table).find(([path]) => path.endsWith(`/${id}.${ext}`))
      return [id, entry?.[1] ?? '']
    }),
  ) as Record<SoundId, string>
}

let instance: Sfx | null = null

export function sfx(): Sfx | null {
  return instance
}

// Touch não conta como ativação de usuário em todos os navegadores móveis: 'click' e 'touchend' cobrem.
const GESTURES = ['pointerdown', 'keydown', 'click', 'touchend'] as const

export function installSfx(): void {
  const prefs = usePrefsStore()
  const current = new Sfx({
    createContext: () => new AudioContext() as never,
    fetchBuffer: async (url) => {
      const res = await fetch(url)
      if (!res.ok) throw new Error(`som indisponível: ${res.status}`)
      return res.arrayBuffer()
    },
    urls: urlsFor(),
    volume: () => prefs.volume,
    muted: () => prefs.muted,
  })
  instance = current
  const removeGestures = () => {
    for (const ev of GESTURES) window.removeEventListener(ev, unlock)
  }
  const unlock = () => {
    current.unlock().then(
      () => {
        // só desiste dos gestos quando o contexto de fato roda; senão tenta no próximo
        if (current.running) removeGestures()
      },
      () => {
        // falha de desbloqueio: tentativa de novo no próximo gesto
      },
    )
  }
  for (const ev of GESTURES) window.addEventListener(ev, unlock)
  setGameSoundPlayer((ev) => {
    const id = soundForEvent(ev, useGameStore().myTeam)
    if (id) instance?.play(id)
  })
  setMatchSoundPlayer((id) => instance?.play(id))
}
