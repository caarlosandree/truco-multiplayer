import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './styles.css'
import { installSfx } from './audio/install'

const app = createApp(App).use(createPinia())
installSfx()
app.mount('#app')
