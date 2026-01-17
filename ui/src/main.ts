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
