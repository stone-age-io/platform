import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      // Not __dirname: Vite's native ESM config loader has no CJS globals.
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
      // PocketBase admin UI
      '/_': {
        target: 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
      // Operator branding overlay (theme.css, logo.svg, branding.json)
      '/branding': {
        target: 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../pb_public',
    emptyOutDir: true,
    // Vite 8 bundles with Rolldown: `rolldownOptions`, and `codeSplitting.groups`
    // in place of object-form `manualChunks`. A group also captures the
    // dependencies of what it matches (`includeDependenciesRecursively`), which
    // keeps e.g. zrender in the echarts chunk.
    //
    // Each `test` requires a path separator after the package name, so `vue/`
    // does not capture `vue-echarts/`.
    //
    // ORDER IS LOAD-BEARING: earlier groups win, so vue-vendor goes first to
    // claim the framework before echarts or grid-layout-plus drag @vue/* into
    // their own chunks. If you reorder, verify with `vite build --sourcemap` and
    // the chunk .map `sources`; chunk names stay stable, only sizes move.
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            // @vue/* is named explicitly: `vue` is a thin re-export of them.
            { name: 'vue-vendor', test: /[\\/]node_modules[\\/](?:@vue[\\/][^\\/]+|vue|vue-router|pinia)[\\/]/ },
            { name: 'echarts', test: /[\\/]node_modules[\\/](echarts|vue-echarts)[\\/]/ },
            { name: 'leaflet', test: /[\\/]node_modules[\\/]leaflet[\\/]/ },
            // ~1MB on its own; ungrouped it lands in the useLeafletMap chunk and
            // re-hashes with every edit to that composable.
            { name: 'maplibre', test: /[\\/]node_modules[\\/](?:maplibre-gl|@maplibre[\\/][^\\/]+)[\\/]/ },
            { name: 'nats', test: /[\\/]node_modules[\\/]@nats-io[\\/]/ },
            { name: 'grid', test: /[\\/]node_modules[\\/]grid-layout-plus[\\/]/ },
          ],
        },
      },
    },
    chunkSizeWarningLimit: 500,
  },
})

