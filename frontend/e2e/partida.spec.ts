import { expect, test, type Browser, type Page } from '@playwright/test'
import { actOnce, handLabels, newPlayer, scores } from './helpers'

const fim = (p: Page) => p.locator('[data-acao="revanche"]')

async function playUntilEnd(players: Page[]) {
  const deadline = Date.now() + 8 * 60_000
  while (Date.now() < deadline) {
    if (await fim(players[0] as Page).isVisible().catch(() => false)) return
    let acted = false
    for (const p of players) acted = (await actOnce(p)) || acted
    if (!acted) await (players[0] as Page).waitForTimeout(150)
  }
  throw new Error('partida não terminou')
}

async function setupMatch(browser: Browser): Promise<Page[]> {
  const players: Page[] = []
  for (const n of ['Ana', 'Bia', 'Caio', 'Duda']) players.push(await newPlayer(browser, n))
  const [host, ...rest] = players as [Page, Page, Page, Page]
  await host.locator('[data-abrir="criar"]').click()
  await host.locator('[data-criar="private"]').click()
  const code = (await host.getByTestId('codigo').innerText()).trim()
  for (const p of rest) {
    await p.goto(`/sala/${code.toLowerCase()}`)
    await expect(p.getByRole('switch', { name: 'Estou pronto' })).toBeVisible()
  }
  for (const p of players) await p.getByRole('switch', { name: 'Estou pronto' }).click()
  for (const p of players) await expect(p.locator('[data-mao]')).toBeVisible()
  return players
}

test('quatro jogadores jogam uma partida completa', async ({ browser }) => {
  const players = await setupMatch(browser)
  await playUntilEnd(players)
  for (const p of players) {
    const dialog = p.getByRole('dialog', { name: 'Fim da partida' })
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText(/Vitória|Derrota/)
    await expect(dialog.getByRole('button', { name: 'Revanche' })).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Sair da sala' })).toBeVisible()
  }
})

test('recarregar no meio da partida retoma a mesa', async ({ browser }) => {
  const players = await setupMatch(browser)
  const p = players[1] as Page
  for (let i = 0; i < 12; i++) for (const q of players) await actOnce(q)
  const ler = async () => ({ hand: await handLabels(p), s: await scores(p) })
  // a mesa pode avançar por timer entre a leitura e o reload: repete até uma leitura bater
  let antes = await ler()
  let depois = antes
  for (let tentativa = 0; tentativa < 4; tentativa++) {
    await p.waitForTimeout(500)
    antes = await ler()
    await p.reload()
    await expect(p.locator('[data-mao] button').first()).toBeVisible()
    await expect(p.locator('[data-placar]')).toBeVisible()
    depois = await ler()
    if (JSON.stringify(depois) === JSON.stringify(antes)) break
  }
  expect(antes.hand.length).toBeGreaterThan(0)
  expect(depois.hand).toEqual(antes.hand)
  expect(depois.s).toEqual(antes.s)
  await playUntilEnd(players)
})
