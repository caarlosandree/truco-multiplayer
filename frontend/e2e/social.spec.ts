import { expect, test, type Page } from '@playwright/test'
import { createAccount, fazerLogin, inviteLink, loginAs, newPlayer } from './helpers'

async function criarSalaPrivada(page: Page) {
  await page.locator('[data-abrir="criar"]').click()
  await page.locator('[data-criar="private"]').click()
  await expect(page.getByTestId('codigo')).toBeVisible()
}

test('convite no app: quem está online recebe o aviso, entra e aparece como aceito', async ({ browser }) => {
  const a = await newPlayer(browser, 'Ana')
  const contaB = await createAccount(browser, 'Bia')
  const b = await loginAs(browser, contaB)
  await expect(b.getByRole('heading', { name: 'Salas', exact: true })).toBeVisible()
  const bUsername = contaB.username

  await criarSalaPrivada(a)
  await a.locator('[data-acao="convidar"]').click()
  const online = a.getByRole('region', { name: 'Online' })
  const convidarB = online.locator(`[data-convidar="${bUsername}"]`)
  await expect(convidarB).toBeVisible()
  await convidarB.click()

  const aviso = b.getByText('te chamou pra mesa!')
  await expect(aviso).toBeVisible()
  await b.locator('[data-acao="entrar"]').click()

  await expect(b.getByRole('switch', { name: 'Estou pronto' })).toBeVisible()
  await expect(a.locator('[data-assento]').filter({ hasText: 'Bia' })).toBeVisible()
  const convites = a.getByRole('region', { name: 'Convites' })
  await expect(convites.locator(`[data-convite="${bUsername}"]`)).toContainText('aceitou')
})

test('convite por e-mail: quem está offline entra pelo link depois de fazer login', async ({ browser }) => {
  const a = await newPlayer(browser, 'Ana')
  const c = await createAccount(browser, 'Caio')

  await criarSalaPrivada(a)
  await a.locator('[data-acao="convidar"]').click()
  await a.getByLabel('Buscar colega por nome ou usuário').fill(c.username)
  const convidarC = a.locator(`[data-convidar="${c.username}"]`)
  await expect(convidarC).toBeVisible()
  await convidarC.click()

  const convites = a.getByRole('region', { name: 'Convites' })
  await expect(convites.locator(`[data-convite="${c.username}"]`)).toContainText('e-mail enviado')

  const link = await inviteLink(c.email)
  // deslogado: o link manda para /entrar com o destino guardado; o login volta ao link e senta
  const page = await (await browser.newContext({ extraHTTPHeaders: { 'X-Forwarded-For': c.ip } })).newPage()
  await page.goto(link)
  await expect(page).toHaveURL(/\/entrar\?volta=/)
  await fazerLogin(page, c)
  await expect(page.getByRole('switch', { name: 'Estou pronto' })).toBeVisible()
  await expect(a.locator('[data-assento]').filter({ hasText: 'Caio' })).toBeVisible()
  await expect(convites.locator(`[data-convite="${c.username}"]`)).toContainText('aceitou')
})
