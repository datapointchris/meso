import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

// The five top-level surfaces (Sessions | Workouts | Movements | Log | Stats), each
// with a detail route. Cycles is still routed, reached from the Workouts header.
//
// Opening the app lands on Sessions: the reason to open it is almost always to train.
const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/sessions' },
  {
    path: '/workouts',
    name: 'workouts',
    component: () => import('@/views/WorkoutsView.vue'),
  },
  {
    path: '/workouts/:id',
    name: 'workout-detail',
    component: () => import('@/views/WorkoutDetailView.vue'),
  },
  {
    path: '/movements',
    name: 'movements',
    component: () => import('@/views/MovementsView.vue'),
  },
  {
    path: '/movements/:id',
    name: 'movement-detail',
    component: () => import('@/views/MovementDetailView.vue'),
  },
  {
    path: '/sessions',
    name: 'sessions',
    component: () => import('@/views/SessionsView.vue'),
  },
  {
    path: '/sessions/:id',
    name: 'session-detail',
    component: () => import('@/views/ActiveSessionView.vue'),
  },
  {
    path: '/cycles',
    name: 'cycles',
    component: () => import('@/views/CyclesView.vue'),
  },
  {
    path: '/cycles/:id',
    name: 'cycle-detail',
    component: () => import('@/views/CycleDetailView.vue'),
  },
  {
    path: '/log',
    name: 'log',
    component: () => import('@/views/LogView.vue'),
  },
  {
    path: '/stats',
    name: 'stats',
    component: () => import('@/views/StatsView.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
