# Créditos — fontes

As duas fontes são auto-hospedadas (a CSP do servidor não permite CDN) e distribuídas sob a
**SIL Open Font License 1.1** (https://openfontlicense.org). O texto completo da licença acompanha
cada pacote em `node_modules/@fontsource/<fonte>/LICENSE`.

| Fonte | Uso | Autor / copyright | Licença | Origem |
|---|---|---|---|---|
| Kalam 400 e 700 | placar, tentos e anúncios da mesa ("Truco!", "Seis!") | Copyright (c) 2014 Indian Type Foundry | OFL 1.1 | https://github.com/google/fonts/tree/main/ofl/kalam |
| Atkinson Hyperlegible Next 400 e 700 | todo o resto da interface | Copyright 2020-2024 The Atkinson Hyperlegible Next Project Authors | OFL 1.1 | https://github.com/googlefonts/atkinson-hyperlegible-next |

## Como entram no app

- Pacotes npm `@fontsource/kalam@5.3.0` e `@fontsource/atkinson-hyperlegible-next@5.3.0`
  (versões fixadas no `pnpm-lock.yaml`), que redistribuem os arquivos do Google Fonts sem alteração.
- `fonts.css` (nesta pasta, importado em `src/main.ts`) declara os `@font-face` só com os pesos 400 e
  700 e só os subconjuntos `latin` e `latin-ext` (cobrem o pt-BR), em `.woff2`, com `unicode-range`
  e `font-display: swap`. O subconjunto devanágari da Kalam e os `.woff` de fallback não são usados.
- No `vite build` os `.woff2` saem como arquivos separados em `dist/assets/` (o
  `build.assetsInlineLimit` do `vite.config.ts` nunca embute fontes como `data:`).
- Nenhum nome reservado foi alterado e as fontes não são vendidas isoladamente, como a OFL exige.
