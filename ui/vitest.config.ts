import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// Node environment by default, and component mounting is the exception.
//
// The highest-risk logic in this console is pure: widget default construction,
// the role/capability map, dashboard import/export, twin drift comparison. All
// of it is reachable without a DOM, and `vue-tsc && vite build` stays green
// while any of it is wrong, so that is where the cheap wins are.
//
// A spec that genuinely needs a DOM opts in per file with
// `// @vitest-environment jsdom` on its first line. ConfirmDialog is the
// original case: what is under test there IS the DOM contract — dialog
// semantics, focus movement, key handling — and it guards every destructive
// action in the product. Prefer a pure test where one is possible; mounting a
// component to check what it renders tests the rendering rather than the
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
// That is five files out of 242 components, and the list is meant to stay
// short. A coverage target here would turn this into a chore nobody finishes;
// what earns a mount is a failure that is both silent and total.
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
