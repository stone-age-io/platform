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
    },
  },
  build: {
    outDir: '../pb_public',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        // Split large dependencies into separate chunks for better caching
        manualChunks: {
          // Charting library (largest dependency ~1MB)
          'echarts': ['echarts', 'vue-echarts'],
          // Mapping library
          'leaflet': ['leaflet'],
          // NATS messaging libraries
          'nats': ['@nats-io/nats-core', '@nats-io/jetstream', '@nats-io/kv'],
          // Grid layout
          'grid': ['grid-layout-plus'],
          // Vue core (cached across deployments)
          'vue-vendor': ['vue', 'vue-router', 'pinia'],
        },
      },
    },
    // Warn if a chunk exceeds 500KB
    chunkSizeWarningLimit: 500,
  },
})

