import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import { router } from './router'
import { applyTheme } from './lib/theme'

import 'ant-design-vue/dist/reset.css'
import './styles/fonts.css'
import './styles/tokens.css'
import './styles/base.css'

applyTheme()

createApp(App).use(createPinia()).use(router).mount('#app')
