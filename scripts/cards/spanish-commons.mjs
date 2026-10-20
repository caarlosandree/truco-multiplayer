// Uso: node scripts/cards/spanish-commons.mjs "Category:SVG Spanish deck"
const category = process.argv[2] ?? 'Category:SVG Spanish deck'
const api = 'https://commons.wikimedia.org/w/api.php'
const ALLOWED = /^(public domain|pd|cc0)/i

const params = new URLSearchParams({
  action: 'query', format: 'json', generator: 'categorymembers', gcmtitle: category,
  gcmtype: 'file', gcmlimit: '200', prop: 'imageinfo', iiprop: 'url|extmetadata', origin: '*',
})
const res = await fetch(`${api}?${params}`, { headers: { 'User-Agent': 'truco-assets/1.0' } })
const data = await res.json()
const rows = Object.values(data.query?.pages ?? {}).map((p) => {
  const meta = p.imageinfo?.[0]?.extmetadata ?? {}
  return { title: p.title, license: meta.LicenseShortName?.value ?? '?', artist: (meta.Artist?.value ?? '').replace(/<[^>]+>/g, ''), url: p.imageinfo?.[0]?.url }
})
for (const r of rows) console.log(`${ALLOWED.test(r.license) ? 'OK ' : '-- '} ${r.license.padEnd(14)} ${r.title}`)
console.log(`\n${rows.filter((r) => ALLOWED.test(r.license)).length} de ${rows.length} com licença aceita`)
