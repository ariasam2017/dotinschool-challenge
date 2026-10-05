<script setup lang="ts">
// Game مدرسه داتین — a Quiz-of-Kings style individual quiz, built mobile-first.
//
// The server owns every rule: it deals the questions, times each answer from
// the moment it served the question, and only reveals the right option after
// the player has answered. This screen just renders GameState and keeps a
// local countdown in sync with the server's remainingMs.
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { ApiError } from '@/api/client'
import {
  gameInfo,
  gameRegister,
  gameState,
  gameStart,
  gameAnswer,
  gameNext,
  gameReplay,
  type GameState,
  type GameInfo,
} from '@/api/game'
import { toFa, toEn } from '@/lib/fa'
import AppIcon from '@/components/AppIcon.vue'
import dotinLogo from '@/assets/brand/dotin-school-logo-w.svg'

const TOKEN_KEY = 'dotinschool_game_token_v1'
const REVEAL_MS = 1800 // how long the green/red reveal stays before moving on
const LETTERS = ['A', 'B', 'C', 'D']

const tokenStore = {
  get: () => {
    try {
      return localStorage.getItem(TOKEN_KEY)
    } catch {
      return null
    }
  },
  set: (t: string) => {
    try {
      localStorage.setItem(TOKEN_KEY, t)
    } catch {
      /* private mode: the session just won't survive a reload */
    }
  },
  clear: () => {
    try {
      localStorage.removeItem(TOKEN_KEY)
    } catch {
      /* ignore */
    }
  },
}

type Screen = 'boot' | 'register' | 'welcome' | 'game'
const screen = ref<Screen>('boot')
const token = ref<string | null>(null)
const st = ref<GameState | null>(null)
const info = ref<GameInfo | null>(null)

/* ---------------- registration ---------------- */
const form = ref({ firstName: '', lastName: '', phone: '' })
const formError = ref('')
const registering = ref(false)
const welcomeNew = ref(true)

const phoneDigits = computed(() => toEn(form.value.phone).replace(/\D/g, ''))
const phoneValid = computed(() => /^(0?9\d{9}|989\d{9})$/.test(phoneDigits.value))
const canRegister = computed(
  () => form.value.firstName.trim() && form.value.lastName.trim() && phoneValid.value,
)

async function register() {
  formError.value = ''
  if (!form.value.firstName.trim() || !form.value.lastName.trim()) {
    formError.value = 'نام و نام خانوادگی را کامل وارد کن.'
    return
  }
  if (!phoneValid.value) {
    formError.value = 'شماره موبایل معتبر نیست — مثلاً ۰۹۱۲۱۲۳۴۵۶۷'
    return
  }
  registering.value = true
  try {
    const r = await gameRegister(form.value.firstName.trim(), form.value.lastName.trim(), phoneDigits.value)
    token.value = r.token
    tokenStore.set(r.token)
    st.value = r.state
    welcomeNew.value = r.created
    screen.value = 'welcome'
    welcomeTimer = window.setTimeout(enterGame, 3200)
  } catch (e) {
    formError.value = e instanceof Error ? e.message : 'ثبت‌نام انجام نشد، دوباره تلاش کن.'
  } finally {
    registering.value = false
  }
}

let welcomeTimer: number | undefined
function enterGame() {
  clearTimeout(welcomeTimer)
  if (screen.value !== 'welcome') return
  screen.value = 'game'
  // a returning player may be mid-question: refetch so the countdown is not
  // stale by however long the welcome card was on screen
  if (st.value?.phase === 'question') void resync()
  else applyState(st.value!)
}

/* ---------------- the game loop ---------------- */
const busy = ref(false)
const picked = ref<number | null>(null)
const leftMs = ref(0)
let deadline = 0
let tickTimer: number | undefined
let advanceTimer: number | undefined
let infoPoller: number | undefined

const phase = computed(() => st.value?.phase ?? 'ready')
const q = computed(() => st.value?.question ?? null)
const reveal = computed(() => st.value?.reveal ?? null)
/** "finished" shows the result page only after the last reveal has played. */
const showResult = ref(false)

const limitMs = computed(() => q.value?.limitMs || (st.value?.secondsPerQuestion ?? 20) * 1000)
const timePct = computed(() => Math.max(0, Math.min(100, (leftMs.value / limitMs.value) * 100)))
const timeTone = computed(() => (timePct.value > 50 ? 'ok' : timePct.value > 25 ? 'warn' : 'danger'))
const secsLeft = computed(() => Math.ceil(leftMs.value / 1000))

const fullName = computed(() => (st.value ? `${st.value.player.firstName} ${st.value.player.lastName}` : ''))

function stopTick() {
  if (tickTimer !== undefined) {
    clearInterval(tickTimer)
    tickTimer = undefined
  }
}

function startTick(remainingMs: number) {
  stopTick()
  deadline = performance.now() + remainingMs
  leftMs.value = remainingMs
  tickTimer = window.setInterval(() => {
    leftMs.value = Math.max(0, deadline - performance.now())
    if (leftMs.value <= 0) {
      stopTick()
      if (picked.value === null && phase.value === 'question') void choose(-1)
    }
  }, 100)
}

/** Render a fresh server state and schedule whatever comes next. */
function applyState(s: GameState) {
  st.value = s
  clearTimeout(advanceTimer)

  if (s.phase === 'ready') {
    showResult.value = false
    stopTick()
    ensureInfoPolling(!s.open)
    return
  }
  ensureInfoPolling(false)

  if (s.phase === 'question') {
    picked.value = null
    showResult.value = false
    startTick(s.question?.remainingMs ?? 0)
    return
  }

  stopTick()
  leftMs.value = 0
  if (s.reveal) picked.value = s.reveal.choice

  if (s.phase === 'reveal') {
    advanceTimer = window.setTimeout(next, REVEAL_MS)
  } else if (s.phase === 'finished') {
    if (!showResult.value) advanceTimer = window.setTimeout(() => (showResult.value = true), REVEAL_MS)
  }
}

async function call(fn: () => Promise<GameState>): Promise<boolean> {
  try {
    applyState(await fn())
    return true
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      signOut(true)
      return false
    }
    message.error(e instanceof Error ? e.message : 'خطا در ارتباط با سرور')
    return false
  }
}

async function start() {
  if (!token.value || busy.value) return
  busy.value = true
  await call(() => gameStart(token.value!))
  busy.value = false
}

async function choose(i: number) {
  if (!token.value || !q.value || phase.value !== 'question' || picked.value !== null) return
  picked.value = i
  stopTick()
  const index = q.value.index
  const ok = await call(() => gameAnswer(token.value!, index, i))
  if (!ok) {
    // let them retry: resync from the server (the timer is authoritative there)
    window.setTimeout(resync, 800)
    return
  }
  if (reveal.value && !reveal.value.isCorrect) {
    try {
      navigator.vibrate?.(120)
    } catch {
      /* unsupported */
    }
  }
}

/** Test mode only: wipe this run and go back to the lobby. */
async function replay() {
  if (!token.value || busy.value) return
  busy.value = true
  const ok = await call(() => gameReplay(token.value!))
  busy.value = false
  if (ok) message.success('نتیجه‌ی قبلی پاک شد — دوباره بازی کن!')
}

async function next() {
  if (!token.value) return
  await call(() => gameNext(token.value!))
}

async function resync() {
  if (!token.value) return
  await call(() => gameState(token.value!))
}

function ensureInfoPolling(on: boolean) {
  if (on && infoPoller === undefined) {
    infoPoller = window.setInterval(async () => {
      try {
        info.value = await gameInfo()
        if (info.value.open && st.value && !st.value.open) await resync()
      } catch {
        /* transient */
      }
    }, 4000)
  } else if (!on && infoPoller !== undefined) {
    clearInterval(infoPoller)
    infoPoller = undefined
  }
}

function optionClass(i: number): Record<string, boolean> {
  const r = reveal.value
  if (!r || (phase.value !== 'reveal' && phase.value !== 'finished')) {
    return { picked: picked.value === i }
  }
  return {
    correct: i === r.correct,
    wrong: i === r.choice && !r.isCorrect,
    dim: i !== r.correct && i !== r.choice,
  }
}

const feedback = computed(() => {
  const r = reveal.value
  if (!r || phase.value === 'question') return null
  if (r.isCorrect) return { tone: 'ok', icon: 'check', text: 'آفرین! جوابت درست بود' }
  if (r.timedOut) return { tone: 'bad', icon: 'clock', text: 'وقت تمام شد!' }
  return { tone: 'bad', icon: 'x', text: 'جوابت اشتباه بود' }
})

function signOut(expired = false) {
  stopTick()
  clearTimeout(advanceTimer)
  ensureInfoPolling(false)
  tokenStore.clear()
  token.value = null
  st.value = null
  screen.value = 'register'
  if (expired) message.warning('نشست بازی معتبر نیست — دوباره ثبت‌نام کن.')
}

function confirmExit() {
  Modal.confirm({
    title: 'خروج از بازی',
    content:
      'با همان نام و شماره موبایل می‌توانی دوباره وارد شوی و نتیجه‌ات محفوظ می‌ماند.',
    okText: 'خروج',
    cancelText: 'ماندن',
    onOk: () => signOut(),
  })
}

function onVisible() {
  if (document.visibilityState === 'visible' && screen.value === 'game' && phase.value === 'question' && picked.value === null) {
    void resync()
  }
}

onMounted(async () => {
  document.addEventListener('visibilitychange', onVisible)
  gameInfo()
    .then((i) => (info.value = i))
    .catch(() => {})
  const t = tokenStore.get()
  if (!t) {
    screen.value = 'register'
    return
  }
  token.value = t
  try {
    const s = await gameState(t)
    screen.value = 'game'
    if (s.phase === 'finished') showResult.value = true
    applyState(s)
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      tokenStore.clear()
      token.value = null
    } else {
      message.error(e instanceof Error ? e.message : 'خطا در ارتباط با سرور')
    }
    screen.value = 'register'
  }
})

onBeforeUnmount(() => {
  stopTick()
  clearTimeout(advanceTimer)
  clearTimeout(welcomeTimer)
  ensureInfoPolling(false)
  document.removeEventListener('visibilitychange', onVisible)
})

const qCount = computed(() => st.value?.questionsPerGame ?? info.value?.questionsPerGame ?? 3)
const qSecs = computed(() => st.value?.secondsPerQuestion ?? info.value?.secondsPerQuestion ?? 20)
const gameOpen = computed(() => (info.value?.open ?? true) && (st.value?.open ?? true))
</script>

<template>
  <div class="g-root">
    <div class="g-bg" aria-hidden="true">
      <span class="g-grid" />
      <span class="glow gl1" /><span class="glow gl2" /><span class="glow gl3" />
      <span class="g-scan" />
    </div>

    <header class="g-top">
      <div class="g-logo">
        <span class="g-logo-mark"><AppIcon name="cpu" :size="20" /></span>
        <span class="g-logo-text"><b>Game</b> مدرسه داتین</span>
        <span v-if="st?.testMode" class="g-test-badge">حالت تست</span>
      </div>
      <button
        v-if="screen === 'game' && st && (phase === 'ready' || showResult)"
        type="button"
        class="g-exit"
        aria-label="خروج"
        @click="confirmExit"
      >
        <AppIcon name="log-out" :size="18" />
      </button>
    </header>

    <main class="g-main">
      <Transition name="g-swap" mode="out-in">
        <!-- boot -->
        <div v-if="screen === 'boot'" key="boot" class="g-center">
          <span class="g-spinner" aria-label="در حال بارگذاری" />
        </div>

        <!-- registration -->
        <section v-else-if="screen === 'register'" key="register" class="g-panel">
          <div class="g-hero">
            <img class="g-hero-logo" :src="dotinLogo" alt="مدرسه داتین" width="108" height="92" />
            <p class="g-kicker">&lt;tech-quiz /&gt;</p>
            <h1>به <b>Game</b> مدرسه داتین خوش آمدی!</h1>
            <p>
              {{ toFa(qCount) }} سوال چهارگزینه‌ای تکنولوژی، هر سوال {{ toFa(qSecs) }} ثانیه.
              درست‌تر و سریع‌تر جواب بده تا نفر اول شوی.
            </p>
          </div>

          <form class="g-card g-form" novalidate @submit.prevent="register">
            <label class="g-field">
              <span>نام</span>
              <input
                v-model="form.firstName"
                type="text"
                autocomplete="given-name"
                maxlength="60"
                placeholder="مثلاً علی"
                enterkeyhint="next"
              />
            </label>
            <label class="g-field">
              <span>نام خانوادگی</span>
              <input
                v-model="form.lastName"
                type="text"
                autocomplete="family-name"
                maxlength="60"
                placeholder="مثلاً رضایی"
                enterkeyhint="next"
              />
            </label>
            <label class="g-field">
              <span>شماره موبایل</span>
              <input
                v-model="form.phone"
                class="ltr"
                type="tel"
                inputmode="numeric"
                autocomplete="tel-national"
                maxlength="14"
                placeholder="09121234567"
                enterkeyhint="go"
              />
            </label>
            <p v-if="formError" class="g-error" role="alert">
              <AppIcon name="alert-circle" :size="16" /> {{ formError }}
            </p>
            <button type="submit" class="g-btn g-btn-primary" :disabled="registering || !canRegister">
              <span v-if="registering" class="g-spinner sm" />
              <template v-else>ثبت‌نام و ورود به بازی</template>
            </button>
          </form>
        </section>

        <!-- registration success -->
        <section v-else-if="screen === 'welcome'" key="welcome" class="g-center">
          <div class="g-card g-welcome">
            <span class="g-ok-badge"><AppIcon name="check" :size="40" /></span>
            <h2 v-if="welcomeNew">ثبت‌نام با موفقیت انجام شد!</h2>
            <h2 v-else>خوش برگشتی!</h2>
            <p>
              <b>{{ fullName }}</b> عزیز،
              <template v-if="welcomeNew">به بازی خوش آمدی. آماده‌ای؟</template>
              <template v-else>اطلاعاتت پیدا شد و وارد بازی‌ات می‌شوی.</template>
            </p>
            <button type="button" class="g-btn g-btn-primary" @click="enterGame">ورود به بازی</button>
          </div>
        </section>

        <!-- in game -->
        <section v-else-if="st" key="game" class="g-panel">
          <!-- lobby -->
          <div v-if="phase === 'ready'" class="g-lobby">
            <div class="g-card g-player">
              <span class="g-avatar">{{ st.player.firstName.charAt(0) }}{{ st.player.lastName.charAt(0) }}</span>
              <div>
                <div class="g-player-name">{{ fullName }}</div>
                <div class="g-player-sub"><i class="g-led" /> آماده‌ی مسابقه</div>
              </div>
            </div>

            <div class="g-card g-rules">
              <h2><AppIcon name="help-circle" :size="20" /> قوانین بازی</h2>
              <ul>
                <li><AppIcon name="list" :size="18" /><span><b>{{ toFa(qCount) }}</b> سوال چهارگزینه‌ای از حوزه‌ی تکنولوژی، به‌صورت تصادفی</span></li>
                <li><AppIcon name="clock" :size="18" /><span>برای هر سوال <b>{{ toFa(qSecs) }}</b> ثانیه وقت داری</span></li>
                <li><AppIcon name="zap" :size="18" /><span>رتبه با تعداد جواب درست و سرعت پاسخ‌گویی تعیین می‌شود</span></li>
                <li><AppIcon name="target" :size="18" /><span>فقط یک بار می‌توانی بازی کنی — با دقت انتخاب کن!</span></li>
              </ul>
            </div>

            <p v-if="!gameOpen" class="g-closed">
              <AppIcon name="clock" :size="18" /> بازی هنوز توسط مدیر باز نشده؛ به‌محض باز شدن، دکمه فعال می‌شود.
            </p>
            <button type="button" class="g-btn g-btn-start" :disabled="busy || !gameOpen" @click="start">
              <span v-if="busy" class="g-spinner sm" />
              <template v-else><AppIcon name="play" :size="22" /> شروع بازی</template>
            </button>
          </div>

          <!-- question / reveal (and the last reveal before the result page) -->
          <div v-else-if="q && !showResult" class="g-play">
            <div class="g-hud">
              <span class="g-qnum">سوال {{ toFa(q.index + 1) }} از {{ toFa(st.total) }}</span>
              <div class="g-dots" :class="{ many: st.progress.length > 6 }" aria-hidden="true">
                <span
                  v-for="(p, i) in st.progress"
                  :key="i"
                  class="g-dot"
                  :class="[p, { current: i === q.index && p === 'pending' }]"
                >
                  <template v-if="st.progress.length <= 6">
                    <AppIcon v-if="p === 'correct'" name="check" :size="12" />
                    <AppIcon v-else-if="p === 'wrong'" name="x" :size="12" />
                  </template>
                </span>
              </div>
            </div>

            <div class="g-timer" :class="timeTone" role="timer" :aria-label="`${secsLeft} ثانیه باقی مانده`">
              <div class="g-timer-track"><div class="g-timer-fill" :style="{ width: timePct + '%' }" /></div>
              <span class="g-timer-num"><AppIcon name="clock" :size="15" /> {{ toFa(secsLeft) }}</span>
            </div>

            <Transition name="g-pop" mode="out-in">
              <div :key="q.index" class="g-qcard">
                <span class="g-cat"><AppIcon name="cpu" :size="13" /> {{ q.category }}</span>
                <p class="g-qtext">{{ q.text }}</p>
              </div>
            </Transition>

            <div class="g-options">
              <button
                v-for="(opt, i) in q.options"
                :key="q.index + '-' + i"
                type="button"
                class="g-opt"
                :class="optionClass(i)"
                :disabled="phase !== 'question' || picked !== null"
                @click="choose(i)"
              >
                <span class="g-opt-letter">{{ LETTERS[i] }}</span>
                <span class="g-opt-text">{{ opt }}</span>
                <span class="g-opt-mark">
                  <AppIcon v-if="optionClass(i).correct" name="check" :size="20" />
                  <AppIcon v-else-if="optionClass(i).wrong" name="x" :size="20" />
                </span>
              </button>
            </div>

            <Transition name="g-pop">
              <p v-if="feedback" class="g-feedback" :class="feedback.tone">
                <AppIcon :name="feedback.icon" :size="20" /> {{ feedback.text }}
              </p>
            </Transition>
          </div>

          <!-- result: only how many were right; the answers stay on screen -->
          <div v-else-if="phase === 'finished' && st.result" class="g-result">
            <div class="g-card g-result-card">
              <span class="g-trophy"><AppIcon name="trophy" :size="44" /></span>
              <h2>بازی تمام شد، {{ st.player.firstName }}!</h2>
              <div class="g-score-ring" :style="{ '--pct': (st.result.total ? (st.result.correct / st.result.total) * 100 : 0) + '%' }">
                <span class="g-score-num">{{ toFa(st.result.correct) }}</span>
                <span class="g-score-of">از {{ toFa(st.result.total) }}</span>
              </div>
              <p class="g-score-lbl">جواب درست</p>
              <p class="g-finish-msg">{{ st.result.finishMessage }}</p>
            </div>

            <button v-if="st.testMode" type="button" class="g-btn g-btn-replay" :disabled="busy" @click="replay">
              <span v-if="busy" class="g-spinner sm" />
              <template v-else><AppIcon name="rotate-ccw" :size="20" /> بازی دوباره (حالت تست)</template>
            </button>

            <h3 class="g-review-title"><AppIcon name="list" :size="16" /> پاسخ‌های تو</h3>
            <div v-for="(r, qi) in st.review || []" :key="qi" class="g-card g-review">
              <div class="g-review-hd">
                <span class="g-review-n">سوال {{ toFa(qi + 1) }}</span>
                <span class="g-review-tag" :class="r.isCorrect ? 'ok' : 'bad'">
                  <template v-if="r.isCorrect">درست</template>
                  <template v-else-if="r.choice === -1">وقت تمام شد</template>
                  <template v-else>اشتباه</template>
                </span>
              </div>
              <p class="g-review-q">{{ r.text }}</p>
              <ul class="g-review-opts">
                <li
                  v-for="(o, oi) in r.options"
                  :key="oi"
                  :class="{ correct: oi === r.correct, wrong: oi === r.choice && !r.isCorrect }"
                >
                  <AppIcon v-if="oi === r.correct" name="check" :size="15" />
                  <AppIcon v-else-if="oi === r.choice" name="x" :size="15" />
                  <span v-else class="g-bullet" />
                  <span>{{ o }}</span>
                </li>
              </ul>
            </div>
          </div>
        </section>
      </Transition>
    </main>
  </div>
</template>

<style scoped>
/* "Tech" palette — fixed in both themes: deep-space navy, electric cyan, a
   violet second accent, and neon green/red for right/wrong. */
.g-root {
  --g-bg0: #030712;
  --g-bg1: #07122b;
  --g-glass: rgba(13, 25, 50, 0.72);
  --g-glass-2: rgba(22, 40, 74, 0.7);
  --g-line: rgba(56, 189, 248, 0.22);
  --g-line-strong: rgba(56, 189, 248, 0.45);
  --g-text: #e6f1ff;
  --g-dim: #9fb3d1;
  --g-faint: #6b7fa3;
  --g-cyan: #22d3ee;
  --g-blue: #3b82f6;
  --g-violet: #8b5cf6;
  --g-ok: #10b981;
  --g-bad: #f43f5e;
  --g-warn: #f59e0b;
  --g-mono: 'JetBrains Mono', ui-monospace, monospace;

  position: relative;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  color: var(--g-text);
  background: radial-gradient(120% 80% at 50% 0%, var(--g-bg1) 0%, var(--g-bg0) 70%);
  overscroll-behavior: none;
  -webkit-tap-highlight-color: transparent;
  padding-bottom: env(safe-area-inset-bottom);
}
.g-bg {
  position: fixed;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}
.g-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(56, 189, 248, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(56, 189, 248, 0.07) 1px, transparent 1px);
  background-size: 34px 34px;
  mask-image: radial-gradient(ellipse 90% 70% at 50% 30%, #000 30%, transparent 80%);
  -webkit-mask-image: radial-gradient(ellipse 90% 70% at 50% 30%, #000 30%, transparent 80%);
}
.glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(70px);
}
.gl1 {
  width: 300px;
  height: 300px;
  background: rgba(34, 211, 238, 0.28);
  top: -110px;
  inset-inline-end: -90px;
}
.gl2 {
  width: 280px;
  height: 280px;
  background: rgba(139, 92, 246, 0.25);
  bottom: 8%;
  inset-inline-start: -130px;
}
.gl3 {
  width: 220px;
  height: 220px;
  background: rgba(59, 130, 246, 0.2);
  bottom: -80px;
  inset-inline-end: 5%;
}
.g-scan {
  position: absolute;
  inset-inline: 0;
  top: 0;
  height: 120px;
  background: linear-gradient(180deg, transparent, rgba(34, 211, 238, 0.06), transparent);
  animation: g-scan 7s linear infinite;
}

/* ---------- header ---------- */
.g-top {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: calc(12px + env(safe-area-inset-top)) 16px 8px;
  max-width: 560px;
  width: 100%;
  margin: 0 auto;
}
.g-logo {
  display: flex;
  align-items: center;
  gap: 10px;
}
.g-logo-mark {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 11px;
  background: linear-gradient(135deg, var(--g-cyan), var(--g-blue));
  color: #04111f;
  box-shadow: 0 0 18px rgba(34, 211, 238, 0.45);
}
.g-logo-text {
  font-size: 1.05rem;
  font-weight: 700;
  white-space: nowrap;
}
.g-logo-text b {
  font-family: var(--g-mono);
  font-weight: 700;
  color: var(--g-cyan);
  text-shadow: 0 0 12px rgba(34, 211, 238, 0.6);
}
.g-exit {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  border: 1px solid var(--g-line);
  background: var(--g-glass);
  color: var(--g-text);
}

/* ---------- layout ---------- */
.g-main {
  position: relative;
  z-index: 1;
  flex: 1;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 560px;
  margin: 0 auto;
  padding: 8px 16px 24px;
}
.g-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}
.g-center {
  flex: 1;
  display: grid;
  place-items: center;
}
.g-card {
  position: relative;
  background: var(--g-glass);
  color: var(--g-text);
  border: 1px solid var(--g-line);
  border-radius: 20px;
  box-shadow: 0 12px 34px rgba(0, 0, 0, 0.45), inset 0 1px 0 rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

/* ---------- registration ---------- */
.g-hero {
  text-align: center;
  padding: 10px 4px 0;
}
.g-hero-logo {
  display: block;
  width: 108px;
  height: auto;
  margin: 4px auto 14px;
  filter: drop-shadow(0 0 18px rgba(34, 211, 238, 0.45));
  animation: g-float 3.2s ease-in-out infinite;
}
.g-kicker {
  font-family: var(--g-mono);
  font-size: 0.8rem;
  color: var(--g-cyan);
  direction: ltr;
  margin: 0 0 6px;
  opacity: 0.85;
}
.g-hero h1 {
  color: var(--g-text);
  font-size: 1.4rem;
  font-weight: 800;
  margin: 0 0 8px;
}
.g-hero h1 b {
  font-family: var(--g-mono);
  color: var(--g-cyan);
}
.g-hero p {
  color: var(--g-dim);
  font-size: 0.95rem;
  max-width: 400px;
  margin: 0 auto;
}
.g-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  max-width: 380px; /* a slim card, not the full column, on wide screens */
  margin: 0 auto;
  padding: 22px 20px 20px;
}
.g-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.g-field > span {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--g-dim);
}
.g-field input {
  width: 100%;
  height: 48px;
  padding: 0 16px;
  border-radius: 14px;
  border: 1px solid var(--g-line);
  background: rgba(3, 10, 25, 0.65);
  color: var(--g-text);
  font-family: inherit;
  font-size: 16px; /* >= 16px: stops iOS zooming into the field */
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.g-field input:focus {
  border-color: var(--g-cyan);
  box-shadow: 0 0 0 3px rgba(34, 211, 238, 0.2);
}
.g-field input.ltr {
  direction: ltr;
  text-align: right;
  font-family: var(--g-mono);
  letter-spacing: 0.05em;
}
.g-field input::placeholder {
  color: var(--g-faint);
}
/* keep autofilled fields on the dark glass */
.g-field input:-webkit-autofill {
  -webkit-text-fill-color: var(--g-text);
  -webkit-box-shadow: inset 0 0 0 1000px #071430 !important;
  box-shadow: inset 0 0 0 1000px #071430 !important;
}
.g-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(244, 63, 94, 0.14);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fda4af;
  font-size: 0.88rem;
  font-weight: 600;
}

/* ---------- buttons ---------- */
.g-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  width: 100%;
  min-height: 56px;
  border: 0;
  border-radius: 16px;
  font-size: 1.05rem;
  font-weight: 800;
  touch-action: manipulation;
  user-select: none;
  transition: transform 0.12s, filter 0.15s, opacity 0.15s, box-shadow 0.15s;
}
.g-btn:active:not(:disabled) {
  transform: scale(0.97);
}
.g-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.g-btn-primary {
  background: linear-gradient(135deg, var(--g-cyan), var(--g-blue) 55%, var(--g-violet));
  color: #fff;
  box-shadow: 0 0 22px rgba(34, 211, 238, 0.35);
}
.g-btn-start {
  min-height: 64px;
  font-size: 1.25rem;
  background: linear-gradient(135deg, var(--g-cyan), var(--g-blue));
  color: #fff;
  box-shadow: 0 0 0 1px rgba(34, 211, 238, 0.6), 0 0 34px rgba(34, 211, 238, 0.45);
  margin-bottom: 10px;
  animation: g-glow 2.4s ease-in-out infinite;
}
.g-btn-start:disabled {
  animation: none;
}

.g-btn-replay {
  background: rgba(245, 158, 11, 0.14);
  border: 1px dashed rgba(245, 158, 11, 0.6);
  color: #fcd34d;
}
.g-test-badge {
  padding: 2px 10px;
  border-radius: 999px;
  border: 1px dashed rgba(245, 158, 11, 0.6);
  color: #fcd34d;
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
}

/* ---------- welcome ---------- */
.g-welcome {
  width: 100%;
  text-align: center;
  padding: 30px 22px 22px;
  animation: g-rise 0.45s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.g-ok-badge {
  display: inline-grid;
  place-items: center;
  width: 84px;
  height: 84px;
  border-radius: 50%;
  background: var(--g-ok);
  color: #fff;
  margin-bottom: 16px;
  animation: g-pulse 1.6s ease-out infinite;
}
.g-welcome h2 {
  color: var(--g-text);
  font-size: 1.35rem;
  font-weight: 800;
  margin-bottom: 8px;
}
.g-welcome p {
  color: var(--g-dim);
  margin-bottom: 20px;
}
.g-welcome b {
  color: var(--g-text);
}

/* ---------- lobby ---------- */
.g-lobby {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}
.g-player {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
}
.g-avatar {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  flex: none;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--g-violet), var(--g-blue));
  color: #fff;
  font-weight: 800;
  font-size: 1.1rem;
  box-shadow: 0 0 16px rgba(139, 92, 246, 0.45);
}
.g-player-name {
  font-weight: 800;
  font-size: 1.08rem;
}
.g-player-sub {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.82rem;
  color: var(--g-dim);
}
.g-led {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--g-ok);
  box-shadow: 0 0 8px var(--g-ok);
}
.g-rules {
  padding: 18px;
}
.g-rules h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--g-text);
  font-size: 1.05rem;
  font-weight: 800;
  margin-bottom: 10px;
}
.g-rules ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.g-rules li {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 0.92rem;
  color: var(--g-dim);
}
.g-rules li b {
  color: var(--g-cyan);
}
.g-rules li :deep(svg) {
  flex: none;
  margin-top: 4px;
  color: var(--g-cyan);
}
.g-closed {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 12px 14px;
  border-radius: 14px;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.45);
  color: #fcd34d;
  font-size: 0.9rem;
}
.g-lobby .g-btn-start {
  margin-top: auto;
}

/* ---------- play ---------- */
.g-play {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.g-hud {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.g-qnum {
  font-weight: 800;
  font-size: 1rem;
  white-space: nowrap;
  flex: none;
}
/* many questions (e.g. 10+): the dots become one slim segmented bar */
.g-dots.many {
  flex: 1;
  flex-wrap: nowrap;
  gap: 3px;
}
.g-dots.many .g-dot {
  flex: 1;
  width: auto;
  height: 8px;
  border-radius: 4px;
  border-width: 1px;
}
.g-dots {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.g-dot {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 8px;
  background: var(--g-glass);
  border: 1px solid var(--g-line);
  color: #fff;
}
.g-dot.current {
  border-color: var(--g-cyan);
  box-shadow: 0 0 10px rgba(34, 211, 238, 0.6);
}
.g-dot.correct {
  background: var(--g-ok);
  border-color: var(--g-ok);
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.6);
}
.g-dot.wrong {
  background: var(--g-bad);
  border-color: var(--g-bad);
  box-shadow: 0 0 10px rgba(244, 63, 94, 0.6);
}

.g-timer {
  display: flex;
  align-items: center;
  gap: 10px;
}
.g-timer-track {
  flex: 1;
  height: 10px;
  border-radius: 999px;
  background: rgba(13, 25, 50, 0.9);
  border: 1px solid var(--g-line);
  overflow: hidden;
}
.g-timer-fill {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--g-blue), var(--g-cyan));
  box-shadow: 0 0 12px rgba(34, 211, 238, 0.7);
  transition: width 0.1s linear, background 0.3s;
}
.g-timer.warn .g-timer-fill {
  background: var(--g-warn);
  box-shadow: 0 0 12px rgba(245, 158, 11, 0.7);
}
.g-timer.danger .g-timer-fill {
  background: var(--g-bad);
  box-shadow: 0 0 12px rgba(244, 63, 94, 0.7);
}
.g-timer-num {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 56px;
  justify-content: center;
  padding: 4px 10px;
  border-radius: 10px;
  background: var(--g-glass);
  border: 1px solid var(--g-line);
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}
.g-timer.danger .g-timer-num {
  color: #fda4af;
  border-color: rgba(244, 63, 94, 0.5);
  animation: g-beat 0.6s ease-in-out infinite;
}

.g-qcard {
  position: relative;
  background: linear-gradient(160deg, rgba(22, 40, 74, 0.85), rgba(10, 20, 42, 0.85));
  border: 1px solid var(--g-line-strong);
  border-radius: 22px;
  padding: 32px 20px 26px;
  min-height: 150px;
  display: grid;
  place-items: center;
  text-align: center;
  box-shadow: 0 0 30px rgba(34, 211, 238, 0.12), 0 14px 34px rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}
/* corner brackets — a HUD frame */
.g-qcard::before,
.g-qcard::after {
  content: '';
  position: absolute;
  width: 18px;
  height: 18px;
  border: 2px solid var(--g-cyan);
  opacity: 0.8;
}
.g-qcard::before {
  top: 10px;
  left: 10px;
  border-right: 0;
  border-bottom: 0;
  border-radius: 6px 0 0 0;
}
.g-qcard::after {
  bottom: 10px;
  right: 10px;
  border-left: 0;
  border-top: 0;
  border-radius: 0 0 6px 0;
}
.g-cat {
  position: absolute;
  top: -13px;
  left: 50%;
  transform: translateX(-50%);
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 14px;
  border-radius: 999px;
  background: #06243a;
  border: 1px solid var(--g-cyan);
  color: var(--g-cyan);
  font-size: 0.8rem;
  font-weight: 800;
  white-space: nowrap;
  box-shadow: 0 0 14px rgba(34, 211, 238, 0.35);
}
.g-qtext {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 800;
  line-height: 1.8;
  overflow-wrap: anywhere;
}

.g-options {
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}
.g-opt {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 58px;
  padding: 10px 14px;
  border: 1px solid var(--g-line);
  border-radius: 16px;
  background: var(--g-glass);
  color: var(--g-text);
  font-size: 1rem;
  font-weight: 700;
  text-align: start;
  touch-action: manipulation;
  user-select: none;
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  transition: transform 0.12s, box-shadow 0.15s, background 0.2s, border-color 0.2s, opacity 0.2s;
}
.g-opt:not(:disabled):active {
  transform: scale(0.98);
  border-color: var(--g-cyan);
}
@media (hover: hover) {
  .g-opt:not(:disabled):hover {
    border-color: var(--g-line-strong);
    background: var(--g-glass-2);
  }
}
.g-opt:disabled {
  cursor: default;
}
.g-opt-letter {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  flex: none;
  border-radius: 10px;
  background: rgba(34, 211, 238, 0.1);
  border: 1px solid rgba(34, 211, 238, 0.35);
  color: var(--g-cyan);
  font-family: var(--g-mono);
  font-size: 0.9rem;
  font-weight: 700;
}
.g-opt-text {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.g-opt-mark {
  display: grid;
  place-items: center;
  width: 24px;
  flex: none;
}
.g-opt.picked {
  border-color: var(--g-cyan);
  box-shadow: 0 0 18px rgba(34, 211, 238, 0.45);
  animation: g-wait 0.9s ease-in-out infinite;
}
.g-opt.correct {
  background: rgba(16, 185, 129, 0.9);
  border-color: #34d399;
  color: #fff;
  box-shadow: 0 0 22px rgba(16, 185, 129, 0.6);
  animation: g-pop-in 0.35s ease-out;
}
.g-opt.wrong {
  background: rgba(244, 63, 94, 0.9);
  border-color: #fb7185;
  color: #fff;
  box-shadow: 0 0 22px rgba(244, 63, 94, 0.55);
  animation: g-shake 0.4s ease-in-out;
}
.g-opt.correct .g-opt-letter,
.g-opt.wrong .g-opt-letter {
  background: rgba(255, 255, 255, 0.2);
  border-color: transparent;
  color: #fff;
}
.g-opt.dim {
  opacity: 0.45;
}

.g-feedback {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 2px 0 0;
  padding: 12px;
  border-radius: 14px;
  font-weight: 800;
  font-size: 1.05rem;
  border: 1px solid;
}
.g-feedback.ok {
  background: rgba(16, 185, 129, 0.14);
  border-color: rgba(16, 185, 129, 0.5);
  color: #6ee7b7;
}
.g-feedback.bad {
  background: rgba(244, 63, 94, 0.14);
  border-color: rgba(244, 63, 94, 0.5);
  color: #fda4af;
}

/* ---------- result ---------- */
.g-result {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.g-result-card {
  text-align: center;
  padding: 26px 18px 22px;
  animation: g-rise 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.g-trophy {
  display: inline-grid;
  place-items: center;
  width: 80px;
  height: 80px;
  border-radius: 24px;
  background: linear-gradient(135deg, var(--g-cyan), var(--g-violet));
  color: #04111f;
  box-shadow: 0 0 34px rgba(34, 211, 238, 0.45);
  margin-bottom: 12px;
  animation: g-float 3.2s ease-in-out infinite;
}
.g-result-card h2 {
  color: var(--g-text);
  font-size: 1.3rem;
  font-weight: 900;
  margin-bottom: 16px;
}
.g-score-ring {
  --pct: 0%;
  position: relative;
  width: 132px;
  height: 132px;
  margin: 0 auto;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(closest-side, #081733 80%, transparent 81%),
    conic-gradient(var(--g-cyan) var(--pct), rgba(56, 189, 248, 0.15) 0);
  box-shadow: 0 0 30px rgba(34, 211, 238, 0.3);
}
.g-score-num {
  font-size: 2.6rem;
  font-weight: 900;
  line-height: 1.1;
  color: var(--g-cyan);
  text-shadow: 0 0 14px rgba(34, 211, 238, 0.6);
}
.g-score-of {
  font-size: 0.9rem;
  color: var(--g-dim);
  font-weight: 700;
}
.g-score-lbl {
  margin: 10px 0 12px;
  font-weight: 800;
  color: var(--g-text);
}
.g-finish-msg {
  margin: 0;
  padding: 12px;
  border-radius: 14px;
  background: rgba(34, 211, 238, 0.07);
  border: 1px dashed rgba(34, 211, 238, 0.3);
  color: var(--g-dim);
  font-size: 0.93rem;
  white-space: pre-line;
}
.g-review-title {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--g-text);
  font-size: 1rem;
  font-weight: 800;
  margin: 8px 4px 0;
}
.g-review {
  padding: 14px 16px;
}
.g-review-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}
.g-review-n {
  font-size: 0.8rem;
  font-weight: 800;
  color: var(--g-dim);
}
.g-review-tag {
  padding: 2px 12px;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 800;
  border: 1px solid;
}
.g-review-tag.ok {
  background: rgba(16, 185, 129, 0.14);
  border-color: rgba(16, 185, 129, 0.5);
  color: #6ee7b7;
}
.g-review-tag.bad {
  background: rgba(244, 63, 94, 0.14);
  border-color: rgba(244, 63, 94, 0.5);
  color: #fda4af;
}
.g-review-q {
  font-weight: 800;
  margin-bottom: 8px;
  overflow-wrap: anywhere;
}
.g-review-opts {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.g-review-opts li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(3, 10, 25, 0.5);
  border: 1px solid rgba(56, 189, 248, 0.12);
  color: var(--g-dim);
  font-size: 0.9rem;
  overflow-wrap: anywhere;
}
.g-review-opts li.correct {
  background: rgba(16, 185, 129, 0.85);
  border-color: #34d399;
  color: #fff;
  font-weight: 700;
}
.g-review-opts li.wrong {
  background: rgba(244, 63, 94, 0.85);
  border-color: #fb7185;
  color: #fff;
  font-weight: 700;
}
.g-review-opts li :deep(svg) {
  flex: none;
}
.g-bullet {
  width: 15px;
  height: 15px;
  flex: none;
  border-radius: 4px;
  border: 1px solid rgba(56, 189, 248, 0.35);
}

/* ---------- spinner ---------- */
.g-spinner {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  border: 3px solid rgba(34, 211, 238, 0.2);
  border-top-color: var(--g-cyan);
  animation: g-spin 0.8s linear infinite;
}
.g-spinner.sm {
  width: 24px;
  height: 24px;
  border-color: rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
}

/* ---------- larger screens: 2x2 answers ---------- */
@media (min-width: 600px) {
  .g-options {
    grid-template-columns: 1fr 1fr;
  }
  .g-qtext {
    font-size: 1.3rem;
  }
}
@media (max-width: 360px) {
  .g-logo-text {
    font-size: 0.95rem;
  }
  .g-qtext {
    font-size: 1.05rem;
  }
  .g-opt {
    min-height: 54px;
    font-size: 0.95rem;
  }
}
/* short landscape phones: keep everything above the fold */
@media (max-height: 560px) and (orientation: landscape) {
  .g-qcard {
    min-height: 100px;
    padding: 22px 16px 16px;
  }
  .g-options {
    grid-template-columns: 1fr 1fr;
  }
}

/* ---------- motion ---------- */
.g-swap-enter-active,
.g-swap-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease;
}
.g-swap-enter-from {
  opacity: 0;
  transform: translateY(12px);
}
.g-swap-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.g-pop-enter-active {
  transition: opacity 0.25s ease, transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.g-pop-leave-active {
  transition: opacity 0.15s ease;
}
.g-pop-enter-from {
  opacity: 0;
  transform: scale(0.94);
}
.g-pop-leave-to {
  opacity: 0;
}
@keyframes g-spin {
  to {
    transform: rotate(360deg);
  }
}
@keyframes g-scan {
  from {
    transform: translateY(-120px);
  }
  to {
    transform: translateY(100vh);
  }
}
@keyframes g-float {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-6px);
  }
}
@keyframes g-glow {
  0%,
  100% {
    box-shadow: 0 0 0 1px rgba(34, 211, 238, 0.6), 0 0 24px rgba(34, 211, 238, 0.35);
  }
  50% {
    box-shadow: 0 0 0 1px rgba(34, 211, 238, 0.9), 0 0 40px rgba(34, 211, 238, 0.6);
  }
}
@keyframes g-rise {
  from {
    opacity: 0;
    transform: translateY(16px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@keyframes g-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.5);
  }
  100% {
    box-shadow: 0 0 0 18px rgba(16, 185, 129, 0);
  }
}
@keyframes g-shake {
  0%,
  100% {
    transform: translateX(0);
  }
  20%,
  60% {
    transform: translateX(-7px);
  }
  40%,
  80% {
    transform: translateX(7px);
  }
}
@keyframes g-pop-in {
  0% {
    transform: scale(0.96);
  }
  60% {
    transform: scale(1.03);
  }
  100% {
    transform: scale(1);
  }
}
@keyframes g-wait {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.7;
  }
}
@keyframes g-beat {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.08);
  }
}
</style>
