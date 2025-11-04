import { createRouter, createWebHistory } from 'vue-router';
import Login from './views/Login.vue';
import Main from './views/Main.vue';

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login,
  },
  {
    path: '/',
    name: 'Main',
    component: Main,
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
