/**
 * eslint.config.mjs
 *
 * ESLint configuration file.
 */

import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import { withVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'

export default withVueTs(
  {
    // built and generated files
    ignores: ['dist/**', 'components.d.ts', 'typed-router.d.ts'],
  },
  js.configs.recommended,
  pluginVue.configs['flat/essential'],
  // same rule set as with ESLint 8 and @vue/eslint-config-typescript 13; vueTsConfigs.recommended would additionally
  // enable typescript-eslint's recommended rules (e.g. no-explicit-any)
  vueTsConfigs.base,
  vueTsConfigs.eslintRecommended,
  {
    files: ['**/*.ts', '**/*.cts', '**/*.mts', '**/*.tsx', '**/*.vue'],
    rules: {
      // the core rules do not work with type definitions, and TypeScript already checks for undefined variables
      'no-unused-vars': 'off',
      'no-undef': 'off',
      '@typescript-eslint/no-unused-vars': 'warn',
    },
  },
  {
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },
)
