<script setup lang="ts">
import { computed, ref } from 'vue'
import { usePresenceStore } from '@/stores/presence'
import { useLobbyStore } from '@/stores/lobby'
import Botao from '@/components/ui/Botao.vue'
import ListaSalas from '@/components/lobby/ListaSalas.vue'
import CriarSala from '@/components/lobby/CriarSala.vue'
import EntrarComCodigo from '@/components/lobby/EntrarComCodigo.vue'
import OnlineAgora from '@/components/lobby/OnlineAgora.vue'

const lobby = useLobbyStore()
const vazio = computed(() => lobby.loaded && lobby.list.length === 0)
const criando = ref(false)
const entrando = ref(false)

// celular: abas "Salas" / "Online (N)" (padrão WAI); no desktop as duas colunas aparecem lado a lado
const presence = usePresenceStore()
const ABAS = ['salas', 'online'] as const
type Aba = (typeof ABAS)[number]
const aba = ref<Aba>('salas')
const abaEl: Record<Aba, HTMLElement | null> = { salas: null, online: null }

function irPara(a: Aba) {
  aba.value = a
  abaEl[a]?.focus()
}
function teclas(e: KeyboardEvent) {
  const i = ABAS.indexOf(aba.value)
  const alvo =
    e.key === 'ArrowRight' ? ABAS[(i + 1) % ABAS.length]
    : e.key === 'ArrowLeft' ? ABAS[(i - 1 + ABAS.length) % ABAS.length]
    : e.key === 'Home' ? ABAS[0]
    : e.key === 'End' ? ABAS[ABAS.length - 1]
    : undefined
  if (alvo === undefined) return
  e.preventDefault()
  irPara(alvo)
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-300 flex-col gap-6 px-4 py-6 md:px-6 md:py-8">
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
      <h1 class="text-2xl font-bold">
        Salas
      </h1>
      <div class="flex gap-2">
        <Botao
          variant="secondary"
          data-abrir="codigo"
          @click="entrando = true"
        >
          Entrar com código
        </Botao>
        <Botao
          :variant="vazio ? 'secondary' : 'primary'"
          data-abrir="criar"
          @click="criando = true"
        >
          Criar sala
        </Botao>
      </div>
    </div>
    <div
      role="tablist"
      aria-label="Lobby"
      class="flex gap-2 border-b-2 border-line/40 md:hidden"
    >
      <button
        v-for="a in ABAS"
        :id="`aba-${a}`"
        :key="a"
        :ref="(el) => (abaEl[a] = el as HTMLElement | null)"
        type="button"
        role="tab"
        :aria-selected="aba === a"
        :aria-controls="`painel-${a}`"
        :tabindex="aba === a ? 0 : -1"
        class="-mb-0.5 h-11 flex-1 border-b-4 px-3 font-bold"
        :class="aba === a ? 'border-ink text-ink' : 'border-transparent text-ink-muted'"
        @click="aba = a"
        @keydown="teclas"
      >
        {{ a === 'salas' ? 'Salas' : `Online (${presence.count})` }}
      </button>
    </div>
    <div class="grid grid-cols-1 items-start gap-6 md:grid-cols-[minmax(0,1fr)_18rem]">
      <div
        id="painel-salas"
        role="tabpanel"
        aria-labelledby="aba-salas"
        :class="aba === 'salas' ? '' : 'max-md:hidden'"
      >
        <ListaSalas @criar="criando = true" />
      </div>
      <div
        id="painel-online"
        role="tabpanel"
        aria-labelledby="aba-online"
        :class="aba === 'online' ? '' : 'max-md:hidden'"
      >
        <OnlineAgora />
      </div>
    </div>
    <CriarSala
      :open="criando"
      @close="criando = false"
    />
    <EntrarComCodigo
      :open="entrando"
      @close="entrando = false"
    />
  </div>
</template>
