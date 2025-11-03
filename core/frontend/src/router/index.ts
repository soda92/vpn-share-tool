import { createRouter, createWebHistory } from 'vue-router'
import Debug from '../Debug.vue'
import RequestComparator from '../components/RequestComparator.vue'

const routes = [
  {
    path: '/',
    name: 'Debug',
    component: Debug
  },
  {
    path: '/compare',
    name: 'RequestComparator',
    component: RequestComparator
  }
]

const router = createRouter({
  history: createWebHistory('/debug/'),
  routes
})

export default router
