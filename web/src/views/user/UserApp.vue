<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import { useParticipantStore } from '@/stores/participant'
import { useEventStore } from '@/stores/event'
import { ApiError } from '@/api/client'
import { installErrorQueueFlusher, submitClientError } from '@/lib/errorQueue'
import { estimateExecution, performanceScore, countErrors, STARTER_CODE } from '@/lib/estimate'
import { fmtJalaliDateTime } from '@/lib/fa'

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
const { challenge, state: eventState, remainMs, startsInMs, canCompete, loaded } = storeToRefs(eStore)

const booted = ref(false)
const arenaOpen = ref(false)
const code = ref(STARTER_CODE)
const submitState = ref<'idle' | 'submitting' | 'submitted' | 'error'>('idle')
const submitError = ref('')

const problems = ref<EditorProblem[]>([])
const cursor = ref({ line: 1, col: 1 })
const errCount = computed(() => problems.value.filter((p) => p.severity === 'error').length)
const warnCount = computed(() => problems.value.filter((p) => p.severity === 'warning').length)

const RULES = [
  'پس از شروع، تایمر رویداد از حرکت باز نمی‌ایستد.',
  'خطاهای نحوی کد شما به‌صورت زنده زیر ادیتور نمایش داده می‌شود.',
  'می‌توانید تا پایان زمان، کد را ویرایش و چند بار ارسال کنید.',
]

function resetCode() {
  code.value = STARTER_CODE
}

const LOCK_MESSAGES: Record<string, string> = {
  waiting: 'چالش هنوز طبق زمان‌بندی شروع نشده — نوشتن و ارسال کد ممکن نیست',
  paused: 'رویداد توسط مدیر متوقف شده — نوشتن و ارسال کد ممکن نیست',
  ended: 'زمان رویداد به پایان رسید — ارسال دیگر ممکن نیست',
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

onMounted(async () => {
  installErrorQueueFlusher()
  eStore.start()
  const resumed = await pStore.resume()
  if (resumed) {
    message.success(`خوش آمدید دوباره، ${resumed.firstName}`)
    if (resumed.status !== 'registered') {
      arenaOpen.value = true
      code.value = resumed.code || STARTER_CODE
      if (resumed.status === 'submitted') submitState.value = 'submitted'
    }
  }
  booted.value = true
})
onBeforeUnmount(() => eStore.stop())

function onAuthed(status: string) {
  if (status !== 'registered') {
    arenaOpen.value = true
    code.value = user.value?.code || STARTER_CODE
  }
}

async function startChallenge() {
  if (!user.value) return
  if (!canCompete.value) {
    message.error(LOCK_MESSAGES[lockReason.value] || 'شروع چالش ممکن نیست')
    return
  }
  arenaOpen.value = true
  code.value = user.value.code || STARTER_CODE
  await pStore.patchQuietly({ status: 'started' })
  window.scrollTo({ top: 0, behavior: 'smooth' })
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
    return 'ارسال کد انجام نشد — اتصال اینترنت خود را بررسی کنید و دوباره تلاش کنید.'
  if (err.code === 'INTERNAL_ERROR' || err.status >= 500)
    return 'ارسال کد در این لحظه ممکن نشد — چند لحظه صبر کنید و دوباره تلاش کنید.'
  return err.message || 'ارسال کد با خطا مواجه شد — دوباره تلاش کنید.'
}

function confirmExit() {
  const midChallenge = arenaOpen.value && user.value?.status !== 'submitted'
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
        <span class="brand-name">dotinschool</span>
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

    <main class="u-main">
      <!-- not signed in -->
      <section v-if="!user" class="hero">
        <div class="hero-copy">
          <h1>به چالش برنامه‌نویسی خوش آمدید</h1>
          <p>
            کد جاوای خود را در یک ادیتور حرفه‌ای بنویسید و تا پایان مهلت ارسال کنید.
            هویت شما بر اساس شماره موبایل احراز می‌شود.
          </p>
        </div>
        <AuthPanel @authed="onAuthed" />
      </section>

      <!-- signed in -->
      <template v-else>
        <div v-if="loaded && eventState !== 'running' && !arenaOpen" class="event-banner" :class="eventState">
          <AppIcon :name="eventState === 'ended' ? 'clock' : eventState === 'paused' ? 'pause' : 'calendar'" :size="16" />
          <span>
            {{ eventState === 'waiting' ? 'رویداد زمان‌بندی شده و هنوز شروع نشده است' : eventState === 'paused' ? 'رویداد توسط مدیر موقتاً متوقف شده است' : 'زمان رویداد به پایان رسیده است' }}
          </span>
        </div>

        <!-- scheduled: waiting for start -->
        <section v-if="eventState === 'waiting' && !arenaOpen" class="wait-card card card-pad">
          <AppIcon name="clock" :size="28" />
          <h2>چالش هنوز شروع نشده است</h2>
          <CountdownClock :ms="startsInMs" state="waiting" size="xl" label="زمان باقی‌مانده تا شروع خودکار" />
          <p v-if="eStore.config?.eventStartsAt" class="faint">
            شروع: <span class="num">{{ fmtJalaliDateTime(eStore.config.eventStartsAt) }}</span>
          </p>
        </section>

        <!-- challenge intro -->
        <section v-else-if="!arenaOpen && challenge" class="intro-grid">
          <div class="card card-pad">
            <ChallengeCard :challenge="challenge" />
          </div>
          <aside class="intro-side">
            <div class="card card-pad clock-card">
              <span class="section-title">زمان باقی‌مانده</span>
              <CountdownClock :ms="remainMs" :state="clockState" size="lg" />
              <a-button
                type="primary"
                size="large"
                block
                :disabled="!canCompete"
                @click="startChallenge"
              >
                <template #icon><AppIcon name="play" :size="16" /></template>
                شروع حل چالش
              </a-button>
              <p v-if="!canCompete" class="faint lock-note">{{ LOCK_MESSAGES[lockReason] }}</p>
            </div>

            <div class="card card-pad rules-card">
              <span class="section-title">پیش از شروع</span>
              <ul class="rules-list">
                <li v-for="(r, i) in RULES" :key="i">
                  <AppIcon name="check-circle" :size="15" />
                  <span>{{ r }}</span>
                </li>
              </ul>
            </div>
          </aside>
        </section>

        <!-- arena -->
        <section v-else-if="arenaOpen && challenge" class="arena">
          <aside class="arena-brief card card-pad">
            <ChallengeCard :challenge="challenge" compact />
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
                <div v-if="!problems.length" class="problems-empty">مشکلی یافت نشد ✓</div>
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
      </template>
    </main>
  </div>
</template>

<style scoped>
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
  color: var(--primary);
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
.u-header {
  overflow: hidden;
}
.hero {
  display: grid;
  gap: var(--space-6);
  align-items: center;
}
.hero > * {
  min-width: 0;
}
.hero-copy h1,
.hero-copy p {
  overflow-wrap: anywhere;
}
@media (min-width: 900px) {
  .hero {
    grid-template-columns: 1fr 1fr;
    gap: var(--space-8);
    padding-top: var(--space-6);
  }
}
.hero-copy h1 {
  font-size: clamp(1.6rem, 4vw, 2.4rem);
  line-height: 1.3;
}
.hero-copy p {
  font-size: 1rem;
  color: var(--text-dim);
  line-height: 2;
}
.event-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  border-radius: var(--radius);
  font-size: 0.85rem;
  font-weight: 500;
  margin-bottom: var(--space-4);
}
.event-banner.waiting {
  background: var(--info-soft);
  color: var(--info);
}
.event-banner.paused {
  background: var(--warning-soft);
  color: var(--warning);
}
.event-banner.ended {
  background: var(--danger-soft);
  color: var(--danger);
}
.wait-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-4);
  text-align: center;
  color: var(--text-dim);
}
.intro-grid {
  display: grid;
  gap: var(--space-5);
}
@media (min-width: 960px) {
  .intro-grid {
    grid-template-columns: 1fr 320px;
    align-items: start;
  }
}
.clock-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-4);
  position: sticky;
  top: calc(var(--header-h) + 16px);
}
.lock-note {
  font-size: 0.75rem;
  text-align: center;
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
.rules-card {
  margin-top: var(--space-3);
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
</style>
