import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import { defineConfig, globalIgnores } from 'eslint/config'

/**
 * Minimal equivalent of eslint-plugin-react's `react/jsx-uses-vars` rule.
 *
 * Core ESLint does not understand JSX, so an identifier that is only
 * referenced from JSX (e.g. `motion` in `<motion.div />`) is reported as an
 * unused variable by `no-unused-vars`. This plugin marks those identifiers as
 * used so the rule reports genuine dead code only.
 */
const jsxUsesVars = {
  meta: {
    type: 'problem',
    schema: [],
    messages: {},
  },
  create(context) {
    const sourceCode = context.sourceCode ?? context.getSourceCode()

    const markRootIdentifier = (node) => {
      let current = node
      while (current && current.type === 'JSXMemberExpression') {
        current = current.object
      }
      if (current && current.type === 'JSXIdentifier') {
        sourceCode.markVariableAsUsed(current.name, node)
      }
    }

    return {
      JSXOpeningElement(node) {
        markRootIdentifier(node.name)
      },
      JSXClosingElement(node) {
        markRootIdentifier(node.name)
      },
      JSXMemberExpression(node) {
        markRootIdentifier(node)
      },
    }
  },
}

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{js,jsx}'],
    plugins: {
      'local': { rules: { 'jsx-uses-vars': jsxUsesVars } },
    },
    extends: [
      js.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
      parserOptions: {
        ecmaVersion: 'latest',
        ecmaFeatures: { jsx: true },
        sourceType: 'module',
      },
    },
    rules: {
      'no-unused-vars': ['error', { varsIgnorePattern: '^[A-Z_]' }],
      'local/jsx-uses-vars': 'error',
      // Context hooks live next to their provider on purpose: they are pure
      // `useContext` wrappers and must be exported from the same module.
      'react-refresh/only-export-components': ['error', {
        allowConstantExport: true,
        allowExportNames: ['useAuth', 'useGlobalError'],
      }],
    },
  },
])
