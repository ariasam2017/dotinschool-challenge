// Wire types — the exact shapes the Go API returns.

export interface Participant {
  id: string
  publicId: string
  firstName: string
  lastName: string
  phone: string
  telegram: string
  registeredAt: number
  status: 'registered' | 'started' | 'submitted'
  score: number | null
  code: string | null
  aiFlag: 'human' | 'suspicious' | null
  execMs: number | null
  execDepth: number | null
  aiScore: number | null
}

export interface SessionParticipant extends Participant {
  sessionToken: string
}

export interface Challenge {
  title: string
  diff: 'easy' | 'medium' | 'hard'
  tag: string
  desc: string
  sampleIn: string
  sampleOut: string
  image: string | null
  mode: 'correctness' | 'performance'
  timeLimitMs: number
  submitMessage: string
}

export interface EventConfig {
  eventEndsAt: number | null
  eventStartsAt: number | null
  eventPaused: boolean
  pausedRemainMs: number | null
  challenge: Challenge
}

export interface AiStatus {
  configured: boolean
  model: string
  baseUrl: string
  source: 'panel' | 'env' | null
  label: string | null
}

export interface AiProviderMasked {
  id: string
  label: string
  baseUrl: string
  model: string
  keyMask: string
  hasKey: boolean
}

export interface AiPreset {
  label: string
  baseUrl: string
  model: string
}

export interface AiSettings {
  activeId: string
  providers: AiProviderMasked[]
  presets: AiPreset[]
  env: { present: boolean; baseUrl: string; model: string }
  defaults: { baseUrl: string; model: string }
}

export interface AiProviderInput {
  id: string
  label: string
  baseUrl: string
  model: string
  apiKey?: string
}

export interface ReviewResult {
  review: string
  aiScore: number | null
  model: string
  usage: { total_tokens?: number; prompt_tokens?: number; completion_tokens?: number } | null
}

export interface AiDetectResult {
  method: 'signature' | 'probe'
  match: { label: string; baseUrl: string; model: string } | null
  multiple?: string[]
  reason?: 'rejected-everywhere' | 'unreachable' | 'bad-url'
}

export interface ClientErrorRow {
  id: string
  createdAt: number
  participantId: string
  publicId: string
  name: string
  context: string
  errorCode: string
  errorMessage: string
  codeExcerpt: string
  userAgent: string
  seen: boolean
}

export interface ClientErrorList {
  unseen: number
  errors: ClientErrorRow[]
}

export interface UpdateResult {
  updated: boolean
  message: string
}

export interface SmsInfo {
  configured: boolean
  provider: string
  endpoint: string
  sender: string
  registerPattern: string
  loginPattern: string
  adminPattern: string
  adminPhone: string
  authScheme: string
  source: 'panel' | 'env' | ''
}

export interface SmsTestResult {
  ok: boolean
  status: number
  endpoint: string
  message: string
  rawBody: string
}

// The panel-managed override — every field optional; blank falls back to the
// matching IPPANEL_* env var (see SmsInfo for what's actually in effect).
export interface SmsSettings {
  hasKey: boolean
  keyMask: string
  sender: string
  registerPattern: string
  loginPattern: string
  adminNotifyPattern: string
  adminPhone: string
  baseUrl: string
  authScheme: string
}

export interface SmsSettingsInput {
  apiKey?: string
  sender?: string
  registerPattern?: string
  loginPattern?: string
  adminNotifyPattern?: string
  adminPhone?: string
  baseUrl?: string
  authScheme?: string
}
