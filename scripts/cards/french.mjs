// Copia as 40 cartas do Vector Playing Cards para os nomes do app.
// Uso: node scripts/cards/french.mjs <pasta-dos-svgs>
import { copyFileSync, readdirSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'

const src = process.argv[2]
if (!src) throw new Error('informe a pasta com os SVGs')
const out = 'frontend/src/assets/cards/frances'
mkdirSync(out, { recursive: true })

const RANKS = { 1: 'ace', 2: '2', 3: '3', 4: '4', 5: '5', 6: '6', 7: '7', 10: 'jack', 11: 'queen', 12: 'king' }
const SUITS = { ouros: 'diamonds', espadas: 'spades', copas: 'hearts', paus: 'clubs' }
const files = readdirSync(src)

for (const [rank, en] of Object.entries(RANKS)) {
  for (const [naipe, suit] of Object.entries(SUITS)) {
    // Figuras têm variante "...2.svg" com arte completa — prefira-a.
    const wanted = [`${en}_of_${suit}2.svg`, `${en}_of_${suit}.svg`]
    const found = wanted.find((f) => files.includes(f))
    if (!found) throw new Error(`não achei ${wanted.join(' ou ')} em ${src}`)
    copyFileSync(join(src, found), join(out, `${rank}-${naipe}.svg`))
  }
}
console.log('40 cartas copiadas para', out)
