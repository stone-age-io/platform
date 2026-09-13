import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router, { applyDocumentTitle } from './router'
import App from './App.vue'
import { useBrandingStore } from './stores/branding'
import { useAuthStore } from './stores/auth'
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
 * Hydrate auth from localStorage AND load operator branding before mounting.
 *
 * Auth hydration must complete pre-mount: the router's beforeEach guard reads
 * authStore.isAuthenticated, so if we mount before initializeFromAuth runs,
 * the first navigation evaluates against an empty Pinia store and redirects
 * to /login — even when a valid PocketBase token is sitting in localStorage
 * (new-tab/new-window scenario).
 *
 * Branding load is unrelated but already gates first paint, so we run both in
 * parallel. Each is wrapped in a defensive catch so a single failure can't
 * block mount; initializeFromAuth already catches its own auth errors and
 * calls logout() on a bad token.
 */
const brandingStore = useBrandingStore()
const authStore = useAuthStore()

Promise.all([
  brandingStore.load().catch(err => console.error('Branding load failed:', err)),
  authStore.initializeFromAuth().catch(err => console.error('Auth init failed:', err)),
]).finally(() => {
  // Re-stamp the tab now that branding has resolved: the router already set a
  // title for the initial route, but against the compiled-in default name.
  applyDocumentTitle()
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

