import type { RelativePosition } from '@/game/seats'

/**
 * De que lado da placa o balão aparece. No celular os três adversários ficam na faixa de cima, logo abaixo do
 * placar: o balão desce (se subisse, o placar o cobriria). No desktop o parceiro (em cima) tem o balão embaixo
 * e os demais, em cima. `deslocado`: já há outro balão no assento, este sobe/desce mais um degrau.
 * (Classes escritas por extenso para o Tailwind enxergá-las.)
 */
export const BALAO_LADO: Record<RelativePosition, { balao: string; deslocado: string; rabicho: string }> = {
  bottom: { balao: 'bottom-full mb-3', deslocado: 'bottom-full mb-14', rabicho: '-bottom-1' },
  top: { balao: 'top-full mt-3', deslocado: 'top-full mt-14', rabicho: '-top-1' },
  left: {
    balao: 'max-md:top-full max-md:mt-3 md:bottom-full md:mb-3',
    deslocado: 'max-md:top-full max-md:mt-14 md:bottom-full md:mb-14',
    rabicho: 'max-md:-top-1 md:-bottom-1',
  },
  right: {
    balao: 'max-md:top-full max-md:mt-3 md:bottom-full md:mb-3',
    deslocado: 'max-md:top-full max-md:mt-14 md:bottom-full md:mb-14',
    rabicho: 'max-md:-top-1 md:-bottom-1',
  },
}
