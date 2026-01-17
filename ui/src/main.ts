import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
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
 * Mount app
 */
app.mount('#app')

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
