import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { vDialog } from './directives/dialog'
import { vClickable } from './directives/clickable'
import './assets/main.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)
app.directive('dialog', vDialog)
app.directive('clickable', vClickable)
app.mount('#app')

if ('serviceWorker' in navigator && import.meta.env.PROD) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch((err) => {
      console.warn('SW registration failed:', err)
    })
  })
}
