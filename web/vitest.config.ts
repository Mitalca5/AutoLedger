import { defineConfig } from 'vitest/config'
import path from 'path'
import vue from '@vitejs/plugin-vue'

// Tests stay in Node; the Vue plugin also allows rendering component contracts without a browser.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
    setupFiles: ['./vitest.setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['lcov', 'text-summary'],
      include: ['src/units.ts', 'src/currency.ts', 'src/utils/**/*.ts', 'src/composables/**/*.ts', 'src/stores/**/*.ts', 'src/services/**/*.ts', 'src/directives/**/*.ts'],
      exclude: ['**/*.test.ts'],
    },
  },
})
