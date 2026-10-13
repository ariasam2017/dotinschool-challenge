// Admin-side shared state: auth gate, the participants list (polled while a
// data section is visible), and the client-error unseen badge.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { message } from 'ant-design-vue'
import { adminLogin, adminLogout, listParticipants, listClientErrors, ApiError } from '@/api/client'
import { adminToken } from '@/api/session'
import type { Participant } from '@/api/types'

export const useAdminStore = defineStore('admin', () => {
  const authed = ref(adminToken.isAuthed())
  const username = ref(adminToken.user() || '')
  const sessionExpiredNote = ref('')

  const participants = ref<Participant[]>([])
  const participantsLoaded = ref(false)
  const unseenErrors = ref(0)

  let poller: number | undefined

  async function login(user: string, pass: string): Promise<boolean> {
    const ok = await adminLogin(user, pass)
    if (ok) {
      authed.value = true
      username.value = adminToken.user() || user
      sessionExpiredNote.value = ''
    }
    return ok
  }

  async function logout(): Promise<void> {
    await adminLogout()
    authed.value = false
    stopPolling()
  }

  /** Call from any catch: a 401 drops us back to the login screen cleanly. */
  function handleAuthError(e: unknown): boolean {
    if (e instanceof ApiError && (e.code === 'UNAUTHORIZED' || e.status === 401)) {
      adminToken.clear()
      authed.value = false
      stopPolling()
      sessionExpiredNote.value =
        'نشست مدیریتی شما منقضی شده (مثلا به‌خاطر ری‌استارت سرور) — دوباره وارد شوید.'
      return true
    }
    return false
  }

  async function refreshParticipants(): Promise<void> {
    try {
      participants.value = await listParticipants()
      participantsLoaded.value = true
    } catch (e) {
      if (!handleAuthError(e)) {
        /* transient — keep last snapshot */
      }
    }
  }

  async function refreshErrorBadge(): Promise<void> {
    try {
      const { unseen } = await listClientErrors()
      unseenErrors.value = unseen
    } catch (e) {
      handleAuthError(e)
    }
  }

  function startPolling(): void {
    if (poller !== undefined) return
    void refreshParticipants()
    void refreshErrorBadge()
    poller = window.setInterval(() => {
      void refreshParticipants()
      void refreshErrorBadge()
    }, 3000)
  }
  function stopPolling(): void {
    if (poller !== undefined) {
      clearInterval(poller)
      poller = undefined
    }
  }

  const stats = computed(() => {
    const p = participants.value
    const started = p.filter((u) => u.status === 'started').length
    const submitted = p.filter((u) => u.status === 'submitted').length
    const scored = p.filter((u) => u.score != null)
    const avg = scored.length
      ? Math.round(scored.reduce((a, u) => a + (u.score || 0), 0) / scored.length)
      : 0
    return { total: p.length, started, submitted, avg }
  })

  function toastError(e: unknown, fallback: string): void {
    if (handleAuthError(e)) return
    message.error(e instanceof Error ? e.message : fallback)
  }

  return {
    authed,
    username,
    sessionExpiredNote,
    participants,
    participantsLoaded,
    unseenErrors,
    stats,
    login,
    logout,
    handleAuthError,
    refreshParticipants,
    refreshErrorBadge,
    startPolling,
    stopPolling,
    toastError,
  }
})
