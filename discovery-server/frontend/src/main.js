import './assets/main.css';
import { createApp } from 'vue';
import App from './App.vue';
import setupI18n from './i18n';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';

async function init() {
    const i18n = await setupI18n();
    const app = createApp(App);
    app.use(i18n);
    app.use(ElementPlus);
    app.mount('#app');
}

init();
