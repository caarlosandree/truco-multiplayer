<script setup lang="ts">
import { SIGNAL_LEGEND } from '@/game/labels'
import Botao from '@/components/ui/Botao.vue'
import Modal from '@/components/ui/Modal.vue'

/**
 * Legenda dos sinais: a convenção da mesa (spec dos bots §5.6). Os bots mandam e entendem os sinais assim; os
 * humanos podem seguir a mesma combinação. Foco preso e Esc ficam por conta do Modal.
 */
defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Modal
    :open="open"
    title="O que cada sinal significa"
    @close="emit('close')"
  >
    <p class="text-base text-ink-muted">
      A convenção da mesa. Os bots mandam e entendem os sinais do parceiro assim; os dos adversários eles leem, mas
      dão pouco peso.
    </p>
    <div class="mt-4 overflow-x-auto">
      <table
        data-legenda-sinais
        class="w-full border-collapse text-left text-sm"
      >
        <caption class="sr-only">
          Sinais, quando o bot manda e como ele entende o do parceiro
        </caption>
        <thead>
          <tr class="border-b-2 border-line/60 align-bottom">
            <th
              scope="col"
              class="py-2 pr-3 font-bold"
            >
              Sinal
            </th>
            <th
              scope="col"
              class="py-2 pr-3 font-bold"
            >
              O bot manda quando
            </th>
            <th
              scope="col"
              class="py-2 font-bold"
            >
              Do parceiro, ele entende
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="l in SIGNAL_LEGEND"
            :key="l.sinais.join()"
            class="border-t border-line/40 align-top"
          >
            <th
              scope="row"
              class="py-2.5 pr-3 font-bold whitespace-nowrap"
            >
              <span
                v-for="x in l.sinais"
                :key="x"
                class="block"
              >{{ x }}</span>
            </th>
            <td class="py-2.5 pr-3 text-pretty">
              {{ l.manda }}
            </td>
            <td class="py-2.5 text-pretty">
              {{ l.entende }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="mt-4 text-sm text-ink-muted">
      Vale o limite de sempre: um sinal a cada 3 segundos.
    </p>
    <div class="mt-6 flex justify-end">
      <Botao
        data-autofocus
        @click="emit('close')"
      >
        Entendi
      </Botao>
    </div>
  </Modal>
</template>
