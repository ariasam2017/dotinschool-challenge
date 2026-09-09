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

export interface HeroAppearance {
  title?: string
  subtitle?: string
  showSubtitle?: boolean
  align?: '' | 'start' | 'center' | 'end'
  background?: string
  image?: string
  minHeight?: string
}
export interface HeaderAppearance {
  brand?: string
  background?: string
}

export interface TextReplacement {
  find: string
  replace: string
  mode?: 'exact' | 'contains'
}

export interface StyleBag {
  fontSize?: string
  fontWeight?: string
  textAlign?: string
  color?: string
  background?: string
  padding?: string
  margin?: string
  borderWidth?: string
  borderColor?: string
  borderRadius?: string
  width?: string
  maxWidth?: string
}

export interface ElementOverride {
  selector: string
  label?: string
  setText?: boolean
  text?: string
  textMobile?: string
  textDesktop?: string
  hidden?: boolean
  hiddenMobile?: boolean
  hiddenDesktop?: boolean
  all?: StyleBag
  mobile?: StyleBag
  desktop?: StyleBag
  // legacy flat fields (read-only, folded into `all` server-side)
  fontSize?: string
  fontWeight?: string
  textAlign?: string
  color?: string
  background?: string
  padding?: string
  margin?: string
  borderWidth?: string
  borderColor?: string
  borderRadius?: string
  width?: string
  maxWidth?: string
}

/** The admin Design Studio layer applied to the user panel at runtime. */
export interface Appearance {
  tokens?: Record<string, string>
  dark?: Record<string, string>
  sections?: {
    hero?: HeroAppearance
    header?: HeaderAppearance
  }
  customCss?: string
  texts?: TextReplacement[]
  overrides?: ElementOverride[]
  updatedAt?: number
}

export interface EventConfig {
  eventEndsAt: number | null
  eventStartsAt: number | null
  eventPaused: boolean
  pausedRemainMs: number | null
  challenge: Challenge
  appearance?: Appearance
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

// Live host resource snapshot for the admin overview "منابع سرور" card.
export interface ResourceMeter {
  used: number // bytes
  total: number // bytes
  percent: number // 0..100
}
export interface SystemStats {
  cpu: { percent: number; cores: number }
  mem: ResourceMeter
  swap: ResourceMeter
  uptime: number // host uptime, seconds
  load: [number, number, number] // 1/5/15-min load avg (0 on Windows)
  go: { goroutines: number; alloc: number }
  at: number // epoch ms
}
