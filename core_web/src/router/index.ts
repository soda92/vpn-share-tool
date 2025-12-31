import { createRouter, createWebHistory } from 'vue-router'
import DebugView from '../components/DebugView.vue'
import RequestComparator from '../components/RequestComparator.vue'

const routes = [
  {
    path: '/',
    name: 'DebugView',
    component: DebugView
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
