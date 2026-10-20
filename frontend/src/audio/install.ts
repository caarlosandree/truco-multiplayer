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

export function installSfx(): void {
  const prefs = usePrefsStore()
  instance = new Sfx({
    createContext: () => new AudioContext() as never,
    fetchBuffer: async (url) => (await fetch(url)).arrayBuffer(),
    urls: urlsFor(),
    volume: () => prefs.volume,
    muted: () => prefs.muted,
  })
  const unlock = () => {
    void instance?.unlock()
    window.removeEventListener('pointerdown', unlock)
    window.removeEventListener('keydown', unlock)
  }
  window.addEventListener('pointerdown', unlock)
  window.addEventListener('keydown', unlock)
  setGameSoundPlayer((ev) => {
    const id = soundForEvent(ev, useGameStore().myTeam)
    if (id) instance?.play(id)
  })
  setMatchSoundPlayer((id) => instance?.play(id))
}
