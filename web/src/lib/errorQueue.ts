// Failed-submission reporting with an offline queue. A report that can't reach
// the server right now is stashed in localStorage and flushed on the next
// `online` event or page visit. Never throws, never slows the panel down.

import { reportClientError } from '@/api/client'

const QUEUE_KEY = 'dotinschool_err_queue_v1'

export interface ClientErrorInput {
  participantId?: string | null
  sessionToken?: string | null
  publicId?: string | null
  name?: string
  context?: string
  errorCode?: string
  errorMessage?: string
  code?: string
}

function readQueue(): ClientErrorInput[] {
  try {
    return JSON.parse(localStorage.getItem(QUEUE_KEY) || '[]')
  } catch {
    return []
  }
}
function writeQueue(q: ClientErrorInput[]): void {
  try {
    localStorage.setItem(QUEUE_KEY, JSON.stringify(q.slice(-20)))
  } catch {
    /* ignore */
  }
}

export function queueClientError(payload: ClientErrorInput): void {
  const q = readQueue()
  q.push(payload)
  writeQueue(q)
}

/** Fire-and-forget report; falls back to the offline queue on any failure. */
export function submitClientError(payload: ClientErrorInput): void {
  try {
    Promise.resolve(reportClientError(payload as Record<string, unknown>))
      .then((r) => {
        if (!r || (r as { ok?: boolean }).ok !== true) queueClientError(payload)
      })
      .catch(() => queueClientError(payload))
  } catch {
    queueClientError(payload)
  }
}

export async function flushClientErrorQueue(): Promise<void> {
  const q = readQueue()
  if (!q.length) return
  const remaining: ClientErrorInput[] = []
  for (const item of q) {
    let ok = false
    try {
      const r = await reportClientError(item as Record<string, unknown>)
      ok = !!(r && (r as { ok?: boolean }).ok)
    } catch {
      ok = false
    }
    if (!ok) remaining.push(item)
  }
  writeQueue(remaining)
}

export function installErrorQueueFlusher(): void {
  void flushClientErrorQueue()
  window.addEventListener('online', () => void flushClientErrorQueue())
}
