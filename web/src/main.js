import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './styles/base.css'
import './styles/components.css'
import './styles/pages.css'
import './styles/markdown.css'

createApp(App).use(router).mount('#app')
