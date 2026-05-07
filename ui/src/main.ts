import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import { useBrandingStore } from './stores/branding'
import './assets/main.css'
import './assets/dashboard-compat.css'
import './assets/forms.css'

/**
 * Create Vue app instance
 */
const app = createApp(App)

/**
 * Install plugins
 */
app.use(createPinia())
app.use(router)

/**
 * Load operator branding overlay (if any) before mounting so the first paint
 * already shows the correct app name and logo. The pre-Vue loader splash is
 * still platform-default (it's hardcoded in index.html).
 */
const brandingStore = useBrandingStore()
brandingStore.load().then(() => {
  document.title = brandingStore.appName
  app.mount('#app')

  // Smooth fade-out of the initial loader
  const appLoader = document.getElementById('app-loader')
  if (appLoader) {
    requestAnimationFrame(() => {
      appLoader.classList.add('fade-out')
      setTimeout(() => appLoader.remove(), 300)
    })
  }
})

/**
 * Register Service Worker for PWA "Install" support
 * Grug say: Only register if browser support it and we not in dev mode
 */
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js')
      .then(reg => console.log('SW registered:', reg.scope))
      .catch(err => console.error('SW registration failed:', err));
  });
}

