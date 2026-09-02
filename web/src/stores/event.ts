// Shared event/challenge state. Both panels poll GET /api/config once a second
// (one event, one server — this is cheap and keeps every viewer in lock-step),
// and derive the lock state the same way the original front-end did.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { getConfig } from '@/api/client'
import type { EventConfig } from '@/api/types'

export type EventState = 'waiting' | 'running' | 'paused' | 'ended'

export const useEventStore = defineStore('event', () => {
  const config = ref<EventConfig | null>(null)
  const now = ref(Date.now())
  const lastError = ref<string | null>(null)
  const loaded = ref(false)

  let clock: number | undefined
  let poller: number | undefined

  async function refresh(): Promise<void> {
    try {
      config.value = await getConfig()
      lastError.value = null
      loaded.value = true
    } catch (e) {
      lastError.value = e instanceof Error ? e.message : 'خطا در دریافت وضعیت رویداد'
    }
  }

  function start(): void {
    if (clock === undefined) {
      clock = window.setInterval(() => (now.value = Date.now()), 1000)
    }
    if (poller === undefined) {
      void refresh()
      poller = window.setInterval(refresh, 1000)
    }
  }
  function stop(): void {
    if (clock !== undefined) {
      clearInterval(clock)
      clock = undefined
    }
    if (poller !== undefined) {
      clearInterval(poller)
      poller = undefined
    }
  }

  const challenge = computed(() => config.value?.challenge ?? null)

  const isWaiting = computed(
    () => !!config.value?.eventStartsAt && config.value.eventStartsAt > now.value,
  )

  const remainMs = computed(() => {
    const c = config.value
    if (!c) return 0
    if (c.eventPaused) return Math.max(0, c.pausedRemainMs ?? 0)
    if (c.eventEndsAt == null) return 0
    return c.eventEndsAt - now.value
  })

  const startsInMs = computed(() => {
    const c = config.value
    if (!c?.eventStartsAt) return 0
    return Math.max(0, c.eventStartsAt - now.value)
  })

  const state = computed<EventState>(() => {
    if (isWaiting.value) return 'waiting'
    if (config.value?.eventPaused) return 'paused'
    if (remainMs.value <= 0) return 'ended'
    return 'running'
  })

  /** Whether a participant may currently start/submit (server enforces this too). */
  const canCompete = computed(() => state.value === 'running')

  return {
    config,
    now,
    lastError,
    loaded,
    challenge,
    isWaiting,
    remainMs,
    startsInMs,
    state,
    canCompete,
    refresh,
    start,
    stop,
  }
})
