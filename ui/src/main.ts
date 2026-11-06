import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router, { applyDocumentTitle } from './router'
import App from './App.vue'
import { useBrandingStore } from './stores/branding'
import { useAuthStore } from './stores/auth'
import './assets/main.css'
import './assets/dashboard-compat.css'
import './assets/forms.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)

/**
 * Hydrate auth and load branding before mounting.
 *
 * Auth must hydrate pre-mount: otherwise the first navigation sees an empty
 * store and redirects to /login even with a valid token in localStorage (a new
 * tab). The router guard also awaits it; the call is memoized.
 *
 * Each promise has its own catch so one failure cannot block mount;
 * initializeFromAuth calls logout() on a bad token.
 */
const brandingStore = useBrandingStore()
const authStore = useAuthStore()

Promise.all([
  brandingStore.load().catch(err => console.error('Branding load failed:', err)),
  authStore.initializeFromAuth().catch(err => console.error('Auth init failed:', err)),
]).finally(() => {
  // The router set the first title with the compiled-in default name.
  applyDocumentTitle()
  app.mount('#app')

  const appLoader = document.getElementById('app-loader')
  if (appLoader) {
    requestAnimationFrame(() => {
      appLoader.classList.add('fade-out')
      setTimeout(() => appLoader.remove(), 300)
    })
  }
})

// Service worker for PWA install; production only.
if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js')
      .catch(err => console.error('SW registration failed:', err));
  });
}

