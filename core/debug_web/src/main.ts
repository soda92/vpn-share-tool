import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import Toast from 'vue-toastification';
import 'vue-toastification/dist/index.css';
import * as Sentry from '@sentry/vue';
import posthog from 'posthog-js';

const app = createApp(App);

// Initialize PostHog
posthog.init('phc_ym4rdoQgT2oeKEBvyYb35qacFPTEwQyLNMf3jUojXoiq', {
  api_host: 'https://benefit.sodacris.com',
  person_profiles: 'identified_only',
  capture_pageview: true,
});

// Initialize Sentry
Sentry.init({
  app,
  dsn: 'https://bc888ace3f8f6751be2c1a8b8d71c71f@benefit.sodacris.com/4511405673480272',
  integrations: [
    Sentry.browserTracingIntegration(),
    Sentry.replayIntegration(),
  ],
  tracesSampleRate: 1.0,
  replaysSessionSampleRate: 0.1,
  replaysOnErrorSampleRate: 1.0,
});

app.use(router);
app.use(Toast);

app.mount('#app');