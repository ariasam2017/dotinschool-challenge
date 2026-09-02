// The signed-in participant on the user panel: register / login / resume, and
// the guarded PATCH that progresses their own challenge.

import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  registerParticipant,
  lookupByPhone,
  resumeSession,
  updateParticipant,
} from '@/api/client'
import { rememberedSession } from '@/api/session'
import type { SessionParticipant } from '@/api/types'

export const useParticipantStore = defineStore('participant', () => {
  const user = ref<SessionParticipant | null>(null)

  async function register(payload: {
    firstName: string
    lastName: string
    phone: string
    otp: string
  }): Promise<SessionParticipant> {
    const u = await registerParticipant(payload)
    user.value = u
    rememberedSession.set(u.sessionToken)
    return u
  }

  async function loginByPhone(phone: string, otp: string): Promise<SessionParticipant> {
    const u = await lookupByPhone(phone, otp)
    user.value = u
    rememberedSession.set(u.sessionToken)
    return u
  }

  async function resume(): Promise<SessionParticipant | null> {
    const token = rememberedSession.get()
    if (!token) return null
    try {
      const u = await resumeSession(token)
      user.value = u
      return u
    } catch {
      rememberedSession.clear()
      return null
    }
  }

  /** Merge fields locally and persist them (server re-checks the live-event lock). */
  async function patch(fields: Record<string, unknown>): Promise<void> {
    if (!user.value) return
    Object.assign(user.value, fields)
    await updateParticipant(user.value.id, { sessionToken: user.value.sessionToken, ...fields })
  }

  /** Same as patch but swallows errors — used for best-effort "started" writes. */
  async function patchQuietly(fields: Record<string, unknown>): Promise<void> {
    try {
      await patch(fields)
    } catch {
      /* ignore */
    }
  }

  function signOut(): void {
    rememberedSession.clear()
    user.value = null
  }

  return { user, register, loginByPhone, resume, patch, patchQuietly, signOut }
})
