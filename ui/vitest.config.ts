import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Node environment and no component mounting, deliberately.
//
// The highest-risk logic in this console is pure: widget default construction,
// the role/capability map, dashboard import/export, twin drift comparison. All
// of it is reachable without a DOM, and `vue-tsc && vite build` stays green
// while any of it is wrong, so this is the cheapest guard available. Mounting
// components would need jsdom and would test rendering rather than the
// decisions the rendering depends on.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.spec.ts'],
  },
})
