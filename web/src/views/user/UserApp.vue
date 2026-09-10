<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import { useParticipantStore } from '@/stores/participant'
import { useEventStore } from '@/stores/event'
import { ApiError } from '@/api/client'
import { installErrorQueueFlusher, submitClientError } from '@/lib/errorQueue'
import { estimateExecution, performanceScore, countErrors, STARTER_CODE } from '@/lib/estimate'
import { fmtJalaliDateTime } from '@/lib/fa'
import { appearanceText } from '@/lib/appearance'

import AuthPanel from './AuthPanel.vue'
import ChallengeCard from '@/components/ChallengeCard.vue'
import CountdownClock from '@/components/CountdownClock.vue'
import JavaEditor from '@/components/JavaEditor.vue'
import type { EditorProblem } from '@/components/editor-types'
import ThemeToggle from '@/components/ThemeToggle.vue'
import AppIcon from '@/components/AppIcon.vue'
import Chip from '@/components/Chip.vue'
import { toFa } from '@/lib/fa'

const pStore = useParticipantStore()
const eStore = useEventStore()
const { user } = storeToRefs(pStore)
const { challenge, state: eventState, remainMs, startsInMs, canCompete } = storeToRefs(eStore)

const booted = ref(false)
const code = ref(STARTER_CODE)
const submitState = ref<'idle' | 'submitting' | 'submitted' | 'error'>('idle')
const submitError = ref('')
let startedSent = false

// The countdown-to-start is shown to participants only inside the final hour
// before the scheduled start; before that they just see a "not started" notice.
const START_COUNTDOWN_WINDOW_MS = 60 * 60 * 1000

const problems = ref<EditorProblem[]>([])
const cursor = ref({ line: 1, col: 1 })
const errCount = computed(() => problems.value.filter((p) => p.severity === 'error').length)
const warnCount = computed(() => problems.value.filter((p) => p.severity === 'warning').length)

const RULES = [
  'تایمر رویداد پس از شروع متوقف نمی‌شود.',
  'خطاهای نحوی کدت زنده، زیر ادیتور نشان داده می‌شود.',
  'تا پایان زمان می‌توانی کد را ویرایش و چند بار ارسال کنی.',
]

function resetCode() {
  code.value = STARTER_CODE
}

const LOCK_MESSAGES: Record<string, string> = {
  waiting: 'چالش هنوز طبق زمان‌بندی شروع نشده؛ نوشتن و ارسال کد ممکن نیست',
  paused: 'رویداد توسط مدیر متوقف شده؛ نوشتن و ارسال کد ممکن نیست',
  ended: 'زمان رویداد به پایان رسید؛ ارسال دیگر ممکن نیست',
}
const lockReason = computed(() => (eventState.value === 'running' ? '' : eventState.value))
const clockState = computed(() =>
  eventState.value === 'waiting' ? 'waiting' : eventState.value === 'paused' ? 'paused' : eventState.value === 'ended' ? 'ended' : 'running',
)

const initials = computed(() =>
  user.value ? (user.value.firstName[0] || '') + (user.value.lastName[0] || '') : '',
)
const resultMessage = computed(
  () => challenge.value?.submitMessage?.trim() || 'کدت با موفقیت ثبت شد. پس از بررسی مدیر سامانه، نتیجه‌ی چالش به شما اعلام خواهد شد.',
)

// Show the countdown-to-start only inside the final hour before the scheduled
// start time. Outside that window a plain "not started" notice is shown.
const showStartCountdown = computed(
  () => eventState.value === 'waiting' && startsInMs.value > 0 && startsInMs.value <= START_COUNTDOWN_WINDOW_MS,
)

// The participant's screen follows the live event state — it is NOT sticky.
// The moment the admin pauses or the event ends, the challenge + editor are
// replaced with the matching notice (within one poll, ~1s). Any code the
// participant had typed stays in `code` (in memory) and is restored if the
// event resumes.
type UserView = 'notStarted' | 'countdown' | 'running' | 'paused' | 'ended'
const userView = computed<UserView>(() => {
  switch (eventState.value) {
    case 'ended':
      return 'ended'
    case 'paused':
      return 'paused'
    case 'waiting':
      return showStartCountdown.value ? 'countdown' : 'notStarted'
    default:
      return 'running'
  }
})

// Record status=started the first time this participant sees the live arena.
watch(
  [userView, user],
  ([view, u]) => {
    if (view === 'running' && u && !startedSent && u.status === 'registered') {
      startedSent = true
      void pStore.patchQuietly({ status: 'started' })
    }
  },
  { immediate: true },
)

onMounted(async () => {
  installErrorQueueFlusher()
  eStore.start()
  const resumed = await pStore.resume()
  if (resumed) {
    message.success(`خوش آمدید دوباره، ${resumed.firstName}`)
    if (resumed.code) code.value = resumed.code
    if (resumed.status === 'submitted') submitState.value = 'submitted'
    if (resumed.status !== 'registered') startedSent = true
  }
  booted.value = true
})
onBeforeUnmount(() => eStore.stop())

function onAuthed(status: string) {
  if (user.value?.code) code.value = user.value.code
  if (status !== 'registered') startedSent = true
}

async function submit() {
  if (!user.value) return
  if (!canCompete.value) {
    message.error(LOCK_MESSAGES[lockReason.value] || 'ارسال ممکن نیست')
    return
  }
  submitState.value = 'submitting'
  submitError.value = ''

  const src = code.value
  const errs = countErrors(src)
  const mode = challenge.value?.mode || 'correctness'
  let score: number
  let execMs: number | null = null
  let execDepth: number | null = null

  if (mode === 'performance') {
    const target = challenge.value?.timeLimitMs || 200
    const est = estimateExecution(src)
    if (errs > 0) est.ms += 600 + Math.random() * 600
    execMs = Math.round(est.ms)
    execDepth = est.maxDepth
    const v = performanceScore(est.ms, target)
    score = errs > 0 ? Math.min(v, 15) : v
  } else {
    const total = 8
    const passed = errs > 0 ? Math.floor(Math.random() * 2) : 5 + Math.floor(Math.random() * 4)
    score = Math.round((passed / total) * 100)
  }

  try {
    await pStore.patch({ status: 'submitted', score, code: src, execMs, execDepth })
    submitState.value = 'submitted'
  } catch (e) {
    const err = e as ApiError
    if (err.code !== 'EVENT_LOCKED' && err.code !== 'EVENT_NOT_STARTED') {
      submitClientError({
        participantId: user.value.id,
        sessionToken: user.value.sessionToken,
        publicId: user.value.publicId,
        name: `${user.value.firstName} ${user.value.lastName}`,
        context: 'submit',
        errorCode: err.code || 'UNKNOWN',
        errorMessage: err.message || 'خطای نامشخص',
        code: src.slice(0, 6000),
      })
    }
    submitState.value = 'error'
    submitError.value = friendlyError(err)
  }
}

function friendlyError(err: ApiError): string {
  if (err.code === 'NETWORK_ERROR')
    return 'ارسال کد انجام نشد. اتصال اینترنت را بررسی کن و دوباره تلاش کن.'
  if (err.code === 'INTERNAL_ERROR' || err.status >= 500)
    return 'ارسال کد در این لحظه ممکن نشد. چند لحظه صبر کن و دوباره تلاش کن.'
  return err.message || 'ارسال کد با خطا مواجه شد. دوباره تلاش کن.'
}

function confirmExit() {
  const midChallenge = userView.value === 'running' && user.value?.status !== 'submitted'
  Modal.confirm({
    title: 'خروج از حساب',
    content: midChallenge
      ? 'کدی که هنوز ارسال نکرده‌اید ذخیره نمی‌شود و از دست می‌رود. برای ورود دوباره باید شماره موبایل خود را وارد کنید.'
      : 'برای ورود دوباره باید شماره موبایل خود را وارد کنید.',
    okText: 'خروج',
    cancelText: 'ماندن',
    okType: 'danger',
    onOk: () => {
      pStore.signOut()
      location.reload()
    },
  })
}
</script>

<template>
  <div class="user-shell">
    <header class="u-header">
      <div class="brand">
        <span class="brand-mark"><AppIcon name="code" :size="18" /></span>
        <span class="brand-name">{{ appearanceText.brand || 'dotinschool' }}</span>
      </div>
      <div class="grow" />
      <div v-if="user" class="identity">
        <span class="avatar">{{ initials }}</span>
        <span class="identity-text">
          <span class="identity-name">{{ user.firstName }} {{ user.lastName }}</span>
          <span class="identity-id num">{{ user.publicId }}</span>
        </span>
        <button type="button" class="ghost-btn" title="خروج" @click="confirmExit">
          <AppIcon name="log-out" :size="16" />
        </button>
      </div>
      <ThemeToggle />
    </header>

    <main class="u-main" :class="{ 'u-main--auth': !user }">
      <!-- not signed in: auth card, then the one-line headline beneath it.
           .hero / .hero-copy / .hero-sub class names are kept because the admin
           Design Studio targets them (lib/appearance.ts). -->
      <section v-if="!user" class="hero">
        <div class="hero-copy">
          <h1>{{ appearanceText.heroTitle || 'سوال را ببین. کد را بزن. ارسال کن.' }}</h1>
          <p
            v-if="appearanceText.heroSubtitle && appearanceText.heroShowSubtitle"
            class="hero-sub"
          >
            {{ appearanceText.heroSubtitle }}
          </p>
        </div>
        <AuthPanel @authed="onAuthed" />
      </section>

      <!-- signed in — the screen always follows the live event state. Wrapped in
           a fade so the switch between screens (esp. countdown -> live arena
           the instant it starts) is a soft cross-fade instead of a hard cut;
           each branch needs its own :key or Vue reuses the <section> in place
           and no transition fires. -->
      <template v-else>
      <Transition name="fade" mode="out-in">
        <!-- 1) scheduled, more than an hour away: no challenge, no timer.
             Left-aligned editorial panel; the start time is the prominent stat. -->
        <section v-if="userView === 'notStarted'" key="notStarted" class="state-panel tone-neutral">
          <p class="state-kicker"><AppIcon name="calendar" :size="14" /> زمان‌بندی رویداد</p>
          <h2 class="state-title">رویداد هنوز شروع نشده</h2>
          <p class="state-lead">
            زمان برگزاری را مدیر سامانه تعیین می‌کند. لحظهٔ شروع، سوال چالش خودکار همین‌جا باز می‌شود.
          </p>
          <div v-if="eStore.config?.eventStartsAt" class="state-stat">
            <span class="state-stat-label">شروع برنامه‌ریزی‌شده</span>
            <span class="state-stat-value num">{{ fmtJalaliDateTime(eStore.config.eventStartsAt) }}</span>
          </div>
          <ul class="rules-list state-rules">
            <li v-for="(r, i) in RULES" :key="i">
              <AppIcon name="check-circle" :size="15" />
              <span>{{ r }}</span>
            </li>
          </ul>
        </section>

        <!-- 2) final hour before the scheduled start: the clock IS the screen. -->
        <section v-else-if="userView === 'countdown'" key="countdown" class="state-panel tone-info is-centered">
          <p class="state-kicker"><AppIcon name="clock" :size="14" /> تا شروع رویداد</p>
          <h2 class="state-title">به‌زودی شروع می‌شود</h2>
          <CountdownClock :ms="startsInMs" state="waiting" size="xl" label="زمان باقی‌مانده تا شروع" />
          <p v-if="eStore.config?.eventStartsAt" class="state-foot num">
            شروع خودکار در {{ fmtJalaliDateTime(eStore.config.eventStartsAt) }}
          </p>
        </section>

        <!-- 3) admin paused a live event. -->
        <section v-else-if="userView === 'paused'" key="paused" class="state-panel tone-warning">
          <p class="state-kicker"><AppIcon name="pause" :size="14" /> رویداد متوقف است</p>
          <h2 class="state-title">مدیر رویداد را موقتاً نگه داشته</h2>
          <p class="state-lead">
            سوال و ادیتور تا ادامهٔ رویداد پنهان می‌شوند. کدی که نوشته‌ای محفوظ است و برمی‌گردد.
          </p>
          <CountdownClock :ms="remainMs" state="paused" size="lg" label="زمان باقی‌ماندهٔ چالش، متوقف" />
        </section>

        <!-- 4) event finished: the verdict is the prominent element. -->
        <section v-else-if="userView === 'ended'" key="ended" class="state-panel tone-danger">
          <p class="state-kicker"><AppIcon name="clock" :size="14" /> پایان رویداد</p>
          <h2 class="state-title">زمان به پایان رسید</h2>
          <p
            v-if="user.status === 'submitted'"
            class="state-verdict ok"
          >
            <AppIcon name="check-circle" :size="18" />
            کدت ثبت شد. نتیجه پس از بررسی مدیر سامانه به تو اعلام می‌شود.
          </p>
          <p v-else-if="user.status === 'started'" class="state-verdict bad">
            <AppIcon name="alert-circle" :size="18" />
            کدی از تو پیش از پایان مهلت ثبت نشد.
          </p>
          <p v-else class="state-verdict bad">
            <AppIcon name="alert-circle" :size="18" />
            در این رویداد کدی ارسال نکردی.
          </p>
        </section>

        <!-- 5) event live: arena (challenge + editor) — no start button -->
        <section v-else-if="challenge" key="arena" class="arena">
          <aside class="arena-brief card card-pad">
            <ChallengeCard :challenge="challenge" compact />
            <ul class="rules-list arena-rules">
              <li v-for="(r, i) in RULES" :key="i">
                <AppIcon name="check-circle" :size="14" />
                <span>{{ r }}</span>
              </li>
            </ul>
          </aside>

          <div class="arena-editor">
            <div class="editor-toolbar card">
              <span class="file-tab"><AppIcon name="file-code" :size="14" /> Main.java</span>
              <span class="lang-badge">Java 17</span>
              <a-button size="small" :disabled="!canCompete" @click="resetCode">
                <template #icon><AppIcon name="rotate-ccw" :size="13" /></template>
                بازنشانی کد
              </a-button>
              <div class="grow" />
              <Chip v-if="!canCompete" tone="danger" dot>{{ LOCK_MESSAGES[lockReason] }}</Chip>
              <CountdownClock :ms="remainMs" :state="clockState" size="md" />
            </div>

            <JavaEditor
              v-model="code"
              :disabled="!canCompete"
              class="editor-host"
              @problems="problems = $event"
              @cursor="cursor = $event"
            />

            <div class="problems-panel card">
              <div class="problems-head">
                <span>مشکلات</span>
                <span class="cnt cnt-err">{{ toFa(errCount) }} خطا</span>
                <span class="cnt cnt-warn">{{ toFa(warnCount) }} هشدار</span>
                <div class="grow" />
                <span class="status-bit num">خط {{ toFa(cursor.line) }}:{{ toFa(cursor.col) }}</span>
                <span class="status-bit">فاصله‌گذاری: {{ toFa(4) }}</span>
              </div>
              <div class="problems-list">
                <div v-if="!problems.length" class="problems-empty">
                  <AppIcon name="check-circle" :size="14" /> مشکلی یافت نشد
                </div>
                <div
                  v-for="(p, i) in problems"
                  :key="i"
                  class="problem-row"
                  :class="p.severity"
                >
                  <AppIcon :name="p.severity === 'error' ? 'alert-circle' : 'alert-triangle'" :size="13" />
                  <span class="grow">{{ p.message }}</span>
                  <span class="loc num">{{ toFa(p.line) }}:{{ toFa(p.col) }}</span>
                </div>
              </div>
            </div>

            <div class="submit-bar card card-pad">
              <div class="grow">
                <p class="submit-hint faint">
                  {{ canCompete ? 'می‌توانید تا پایان زمان، کد را ویرایش و چند بار ارسال کنید.' : LOCK_MESSAGES[lockReason] }}
                </p>
                <div v-if="submitState === 'submitted'" class="result-panel">
                  <div class="result-head">
                    <Chip tone="success" dot>ارسال شد</Chip>
                    <span class="faint">کدت برای بررسی ثبت شد</span>
                  </div>
                  <p class="result-msg">{{ resultMessage }}</p>
                </div>
                <a-alert v-else-if="submitState === 'error'" type="error" :message="submitError" show-icon banner />
              </div>
              <a-button
                type="primary"
                size="large"
                :loading="submitState === 'submitting'"
                :disabled="!canCompete"
                @click="submit"
              >
                <template #icon><AppIcon name="send" :size="16" /></template>
                {{ submitState === 'submitted' ? 'ارسال مجدد' : submitState === 'error' ? 'تلاش مجدد' : 'ارسال کد' }}
              </a-button>
            </div>
          </div>
        </section>
      </Transition>
      </template>
    </main>
  </div>
</template>

<style scoped>
/* Soft cross-fade between the event-state screens (esp. countdown -> the
   live arena the instant it starts) — a small fade + rise reads better than
   a hard cut for something that switches on its own with no user action. */
.fade-enter-active,
.fade-leave-active {
  transition:
    opacity 0.35s ease,
    transform 0.35s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(6px);
}
.user-shell {
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
}
.u-header {
  position: sticky;
  top: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  height: var(--header-h);
  padding: 0 var(--space-5);
  background: color-mix(in srgb, var(--bg) 88%, transparent);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border);
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}
.brand-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: var(--primary);
  color: var(--on-primary);
}
.brand-name {
  font-weight: 800;
  font-size: 1.05rem;
  letter-spacing: -0.01em;
}
.identity {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 5px 6px 5px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-pill);
  background: var(--surface);
}
.avatar {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: var(--primary-soft);
  color: var(--primary-bright);
  font-weight: 700;
  font-size: 0.8rem;
}
.identity-text {
  display: flex;
  flex-direction: column;
  line-height: 1.25;
}
.identity-name {
  font-size: 0.82rem;
  font-weight: 600;
}
.identity-id {
  font-size: 0.68rem;
  color: var(--text-faint);
}
.ghost-btn {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--text-faint);
}
.ghost-btn:hover {
  color: var(--danger);
  background: var(--danger-soft);
}
.u-main {
  flex: 1;
  width: 100%;
  max-width: 1180px;
  margin: 0 auto;
  padding: var(--space-6) var(--space-4) var(--space-8);
}
@media (min-width: 700px) {
  .u-main {
    padding: var(--space-6) var(--space-5) var(--space-8);
  }
}
/* Signed-out screen: fit inside the visible viewport so the page never scrolls.
   `margin:auto` centres the block but leaves the top reachable if it ever
   overflows on a very short screen; the auth card scrolls inside itself as a
   last resort. */
.u-main--auth {
  min-height: 0;
  height: calc(100dvh - var(--header-h));
  display: flex;
  flex-direction: column;
  padding-block: var(--space-4);
  overflow-y: auto;
}
.u-header {
  overflow: hidden;
}

/* ---- signed-out: auth card, then the one-line headline beneath it ---- */
.hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-4);
  width: 100%;
  max-width: 480px;
  margin: auto;
}
.hero > * {
  width: 100%;
  min-width: 0;
}
.hero-copy {
  text-align: center;
}
.hero-copy h1 {
  margin: 0;
  font-size: clamp(1.25rem, 4.5vw, 1.55rem);
  font-weight: 800;
  line-height: 1.5;
  letter-spacing: -0.01em;
  text-wrap: balance;
}
.hero-sub {
  margin: var(--space-2) 0 0;
  color: var(--text-dim);
  font-size: 0.92rem;
  line-height: 1.9;
}
@media (max-height: 680px) {
  .u-main--auth .hero {
    gap: var(--space-3);
  }
  .u-main--auth .hero-copy h1 {
    font-size: 1.15rem;
    line-height: 1.4;
  }
}
/* ---- event-state screens (not started / countdown / paused / ended) ----
   One panel, four distinct compositions. Tone drives an inline-start accent
   bar + the kicker colour; the countdown variant centres and leads with the
   clock so the four screens do not read as one template. */
.state-panel {
  --tone: var(--text-dim);
  --tone-soft: var(--surface-2);
  position: relative;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  max-width: 600px;
  margin: clamp(var(--space-5), 8vh, var(--space-8)) auto 0;
  padding: var(--space-6);
  padding-inline-start: calc(var(--space-6) + 4px);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-1);
}
.state-panel::before {
  content: '';
  position: absolute;
  inset-block: var(--space-5);
  inset-inline-start: 0;
  width: 4px;
  border-radius: var(--radius-pill);
  background: var(--tone);
}
.state-panel.tone-neutral {
  --tone: var(--text-faint);
}
.state-panel.tone-info {
  --tone: var(--info);
  --tone-soft: var(--info-soft);
}
.state-panel.tone-warning {
  --tone: var(--warning);
  --tone-soft: var(--warning-soft);
}
.state-panel.tone-danger {
  --tone: var(--danger);
  --tone-soft: var(--danger-soft);
}
.state-panel.is-centered {
  align-items: center;
  text-align: center;
}
.state-kicker {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin: 0;
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: var(--tone);
}
.state-title {
  margin: 0;
  font-size: clamp(1.35rem, 4.5vw, 1.7rem);
  font-weight: 800;
  line-height: 1.35;
  letter-spacing: -0.01em;
  color: var(--text);
}
.state-lead {
  margin: 0;
  max-width: 52ch;
  font-size: 0.92rem;
  line-height: 2;
  color: var(--text-dim);
}
.state-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface-2);
}
.state-stat-label {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-faint);
}
.state-stat-value {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text);
  font-variant-numeric: tabular-nums;
}
.state-foot {
  margin: 0;
  font-size: 0.78rem;
  color: var(--text-faint);
  font-variant-numeric: tabular-nums;
}
.state-verdict {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0;
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius);
  font-size: 0.9rem;
  font-weight: 600;
  line-height: 1.8;
}
.state-verdict :deep(.app-icon) {
  flex: none;
}
.state-verdict.ok {
  background: var(--success-soft);
  color: var(--success);
}
.state-verdict.bad {
  background: var(--danger-soft);
  color: var(--danger);
}
.state-rules {
  margin-top: var(--space-2) !important;
  padding-top: var(--space-4) !important;
  border-top: 1px solid var(--border);
}
.arena {
  display: grid;
  gap: var(--space-4);
}
@media (min-width: 1024px) {
  .arena {
    grid-template-columns: 340px 1fr;
    align-items: start;
  }
  .arena-brief {
    position: sticky;
    top: calc(var(--header-h) + 16px);
    max-height: calc(100dvh - var(--header-h) - 32px);
    overflow-y: auto;
  }
}
.arena-editor {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-width: 0;
}
.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  flex-wrap: wrap;
}
.file-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-dim);
}
.lang-badge {
  font-size: 0.68rem;
  font-weight: 700;
  color: var(--accent);
  background: var(--accent-soft);
  padding: 2px 8px;
  border-radius: var(--radius-pill);
}
.editor-host {
  height: min(58vh, 540px);
}
.arena-rules {
  margin-top: var(--space-4) !important;
  padding-top: var(--space-3) !important;
  border-top: 1px solid var(--border);
}
.arena-rules li {
  font-size: 0.74rem;
  gap: 6px;
}
.rules-list {
  list-style: none;
  margin: 10px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.rules-list li {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 0.8rem;
  line-height: 1.8;
  color: var(--text-dim);
}
.rules-list li :deep(.app-icon) {
  color: var(--success);
  margin-top: 3px;
}
.problems-panel {
  padding: 0;
  overflow: hidden;
}
.problems-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--text-dim);
}
.cnt-err {
  color: var(--danger);
}
.cnt-warn {
  color: var(--warning);
}
.status-bit {
  font-size: 0.7rem;
  color: var(--text-faint);
  font-weight: 500;
}
.problems-list {
  max-height: 160px;
  overflow-y: auto;
}
.problems-empty {
  padding: 14px;
  font-size: 0.78rem;
  color: var(--text-faint);
}
.problem-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  font-size: 0.78rem;
  border-bottom: 1px solid var(--border);
}
.problem-row:last-child {
  border-bottom: none;
}
.problem-row.error {
  color: var(--danger);
}
.problem-row.warning {
  color: var(--warning);
}
.problem-row .loc {
  font-size: 0.7rem;
  color: var(--text-faint);
}
.submit-bar {
  display: flex;
  align-items: flex-start;
  gap: var(--space-4);
}
.submit-hint {
  font-size: 0.78rem;
  margin: 0 0 6px;
}
.result-panel {
  border: 1px solid var(--success-soft);
  background: var(--success-soft);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
}
.result-head {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 0.72rem;
}
.result-msg {
  margin: 8px 0 0;
  font-size: 0.86rem;
  line-height: 2;
  color: var(--text-dim);
  white-space: pre-line;
}

/* ---- editor stack reads as one instrument, not three stacked cards:
   toolbar + editor + problems fuse into a single bordered unit, then a normal
   gap before the submit bar. ---- */
.arena-editor .editor-toolbar {
  border-end-start-radius: 0;
  border-end-end-radius: 0;
}
.arena-editor .editor-host {
  margin-top: calc(-1 * var(--space-3));
  border-inline: 1px solid var(--border);
}
.arena-editor .problems-panel {
  margin-top: calc(-1 * var(--space-3));
  border-start-start-radius: 0;
  border-start-end-radius: 0;
}

/* ---- motion layer (MOTION_INTENSITY 6): entrance choreography + tactile
   press. Everything here is decorative and collapses under reduced-motion. ---- */
@media (prefers-reduced-motion: no-preference) {
  .hero > .hero-copy {
    animation: hero-rise 0.5s var(--ease) both;
  }
  .hero > .auth {
    animation: hero-rise 0.5s var(--ease) 0.08s both;
  }
  .state-panel {
    animation: hero-rise 0.4s var(--ease) both;
  }
}
@keyframes hero-rise {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.hero :deep(.ant-btn):active,
.arena :deep(.ant-btn):active {
  transform: translateY(1px);
}
</style>
