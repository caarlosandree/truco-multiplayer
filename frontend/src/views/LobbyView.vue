<script setup lang="ts">
import { ref } from 'vue'
import Botao from '@/components/ui/Botao.vue'
import ListaSalas from '@/components/lobby/ListaSalas.vue'
import CriarSala from '@/components/lobby/CriarSala.vue'
import EntrarComCodigo from '@/components/lobby/EntrarComCodigo.vue'

const criando = ref(false)
const entrando = ref(false)
</script>

<template>
  <div class="mx-auto flex w-full max-w-240 flex-col gap-6 px-4 py-6 md:px-6 md:py-8">
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
          data-abrir="criar"
          @click="criando = true"
        >
          Criar sala
        </Botao>
      </div>
    </div>
    <ListaSalas @criar="criando = true" />
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
