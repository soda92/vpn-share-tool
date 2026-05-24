import './assets/main.css';
import { createApp } from 'vue';
import App from './App.vue';
import setupI18n from './i18n';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import * as Sentry from '@sentry/vue';
import posthog from 'posthog-js';

async function init() {
    const i18n = await setupI18n();
    const app = createApp(App);

    const posthogKey = import.meta.env.VITE_POSTHOG_KEY;
    const posthogHost = import.meta.env.VITE_POSTHOG_HOST || 'https://us.i.posthog.com';
    const sentryDsn = import.meta.env.VITE_SENTRY_DSN;

    // Initialize PostHog
    if (posthogKey) {
        posthog.init(posthogKey, {
            api_host: posthogHost,
            person_profiles: 'identified_only',
            capture_pageview: true,
        });
    }

    // Initialize Sentry
    if (sentryDsn) {
        Sentry.init({
            app,
            dsn: sentryDsn,
            integrations: [
                Sentry.browserTracingIntegration(),
                Sentry.replayIntegration(),
            ],
            tracesSampleRate: 1.0,
            replaysSessionSampleRate: 0.1,
            replaysOnErrorSampleRate: 1.0,
        });
    }

    app.use(i18n);
    app.use(ElementPlus);
    app.mount('#app');
}

init();
