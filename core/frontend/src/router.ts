import { createRouter, createWebHistory } from 'vue-router';
import type { RouteRecordRaw } from 'vue-router';
import LandingPage from './views/LandingPage.vue';
import LiveSession from './views/LiveSession.vue';
import SavedSession from './views/SavedSession.vue';
import RequestDetailView from './views/RequestDetailView.vue';
import FormDecoder from './views/FormDecoder.vue';

const routes: Array<RouteRecordRaw> = [
  { path: '/', component: LandingPage },
  { path: '/live', component: LiveSession },
  { path: '/session/:id', component: SavedSession },
  { path: '/request/:id', component: RequestDetailView },
  { path: '/decoder', component: FormDecoder },
];

const router = createRouter({
  history: createWebHistory('/debug'), // Set the base path for the debug UI
  routes,
});

export default router;
