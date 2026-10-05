import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { APP_MODE, ROUTER_BASE } from '@/lib/mode'

const GAME_TITLE = 'Game مدرسه داتین'

const userRoutes: RouteRecordRaw[] = [
  // The quiz game is the landing page; the coding challenge lives beside it.
  { path: '/', name: 'game', component: () => import('@/views/game/GameApp.vue'), meta: { title: GAME_TITLE } },
  {
    path: '/challenge',
    name: 'user',
    component: () => import('@/views/user/UserApp.vue'),
    meta: { title: 'dotinschool — چالش برنامه‌نویسی' },
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const adminRoutes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('@/views/admin/AdminShell.vue'),
    children: [
      { path: '', redirect: { name: 'admin-game' } },
      { path: 'game', name: 'admin-game', component: () => import('@/views/admin/GameBoardSection.vue') },
      { path: 'game-top', name: 'admin-game-top', component: () => import('@/views/admin/GameTopSection.vue') },
      { path: 'game-questions', name: 'admin-game-questions', component: () => import('@/views/admin/GameQuestionsSection.vue') },
      { path: 'overview', name: 'admin-overview', component: () => import('@/views/admin/OverviewSection.vue') },
      { path: 'schedule', name: 'admin-schedule', component: () => import('@/views/admin/ScheduleSection.vue') },
      { path: 'challenge', name: 'admin-challenge', component: () => import('@/views/admin/ChallengeSection.vue') },
      { path: 'participants', name: 'admin-participants', component: () => import('@/views/admin/ParticipantsSection.vue') },
      { path: 'ai', name: 'admin-ai', component: () => import('@/views/admin/AiReviewSection.vue') },
      { path: 'assistant', name: 'admin-assistant', component: () => import('@/views/admin/CodeAssistantSection.vue') },
      { path: 'appearance', name: 'admin-appearance', component: () => import('@/views/admin/AppearanceSection.vue') },
      { path: 'resources', name: 'admin-resources', component: () => import('@/views/admin/ServerResourcesSection.vue') },
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

router.afterEach((to) => {
  document.title = (to.meta.title as string | undefined) || (APP_MODE === 'admin' ? 'پنل مدیریت — ' + GAME_TITLE : GAME_TITLE)
})
