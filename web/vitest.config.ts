import { defineConfig } from 'vitest/config'
import path from 'path'

// Unit tests target framework-free modules (utils, composables), so no Vue or Tailwind plugin is loaded.
export default defineConfig({
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
      include: ['src/utils/**/*.ts', 'src/composables/**/*.ts', 'src/stores/**/*.ts', 'src/services/**/*.ts', 'src/directives/**/*.ts'],
      exclude: ['**/*.test.ts'],
    },
  },
})
