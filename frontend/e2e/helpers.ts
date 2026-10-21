import { readFileSync } from 'node:fs'
import { expect, type Browser, type Page } from '@playwright/test'

export const PASSWORD = 'senha-de-teste-e2e-123'

/** Lê o log JSON do servidor e devolve o token de verificação mais recente para o e-mail. */
export async function verificationToken(email: string): Promise<string> {
  const file = process.env.E2E_SERVER_LOG
  if (!file) throw new Error('E2E_SERVER_LOG não definido')
  for (let i = 0; i < 50; i++) {
    const lines = readFileSync(file, 'utf8').split('\n').filter(Boolean).reverse()
    for (const line of lines) {
      try {
        const entry = JSON.parse(line) as { msg?: string; to?: string; text?: string }
        if (entry.msg?.startsWith('e-mail') && entry.to === email) {
          const m = /verificar-email\?token=([A-Za-z0-9_-]+)/.exec(entry.text ?? '')
          if (m?.[1]) return m[1]
        }
      } catch {
        // linha que não é JSON
      }
    }
    await new Promise((r) => setTimeout(r, 200))
  }
  throw new Error(`token de verificação não encontrado para ${email}`)
}

const rand = (n: number) => Math.floor(Math.random() * n)

export type Account = { username: string; email: string; displayName: string; ip: string }

/** Lê do log do servidor o último link /entrar-convite/<token> enviado por e-mail a `email`. */
export async function inviteLink(email: string): Promise<string> {
  const file = process.env.E2E_SERVER_LOG
  if (!file) throw new Error('E2E_SERVER_LOG não definido')
  for (let i = 0; i < 50; i++) {
    const lines = readFileSync(file, 'utf8').split('\n').filter(Boolean).reverse()
    for (const line of lines) {
      try {
        const entry = JSON.parse(line) as { msg?: string; to?: string; text?: string }
        if (entry.msg?.startsWith('e-mail') && entry.to === email) {
          const m = /\/entrar-convite\/[A-Za-z0-9_-]+/.exec(entry.text ?? '')
          if (m) return m[0]
        }
      } catch {
        // linha que não é JSON
      }
    }
    await new Promise((r) => setTimeout(r, 200))
  }
  throw new Error(`link de convite não encontrado para ${email}`)
}

/** Cadastra e verifica a conta, sem deixar ninguém logado (o contexto usado é descartado). */
export async function createAccount(browser: Browser, name: string): Promise<Account> {
  const suffix = Math.random().toString(36).slice(2, 7)
  const username = `${name.toLowerCase()}_${suffix}`
  const email = `${username}@e2e.test`
  // IP próprio por jogador (o servidor roda com TRUST_PROXY): o limite de cadastro é por IP
  const ip = `10.${rand(256)}.${rand(256)}.${1 + rand(254)}`
  const context = await browser.newContext({ extraHTTPHeaders: { 'X-Forwarded-For': ip } })
  try {
    const page = await context.newPage()
    await page.goto('/cadastro')
    await page.getByLabel('Usuário', { exact: true }).fill(username)
    await page.getByLabel('E-mail da empresa').fill(email)
    await page.getByLabel('Nome de exibição').fill(name)
    await page.getByLabel('Senha', { exact: true }).fill(PASSWORD)
    await page.getByLabel('Confirmar senha').fill(PASSWORD)
    await page.getByRole('button', { name: 'Criar conta' }).click()
    await expect(page.getByRole('heading', { name: 'Confira seu e-mail' })).toBeVisible()
    await page.goto(`/verificar-email?token=${await verificationToken(email)}`)
    await expect(page.getByRole('heading', { name: 'E-mail confirmado' })).toBeVisible()
  } finally {
    await context.close()
  }
  return { username, email, displayName: name, ip }
}

/** Contexto novo (com o IP da conta) e login pela tela de entrada; devolve a página no lobby. */
export async function loginAs(browser: Browser, account: Account, destino = '/entrar'): Promise<Page> {
  const page = await (await browser.newContext({ extraHTTPHeaders: { 'X-Forwarded-For': account.ip } })).newPage()
  await page.goto(destino)
  await fazerLogin(page, account)
  return page
}

/** Preenche o formulário de /entrar e espera o lobby ou o destino do `volta`. */
export async function fazerLogin(page: Page, account: Account): Promise<void> {
  await page.getByLabel('Usuário ou e-mail').fill(account.username)
  await page.getByLabel('Senha', { exact: true }).fill(PASSWORD)
  await page.getByRole('button', { name: 'Entrar', exact: true }).click()
}

export async function newPlayer(browser: Browser, name: string): Promise<Page> {
  const account = await createAccount(browser, name)
  const page = await loginAs(browser, account)
  await expect(page.getByRole('heading', { name: 'Salas', exact: true })).toBeVisible()
  return page
}

/**
 * Executa a primeira ação habilitada do painel (corte, "Passar" na queima, "Jogar" na Mão de 11,
 * "Aceitar" no aumento) ou a carta de menor slot da mão; devolve true se agiu.
 */
export async function actOnce(page: Page): Promise<boolean> {
  const candidates = [
    page.locator('[data-painel] [data-corte] button:not([disabled])').first(),
    page.locator('[data-painel] [data-acao="passar"]:not([disabled])'),
    page.locator('[data-painel] [data-voto="play"]:not([disabled])'),
    page.locator('[data-painel] [data-voto="accept"]:not([disabled])'),
    page.locator('[data-mao] button:not([disabled])').first(),
  ]
  for (const c of candidates) {
    if (await c.isVisible().catch(() => false)) {
      try {
        await c.click({ timeout: 2000 })
        return true
      } catch {
        // animação ou re-render no meio: tenta de novo na próxima volta
      }
    }
  }
  return false
}

/** Rótulos (aria-label) das cartas da minha mão, na ordem do DOM. */
export async function handLabels(page: Page): Promise<string[]> {
  return page.locator('[data-mao] button').evaluateAll((els) => els.map((e) => e.getAttribute('aria-label') ?? ''))
}

export async function scores(page: Page): Promise<{ nos: string; eles: string }> {
  return {
    nos: (await page.locator('[data-placar-nos]').innerText()).trim(),
    eles: (await page.locator('[data-placar-eles]').innerText()).trim(),
  }
}
