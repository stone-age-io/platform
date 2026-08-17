import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // Proxy API requests to PocketBase
      '/api': {
        target: 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
      // Proxy admin/auth endpoints
      '/_': {
        target: 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
      // Proxy operator branding overlay (theme.css, logo.svg, branding.json)
      '/branding': {
        target: 'http://127.0.0.1:8090',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: '../pb_public',
    emptyOutDir: true,
    // Vite 8 bundles with Rolldown, not Rollup. Two consequences here:
    // `rollupOptions` is a deprecated alias for `rolldownOptions`, and the
    // object form of `manualChunks` is gone -- Rolldown accepts only a
    // function, and `codeSplitting.groups` is the intended replacement. Groups
    // stay closer to the old object form than a function would: a group also
    // captures the dependencies of what it matches, because
    // `includeDependenciesRecursively` defaults to true. That is what kept
    // e.g. zrender in the echarts chunk.
    //
    // Each `test` demands a path separator after the package name so a prefix
    // cannot swallow a sibling: `vue/` must not capture `vue-echarts/`, which
    // belongs in the echarts chunk.
    //
    // ORDER IS LOAD-BEARING, and it interacts with the recursive dependency
    // capture above. Earlier groups win, and a group takes the dependencies of
    // what it matched -- so with echarts listed first, echarts' own dependency
    // on Vue dragged @vue/shared, @vue/reactivity and @vue/runtime-core into
    // the echarts chunk, and grid-layout-plus took @vue/runtime-dom. That left
    // `vue-vendor` holding only pinia and vue-router (~31kB rather than
    // ~114kB): the framework was being re-downloaded under two chunk names
    // that change whenever a chart or grid dependency changes, which is the
    // opposite of what a long-cached vendor chunk is for.
    //
    // So vue-vendor goes FIRST, to claim the framework before any consumer
    // pulls it in. If you reorder these, verify with `vite build --sourcemap`
    // and read the chunk .map `sources` -- chunk *names* stay stable and
    // reveal nothing; only the sizes move.
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            // Vue core (cached across deployments). @vue/* is named explicitly
            // because `vue` is a thin re-export -- the runtime lives in the
            // scoped packages.
            { name: 'vue-vendor', test: /[\\/]node_modules[\\/](?:@vue[\\/][^\\/]+|vue|vue-router|pinia)[\\/]/ },
            // Charting library (largest dependency ~1MB)
            { name: 'echarts', test: /[\\/]node_modules[\\/](echarts|vue-echarts)[\\/]/ },
            // Mapping library
            { name: 'leaflet', test: /[\\/]node_modules[\\/]leaflet[\\/]/ },
            // NATS messaging libraries
            { name: 'nats', test: /[\\/]node_modules[\\/]@nats-io[\\/]/ },
            // Grid layout
            { name: 'grid', test: /[\\/]node_modules[\\/]grid-layout-plus[\\/]/ },
          ],
        },
      },
    },
    // Warn if a chunk exceeds 500KB
    chunkSizeWarningLimit: 500,
  },
})

