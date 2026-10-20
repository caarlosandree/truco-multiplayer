import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import globals from 'globals'

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'src/protocol/**', 'playwright-report', 'test-results'] },
  js.configs.recommended,
  ...tseslint.configs.strict,
  ...vue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: { parser: vueParser, parserOptions: { parser: tseslint.parser, sourceType: 'module' } },
  },
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: {
      'vue/multi-word-component-names': 'off',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            { group: ['@/ws/*', '@/api/*'], message: 'Componentes e views falam com o servidor só por stores.' },
          ],
        },
      ],
    },
  },
  {
    // Quem pode importar ws/ e api/: as próprias camadas, stores, a ligação do app e os testes.
    files: ['src/stores/**', 'src/ws/**', 'src/api/**', 'src/app/**', 'src/test/**', 'src/**/*.test.ts'],
    rules: { 'no-restricted-imports': 'off' },
  },
  {
    // R2: asserção não-nula só é tolerada em testes.
    files: ['**/*.test.ts', 'src/test/**', 'e2e/**'],
    rules: { '@typescript-eslint/no-non-null-assertion': 'off' },
  },
)
