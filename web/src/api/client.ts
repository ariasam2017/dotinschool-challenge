// Thin fetch client over the Go REST API. One-to-one with the endpoints the
// original store.js talked to, including error-code semantics.

import { adminToken } from './session'
import type {
  AiDetectResult,
  AiSettings,
  AiStatus,
  AiProviderInput,
  Appearance,
  ClientErrorList,
  EventConfig,
  Participant,
  ReviewResult,
  SessionParticipant,
  SmsInfo,
  ParticipantExportFile,
  ParticipantImportResult,
  SmsSettings,
  SmsSettingsInput,
  SmsTestResult,
  SystemStats,
  UpdateResult,
} from './types'

export class ApiError extends Error {
  code: string
  status: number
  constructor(message: string, code: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

interface FetchOpts {
  method?: string
  body?: unknown
}

export async function apiFetch<T = unknown>(path: string, opts: FetchOpts = {}): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = adminToken.get()
  if (token) headers['Authorization'] = 'Bearer ' + token

  let res: Response
  try {
    res = await fetch(path, {
      method: opts.method || 'GET',
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    })
  } catch {
    throw new ApiError(
      'ارتباط با سرور برقرار نشد. بررسی کنید: آدرس و پورت درست باشد (همان که پنل روی آن باز شده)، فیلترشکن/پروکسی سیستم روی «دور زدن آدرس‌های محلی» تنظیم باشد، و افزونه‌های مسدودکننده (ad-blocker) روی این صفحه غیرفعال باشند. سپس صفحه را با Ctrl+Shift+R تازه کنید.',
      'NETWORK_ERROR',
      0,
    )
  }

  let data: any = null
  try {
    data = await res.json()
  } catch {
    /* empty / non-JSON body */
  }
  if (!res.ok) {
    throw new ApiError(
      (data && data.message) || `درخواست ناموفق بود (${res.status})`,
      (data && data.code) || 'UNKNOWN_ERROR',
      res.status,
    )
  }
  return data as T
}

/* ---- participant auth + progress ---- */
export const requestOtp = (phone: string, purpose: 'register' | 'login') =>
  apiFetch<{ ok: true }>('/api/otp/request', { method: 'POST', body: { phone, purpose } })

export const registerParticipant = (payload: {
  firstName: string
  lastName: string
  phone: string
  otp: string
}) => apiFetch<SessionParticipant>('/api/register', { method: 'POST', body: payload })

export const lookupByPhone = (phone: string, otp: string) =>
  apiFetch<SessionParticipant>('/api/lookup', { method: 'POST', body: { phone, otp } })

export const resumeSession = (token: string) =>
  apiFetch<SessionParticipant>('/api/session', { method: 'POST', body: { token } })

export const updateParticipant = (id: string, patch: Record<string, unknown>) =>
  apiFetch<Participant>('/api/participants/' + encodeURIComponent(id), { method: 'PATCH', body: patch })

/* ---- config (event + challenge) ---- */
export const getConfig = () => apiFetch<EventConfig>('/api/config')
export const getVersion = () => apiFetch<{ version: string }>('/api/version')
export const setConfig = (patch: Partial<EventConfig> | { challenge: unknown }) =>
  apiFetch<EventConfig>('/api/config', { method: 'PUT', body: patch })

/* ---- admin auth ---- */
export async function adminLogin(username: string, password: string): Promise<boolean> {
  try {
    const { token, username: u } = await apiFetch<{ token: string; username: string }>(
      '/api/admin/login',
      { method: 'POST', body: { username, password } },
    )
    adminToken.set(token, u)
    return true
  } catch (e) {
    if (e instanceof ApiError && e.code === 'INVALID_CREDENTIALS') return false
    throw e
  }
}
export async function adminLogout(): Promise<void> {
  try {
    await apiFetch('/api/admin/logout', { method: 'POST' })
  } catch {
    /* ignore */
  }
  adminToken.clear()
}

/* ---- participants (admin) ---- */
export const listParticipants = () => apiFetch<Participant[]>('/api/participants')
export const deleteParticipant = (id: string) =>
  apiFetch<{ ok: true }>('/api/participants/' + encodeURIComponent(id), { method: 'DELETE' })

/* ---- backup / restore / update ---- */
export async function downloadBackup(): Promise<Blob> {
  const token = adminToken.get()
  let res: Response
  try {
    res = await fetch('/api/admin/backup', {
      headers: token ? { Authorization: 'Bearer ' + token } : {},
    })
  } catch {
    throw new ApiError('اتصال به سرور برقرار نشد.', 'NETWORK_ERROR', 0)
  }
  if (!res.ok) {
    let data: any = null
    try {
      data = await res.json()
    } catch {
      /* ignore */
    }
    throw new ApiError(
      (data && data.message) || 'دریافت بکاپ ناموفق بود',
      (data && data.code) || 'UNKNOWN_ERROR',
      res.status,
    )
  }
  return res.blob()
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('خطا در خواندن فایل'))
    reader.onload = () => {
      const result = String(reader.result)
      const idx = result.indexOf(',')
      resolve(idx >= 0 ? result.slice(idx + 1) : result)
    }
    reader.readAsDataURL(file)
  })
}
export async function restoreBackup(file: File) {
  const data = await fileToBase64(file)
  return apiFetch<{ ok: true; restarting: true }>('/api/admin/backup/restore', {
    method: 'POST',
    body: { data },
  })
}
export const checkForUpdate = () =>
  apiFetch<UpdateResult>('/api/admin/update', { method: 'POST' })

/* ---- portable participant transfer (server -> server) ---- */
export async function exportParticipants(): Promise<Blob> {
  const token = adminToken.get()
  let res: Response
  try {
    res = await fetch('/api/admin/participants/export', {
      headers: token ? { Authorization: 'Bearer ' + token } : {},
    })
  } catch {
    throw new ApiError('اتصال به سرور برقرار نشد.', 'NETWORK_ERROR', 0)
  }
  if (!res.ok) {
    let data: any = null
    try {
      data = await res.json()
    } catch {
      /* ignore */
    }
    throw new ApiError(
      (data && data.message) || 'دریافت خروجی شرکت‌کنندگان ناموفق بود',
      (data && data.code) || 'UNKNOWN_ERROR',
      res.status,
    )
  }
  return res.blob()
}
export const importParticipants = (file: ParticipantExportFile) =>
  apiFetch<ParticipantImportResult>('/api/admin/participants/import', {
    method: 'POST',
    body: file,
  })

/* ---- host resources (admin overview) ---- */
export const systemStats = () => apiFetch<SystemStats>('/api/admin/system')

/* ---- SMS (ippanel) diagnostics + panel-managed settings ---- */
export const smsStatus = () => apiFetch<SmsInfo>('/api/admin/sms-status')
export const smsTest = (payload: { phone: string; purpose?: 'register' | 'login'; pattern?: string }) =>
  apiFetch<SmsTestResult>('/api/admin/sms-test', { method: 'POST', body: payload })
export const getSmsSettings = () => apiFetch<SmsSettings>('/api/admin/sms-settings')
export const saveSmsSettings = (payload: SmsSettingsInput) =>
  apiFetch<SmsSettings>('/api/admin/sms-settings', { method: 'PUT', body: payload })

/* ---- Design Studio / appearance (admin) ---- */
export const getAppearance = () =>
  apiFetch<{ draft: Appearance; published: Appearance }>('/api/admin/appearance')
export const saveAppearanceDraft = (a: Appearance) =>
  apiFetch<{ draft: Appearance }>('/api/admin/appearance', { method: 'PUT', body: a })
export const publishAppearance = () =>
  apiFetch<{ published: Appearance }>('/api/admin/appearance/publish', { method: 'POST' })
export const revertAppearance = () =>
  apiFetch<{ draft: Appearance }>('/api/admin/appearance/revert', { method: 'POST' })
export const resetAppearance = () =>
  apiFetch<{ draft: Appearance; published: Appearance }>('/api/admin/appearance/reset', {
    method: 'POST',
  })

/* ---- AI assistant (admin) ---- */
export const aiStatus = () => apiFetch<AiStatus>('/api/admin/ai-status')
export const getAiSettings = () => apiFetch<AiSettings>('/api/admin/ai-settings')
export const saveAiSettings = (payload: { activeId: string; providers: AiProviderInput[] }) =>
  apiFetch<{ activeId: string; providers: AiSettings['providers'] }>('/api/admin/ai-settings', {
    method: 'PUT',
    body: payload,
  })
export const testAiProvider = (id?: string) =>
  apiFetch<{ ok: true; model: string; sample: string }>('/api/admin/ai-settings/test', {
    method: 'POST',
    body: id ? { id } : {},
  })
export const detectAiProvider = (apiKey: string) =>
  apiFetch<AiDetectResult>('/api/admin/ai-detect', { method: 'POST', body: { apiKey } })
export const reviewCode = (filename: string, code: string) =>
  apiFetch<ReviewResult>('/api/admin/review-code', { method: 'POST', body: { filename, code } })

/* ---- client error reports ---- */
export function reportClientError(payload: Record<string, unknown>) {
  return apiFetch<{ ok: true }>('/api/client-errors', { method: 'POST', body: payload }).catch(
    () => null,
  )
}
export const listClientErrors = () => apiFetch<ClientErrorList>('/api/admin/client-errors')
export const markClientErrorsSeen = () =>
  apiFetch<{ ok: true }>('/api/admin/client-errors/seen', { method: 'POST' })
export const deleteClientError = (id: string) =>
  apiFetch<{ ok: true }>('/api/admin/client-errors/' + encodeURIComponent(id), { method: 'DELETE' })
export const clearClientErrors = () =>
  apiFetch<{ ok: true }>('/api/admin/client-errors', { method: 'DELETE' })
