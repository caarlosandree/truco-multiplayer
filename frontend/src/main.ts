import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './assets/fonts/fonts.css'
import './styles.css'
import { installSfx } from './audio/install'
import { createAppRouter } from './router'
import { installConnection } from './app/wire'
import { installRoomNavigation } from './app/navigation'
import { installInviteAlerts } from './app/inviteAlerts'

const app = createApp(App)
app.use(createPinia())
const router = createAppRouter()
app.use(router)
installSfx()
installInviteAlerts()
installConnection(router)
installRoomNavigation(router)
app.mount('#app')
