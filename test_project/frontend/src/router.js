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
    beforeEnter: async (to, from, next) => {
      try {
        const response = await fetch('/api/check-auth');
        if (response.ok) {
          next();
        } else {
          next('/login');
        }
      } catch (error) {
        console.error('Error checking auth:', error);
        next('/login');
      }
    },
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
