import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { APP_MODE, ROUTER_BASE } from '@/lib/mode'

const userRoutes: RouteRecordRaw[] = [
  { path: '/', name: 'user', component: () => import('@/views/user/UserApp.vue') },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const adminRoutes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('@/views/admin/AdminShell.vue'),
    children: [
      { path: '', redirect: { name: 'admin-overview' } },
      { path: 'overview', name: 'admin-overview', component: () => import('@/views/admin/OverviewSection.vue') },
      { path: 'schedule', name: 'admin-schedule', component: () => import('@/views/admin/ScheduleSection.vue') },
      { path: 'challenge', name: 'admin-challenge', component: () => import('@/views/admin/ChallengeSection.vue') },
      { path: 'participants', name: 'admin-participants', component: () => import('@/views/admin/ParticipantsSection.vue') },
      { path: 'ai', name: 'admin-ai', component: () => import('@/views/admin/AiReviewSection.vue') },
      { path: 'assistant', name: 'admin-assistant', component: () => import('@/views/admin/CodeAssistantSection.vue') },
      { path: 'errors', name: 'admin-errors', component: () => import('@/views/admin/ClientErrorsSection.vue') },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export const router = createRouter({
  history: createWebHistory(ROUTER_BASE),
  routes: APP_MODE === 'admin' ? adminRoutes : userRoutes,
  scrollBehavior: () => ({ top: 0 }),
})
