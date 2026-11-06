import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// Node environment by default; the highest-risk logic is pure and reachable
// without a DOM. A spec that needs one opts in with `// @vitest-environment jsdom`
// on its first line. Prefer a pure test: a mount tests the rendering, not the
// decision behind it.
//
// The mounting exceptions, and why each earns it:
//
//   ConfirmDialog       the DOM contract is the subject
//   QrLabelModal        N records in, N printable boxes out
//   ThingFormView       the body POST /api/org/things parses, and the fields a
//   NebulaHostFormView  member may not send; textareas back into the arrays,
//                       integers and nested object pb-nebula wants
//   AddWidgetModal      a widget missing from the palette cannot be added at all
//
// Keep the list short, with no coverage target: what earns a mount is a
// failure that is both silent and total.
export default defineConfig({
  plugins: [vue()],
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
