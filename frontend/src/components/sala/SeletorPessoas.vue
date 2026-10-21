<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Seat } from '@/protocol/engine'
import { usePlayersStore, type DirectoryEntry } from '@/stores/players'
import type { PresenceEntry } from '@/protocol/messages'
import { presenceText } from '@/game/labels'
import { useInvitesStore } from '@/stores/invites'
import { usePresenceStore } from '@/stores/presence'
import { useRoomStore } from '@/stores/room'
import { useSessionStore } from '@/stores/session'
import { useToastsStore } from '@/stores/toasts'
import Botao from '@/components/ui/Botao.vue'
import Modal from '@/components/ui/Modal.vue'

const props = defineProps<{ open: boolean; seat?: Seat }>()
const emit = defineEmits<{ close: [] }>()

const DEBOUNCE_MS = 250

const presence = usePresenceStore()
const room = useRoomStore()
const session = useSessionStore()
const invites = useInvitesStore()
const players = usePlayersStore()
const toasts = useToastsStore()

const query = ref('')
const results = ref<DirectoryEntry[]>([])
const searching = ref(false)
const searchFailed = ref(false)
const sendingTo = ref<string | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined
let ticket = 0

const seatedIds = computed(() => new Set((room.room?.seats ?? []).flatMap((s) => (s ? [s.userId] : []))))
const meId = computed(() => session.user?.id ?? null)
const meName = computed(() => session.user?.username ?? null)

/** Online: a presença menos eu e quem já está sentado. */
const online = computed(() => presence.list.filter((e) => e.userId !== meId.value && !seatedIds.value.has(e.userId)))
const onlineNames = computed(() => new Set(presence.list.map((e) => e.username)))
/** Outros colegas: o diretório sem mim e sem quem já aparece em Online (quem está sentado está online, então também sai daqui). */
const others = computed(() => results.value.filter((d) => d.username !== meName.value && !onlineNames.value.has(d.username)))

const title = computed(() => (props.seat === undefined ? 'Convidar pra mesa' : `Convidar pro lugar ${props.seat + 1}`))
const trimmed = computed(() => query.value.trim())

function statusText(e: PresenceEntry): string {
  return e.status === 'playing' ? 'jogando — pode recusar' : presenceText(e)
}

watch(query, () => {
  clearTimeout(timer)
  ticket++
  searchFailed.value = false
  if (trimmed.value === '') {
    results.value = []
    searching.value = false
    return
  }
  searching.value = true
  timer = setTimeout(() => void buscar(trimmed.value), DEBOUNCE_MS)
})

async function buscar(q: string) {
  const mine = ++ticket
  try {
    const found = await players.searchDirectory(q)
    if (mine === ticket) results.value = found
  } catch (err) {
    if (mine !== ticket) return
    results.value = []
    searchFailed.value = true
    toasts.fromError(err)
  } finally {
    if (mine === ticket) searching.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) return
    clearTimeout(timer)
    ticket++
    query.value = ''
    results.value = []
    searching.value = false
    searchFailed.value = false
    sendingTo.value = null
  },
)
onBeforeUnmount(() => clearTimeout(timer))

async function convidar(username: string) {
  if (sendingTo.value !== null) return
  sendingTo.value = username
  try {
    await invites.invite(username, props.seat)
    emit('close')
  } catch (err) {
    toasts.fromError(err)
  } finally {
    sendingTo.value = null
  }
}

const BOTAO =
  'inline-flex h-11 shrink-0 items-center justify-center rounded-botao-2 px-3 text-sm font-bold text-ink inset-ring-2 inset-ring-(--botao-sec-borda) hover:bg-(--botao-sec-bg-hover) active:translate-y-(--botao-pressao-y) disabled:cursor-not-allowed disabled:opacity-(--botao-desabilitado-opacidade)'
const LISTA = 'max-h-[min(14rem,28dvh)] overflow-y-auto rounded-painel bg-surface-2'
const LINHA = 'flex items-center gap-3 border-t-2 border-line/40 px-4 py-2 first:border-t-0'
</script>

<template>
  <Modal
    :open="props.open"
    :title="title"
    @close="emit('close')"
  >
    <section aria-labelledby="sel-online">
      <h3
        id="sel-online"
        class="mb-2 text-base font-bold"
      >
        Online
      </h3>
      <p
        v-if="online.length === 0"
        class="text-sm text-ink-muted"
      >
        Ninguém online agora. Busque um colega abaixo.
      </p>
      <ul
        v-else
        :class="LISTA"
      >
        <li
          v-for="e in online"
          :key="e.userId"
          :data-pessoa="e.username"
          :class="LINHA"
        >
          <div class="min-w-0 flex-1">
            <p class="font-bold [overflow-wrap:anywhere]">
              {{ e.displayName }}
            </p>
            <p class="text-sm text-pretty text-ink-muted [overflow-wrap:anywhere]">
              {{ statusText(e) }}
            </p>
          </div>
          <button
            type="button"
            :data-convidar="e.username"
            :aria-label="`Convidar ${e.displayName}`"
            :disabled="sendingTo !== null"
            :class="BOTAO"
            @click="convidar(e.username)"
          >
            Convidar
          </button>
        </li>
      </ul>
    </section>

    <section
      aria-labelledby="sel-outros"
      class="mt-5"
    >
      <h3
        id="sel-outros"
        class="mb-2 text-base font-bold"
      >
        Outros colegas
      </h3>
      <label
        for="sel-busca"
        class="sr-only"
      >Buscar colega por nome ou usuário</label>
      <input
        id="sel-busca"
        v-model="query"
        type="search"
        autocomplete="off"
        placeholder="Buscar por nome ou usuário"
        class="mb-2 h-(--campo-altura) w-full rounded-campo border-2 border-(--campo-borda) bg-(--campo-bg) px-3 text-base text-(--campo-fg) placeholder:text-(--campo-placeholder) [&::-webkit-search-cancel-button]:appearance-none"
      >
      <p
        v-if="trimmed === ''"
        class="text-sm text-ink-muted"
      >
        Digite o nome ou o usuário de quem você quer chamar.
      </p>
      <p
        v-else-if="searching"
        class="text-sm text-ink-muted"
      >
        Buscando…
      </p>
      <p
        v-else-if="searchFailed"
        class="text-sm text-danger"
      >
        Não deu pra buscar agora. Tente de novo.
      </p>
      <p
        v-else-if="others.length === 0"
        class="text-sm text-ink-muted"
      >
        {{ results.length > 0 ? 'Só quem já aparece em Online.' : 'Ninguém encontrado com esse nome.' }}
      </p>
      <ul
        v-else
        :class="LISTA"
      >
        <li
          v-for="d in others"
          :key="d.username"
          :data-pessoa="d.username"
          :class="LINHA"
        >
          <div class="min-w-0 flex-1">
            <p class="font-bold [overflow-wrap:anywhere]">
              {{ d.displayName }}
            </p>
            <p class="text-sm text-ink-muted [overflow-wrap:anywhere]">
              @{{ d.username }}
            </p>
          </div>
          <button
            type="button"
            :data-convidar="d.username"
            :aria-label="`Convidar ${d.displayName}`"
            :disabled="sendingTo !== null"
            :class="BOTAO"
            @click="convidar(d.username)"
          >
            Convidar
          </button>
        </li>
      </ul>
    </section>

    <div class="mt-6 flex justify-end">
      <Botao
        data-acao="fechar-seletor"
        variant="ghost"
        @click="emit('close')"
      >
        Fechar
      </Botao>
    </div>
  </Modal>
</template>
