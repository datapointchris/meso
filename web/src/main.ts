import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import { useTheme } from './composables/useTheme'

import './styles/main.scss'

// Apply the persisted (or system-preferred) theme before mount so there is no
// flash of the wrong palette.
useTheme().init()

createApp(App).use(router).mount('#app')
