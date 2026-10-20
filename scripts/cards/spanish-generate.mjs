// Gera as 40 cartas espanholas e o verso em SVG autoral.
// Uso: node scripts/cards/spanish-generate.mjs
import { mkdirSync, writeFileSync } from 'node:fs'

const out = 'frontend/src/assets/cards/espanhol'
mkdirSync(out, { recursive: true })
const W = 169, H = 245

const SUITS = {
  ouros: { color: '#b45309', name: 'oros' },
  copas: { color: '#b91c1c', name: 'copas' },
  espadas: { color: '#1d4ed8', name: 'espadas' },
  paus: { color: '#15803d', name: 'bastos' },
}

// Símbolos desenhados em caixa 40x40 centrada em (0,0).
function pip(naipe) {
  const c = SUITS[naipe].color
  switch (naipe) {
    case 'ouros':
      return `<circle r="17" fill="${c}"/><circle r="11" fill="none" stroke="#fde68a" stroke-width="3"/><circle r="4" fill="#fde68a"/>`
    case 'copas':
      return `<path d="M-14 -16 H14 Q14 2 3 6 V12 H9 V17 H-9 V12 H-3 V6 Q-14 2 -14 -16 Z" fill="${c}"/><rect x="-14" y="-18" width="28" height="4" rx="2" fill="#fca5a5"/>`
    case 'espadas':
      return `<path d="M0 -19 L4 -6 V10 H-4 V-6 Z" fill="${c}"/><rect x="-10" y="9" width="20" height="4" rx="2" fill="#1e3a8a"/><rect x="-2.5" y="13" width="5" height="7" fill="#1e3a8a"/>`
    case 'paus':
      return `<path d="M-5 19 Q-8 0 -4 -16 Q0 -21 5 -16 Q9 0 5 19 Z" fill="${c}"/><circle cx="-6" cy="-6" r="3" fill="#166534"/><circle cx="6" cy="4" r="3" fill="#166534"/>`
  }
}

// Posições dos símbolos para 1..7 (coordenadas relativas ao centro da carta).
const LAYOUTS = {
  1: [[0, 0, 2.2]],
  2: [[0, -55, 1], [0, 55, 1]],
  3: [[0, -60, 1], [0, 0, 1], [0, 60, 1]],
  4: [[-32, -55, 1], [32, -55, 1], [-32, 55, 1], [32, 55, 1]],
  5: [[-32, -55, 1], [32, -55, 1], [0, 0, 1], [-32, 55, 1], [32, 55, 1]],
  6: [[-32, -60, 1], [32, -60, 1], [-32, 0, 1], [32, 0, 1], [-32, 60, 1], [32, 60, 1]],
  7: [[-32, -65, 1], [32, -65, 1], [0, -32, 1], [-32, 0, 1], [32, 0, 1], [-32, 65, 1], [32, 65, 1]],
}

const FIGURES = { 10: 'Sota', 11: 'Caballo', 12: 'Rey' }

function figure(rank, naipe) {
  const c = SUITS[naipe].color
  return `
    <rect x="-52" y="-82" width="104" height="164" rx="8" fill="${c}" opacity="0.12"/>
    <circle cx="0" cy="-38" r="16" fill="#fcd34d" stroke="${c}" stroke-width="2"/>
    <path d="M-28 30 Q-26 -18 0 -20 Q26 -18 28 30 Z" fill="${c}" opacity="0.85"/>
    ${rank === 12 ? `<path d="M-14 -56 L-8 -66 L0 -56 L8 -66 L14 -56 Z" fill="#f59e0b"/>` : ''}
    ${rank === 11 ? `<path d="M-30 40 Q0 20 30 40 L24 62 H-24 Z" fill="#78350f" opacity="0.8"/>` : ''}
    <g transform="translate(0 48) scale(0.8)">${pip(naipe)}</g>
    <text y="76" text-anchor="middle" font-family="Georgia, serif" font-size="13" fill="${c}">${FIGURES[rank]}</text>`
}

function card(rank, naipe) {
  const c = SUITS[naipe].color
  const body = rank <= 7
    ? LAYOUTS[rank].map(([x, y, s]) => `<g transform="translate(${x} ${y}) scale(${s})">${pip(naipe)}</g>`).join('')
    : figure(rank, naipe)
  const corner = (rot) => `<g transform="rotate(${rot} ${W / 2} ${H / 2})">
      <text x="12" y="26" font-family="Georgia, serif" font-size="22" font-weight="700" fill="${c}">${rank}</text>
      <g transform="translate(20 42) scale(0.35)">${pip(naipe)}</g></g>`
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}">
  <rect x="1" y="1" width="${W - 2}" height="${H - 2}" rx="9" fill="#fffdf7" stroke="#111" stroke-width="1"/>
  <rect x="6" y="6" width="${W - 12}" height="${H - 12}" rx="6" fill="none" stroke="${c}" stroke-width="1.5"/>
  ${corner(0)}${corner(180)}
  <g transform="translate(${W / 2} ${H / 2})">${body}</g>
</svg>
`
}

for (const rank of [1, 2, 3, 4, 5, 6, 7, 10, 11, 12]) {
  for (const naipe of Object.keys(SUITS)) writeFileSync(`${out}/${rank}-${naipe}.svg`, card(rank, naipe))
}
writeFileSync(`${out}/verso.svg`, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}">
  <defs><pattern id="p" width="14" height="14" patternUnits="userSpaceOnUse">
    <rect width="14" height="14" fill="#1e3a8a"/><circle cx="7" cy="7" r="3" fill="#fbbf24"/></pattern></defs>
  <rect x="1" y="1" width="${W - 2}" height="${H - 2}" rx="9" fill="#fff" stroke="#111" stroke-width="1"/>
  <rect x="10" y="10" width="${W - 20}" height="${H - 20}" rx="6" fill="url(#p)"/>
</svg>
`)
console.log('40 cartas + verso geradas em', out)
