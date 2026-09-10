import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { router } from './router'
import { applyTheme } from './lib/theme'
import { startFaDigits } from './lib/fa-digits'
import { initAppearanceBridge } from './lib/appearance'

import 'ant-design-vue/dist/reset.css'
import './styles/fonts.css'
import './styles/tokens.css'
import './styles/base.css'
import './styles/auth.css'

applyTheme()
initAppearanceBridge()

createApp(App).use(createPinia()).use(router).mount('#app')

// App-wide Persian digits (both the user and the admin panel run this entry).
startFaDigits()
