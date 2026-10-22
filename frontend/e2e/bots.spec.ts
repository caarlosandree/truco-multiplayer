import { expect, test } from '@playwright/test'
import { actOnce, newPlayer } from './helpers'

// Partida solo: o humano cria a sala, começa com bots e joga só a primeira carta. Depois de 3 timeouts
// (TIMER_PLAY_SECONDS=5 → ~15 s) o bot assume o assento dele e age a cada ~4,75 s (janela curta de ausente
// menos 250 ms); a partida inteira leva alguns minutos, por isso o timeout generoso.
test('partida solo com bots vai até o fim, fora do ranking', async ({ browser }) => {
  test.setTimeout(12 * 60_000)
  const p = await newPlayer(browser, 'Solo')

  await p.locator('[data-abrir="criar"]').click()
  await p.locator('[data-criar="private"]').click()
  await expect(p.getByRole('switch', { name: 'Estou pronto' })).toBeVisible()
  await p.locator('[data-acao="comecar-com-bots"]').click()

  // mesa: três selos de bot (parceiro e adversários)
  await expect(p.locator('[data-mao]')).toBeVisible()
  await expect(p.locator('[data-selo-bot]')).toHaveCount(3)

  // única jogada humana: espera a vez e joga uma carta; o resto fica por conta do bot que assume
  await expect.poll(() => actOnce(p), { timeout: 60_000, intervals: [200] }).toBe(true)

  const fim = p.getByRole('dialog', { name: 'Fim da partida' })
  await expect(fim).toBeVisible({ timeout: 10 * 60_000 })
  await expect(fim.locator('[data-sem-ranking]')).toContainText('Partida com bots — não conta no ranking.')

  // revanche: volta à sala de espera com os três bots sentados
  await fim.getByRole('button', { name: 'Revanche' }).click()
  await expect(p.getByRole('switch', { name: 'Estou pronto' })).toBeVisible()
  await expect(p.locator('[data-selo-bot]')).toHaveCount(4) // 3 assentos + aviso "Partidas com bots não contam no ranking"
  await expect(p.locator('[data-aviso-bots]')).toBeVisible()
})
