<script setup lang="ts">
// لیگ قهرمانان برنامه‌نویسی — a Quiz-of-Kings style individual quiz, built mobile-first.
//
// The server owns every rule: it deals the questions, times each answer from
// the moment it served the question, and only reveals the right option after
// the player has answered. This screen just renders GameState and keeps a
// local countdown in sync with the server's remainingMs.
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
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

/* ---- after the result: the "follow us" screen ---- */
// The player sees their own result for socialDelaySec, then the logo + social
// links take over. "مشاهده‌ی نتیجه" brings the result back and stops the switch.
const showSocial = ref(false)
let socialDismissed = false
let socialTimer: number | undefined
watch(showResult, (on) => {
  clearTimeout(socialTimer)
  if (!on) {
    showSocial.value = false
    return
  }
  if (socialDismissed) return
  const delay = (st.value?.socialDelaySec || 8) * 1000
  socialTimer = window.setTimeout(() => (showSocial.value = true), delay)
})
function backToResult() {
  socialDismissed = true
  clearTimeout(socialTimer)
  showSocial.value = false
}

interface Social {
  key: string
  name: string
  url: string
  icon: string // inner SVG markup, 24x24 viewBox (static, trusted)
}
// Each network uses its standard mark (Bale's is the one bale.ai itself serves).
const SOCIALS: Social[] = [
  {
    key: 'instagram',
    name: 'اینستاگرام',
    url: 'https://instagram.com/dotinschool',
    icon: '<rect x="3" y="3" width="18" height="18" rx="5" fill="none" stroke="currentColor" stroke-width="2"/><circle cx="12" cy="12" r="4.2" fill="none" stroke="currentColor" stroke-width="2"/><circle cx="17.4" cy="6.6" r="1.3" fill="currentColor"/>',
  },
  {
    key: 'bale',
    name: 'بله',
    url: 'https://ble.ir/dotinschool',
    icon: '<path fill="currentColor" fill-rule="evenodd" d="M13.3847 2.09442H13.3666C13.2161 2.07033 13.0656 2.05828 12.9151 2.0402L12.7586 2.02815L12.4275 2.01008H11.5244L11.3137 2.02213L11.127 2.03418L10.9163 2.05828L10.7297 2.07635L10.525 2.10647L10.3444 2.1366L10.1397 2.17275L9.95906 2.20889L9.75437 2.25107L9.57978 2.29324L9.3811 2.34746L9.20651 2.39566L9.00783 2.45591L8.83926 2.51013L8.6466 2.5764L8.47803 2.63665L8.28538 2.71497L8.12282 2.78124L7.93017 2.86558L7.77364 2.93186L7.58098 3.02825L7.43047 3.10055L7.24383 3.20297L7.09934 3.28129L6.90669 3.39576L6.77424 3.47408L6.56954 3.6006L6.14811 3.88978C6.14811 3.88978 4.32391 2.5041 3.59543 2.12455C3.42904 2.03731 3.24295 1.9946 3.0552 2.00055C2.86745 2.0065 2.68443 2.06092 2.5239 2.15852C2.36336 2.25612 2.23076 2.39359 2.13897 2.55759C2.04718 2.72159 1.99931 2.90655 2.00001 3.09452V12.005C1.99922 14.5454 2.96449 16.9908 4.69985 18.8449C6.43521 20.6989 8.8106 21.8227 11.3438 21.9879H11.4341L11.7351 22H12.4756C12.5659 22 12.6502 21.988 12.7405 21.9819L12.9874 21.9639L13.2462 21.9337L13.4871 21.9036L13.7399 21.8614L13.9807 21.8132C14.065 21.8012 14.1493 21.7771 14.2336 21.759L14.4684 21.7048L14.7152 21.6385L14.944 21.5722L15.1848 21.4939L15.4136 21.4156L15.6484 21.3252L15.8651 21.2349L16.0939 21.1385L16.3107 21.036L16.5334 20.9276L16.7441 20.8192L16.9609 20.6926L17.1656 20.5782L17.3763 20.4456L17.575 20.3191L17.7736 20.1745L17.9663 20.0359C18.0331 19.9891 18.0974 19.9388 18.1589 19.8853L18.3456 19.7407L18.5322 19.5781L18.7128 19.4275L18.8934 19.2527L19.056 19.0961L19.2306 18.9154L19.3931 18.7467L19.5557 18.5599L19.7062 18.3852L19.8627 18.1864L20.0072 18.0056L20.1517 17.8008L20.2842 17.614C20.3323 17.5417 20.3745 17.4694 20.4226 17.4032L20.5491 17.2043C20.5912 17.132 20.6334 17.0598 20.6695 16.9875C20.7056 16.9152 20.7538 16.8549 20.7899 16.7886L20.9043 16.5597L21.0126 16.3549C21.0488 16.2765 21.0849 16.1982 21.115 16.1199C21.1451 16.0416 21.1812 15.9813 21.2113 15.909C21.2414 15.8367 21.2775 15.7464 21.3076 15.668L21.3919 15.4572C21.422 15.3728 21.4461 15.2885 21.4762 15.2041L21.5485 14.9873C21.5786 14.9029 21.5966 14.8125 21.6267 14.7282C21.6568 14.6439 21.6689 14.5836 21.6869 14.5113C21.705 14.439 21.7291 14.3306 21.7471 14.2462C21.7652 14.1619 21.7833 14.1016 21.7953 14.0293L21.8495 13.7462L21.8856 13.5353C21.9037 13.4329 21.9097 13.3365 21.9217 13.2341C21.9338 13.1317 21.9398 13.1015 21.9518 13.0292C21.9639 12.9569 21.9699 12.8063 21.9759 12.6919C21.9829 12.6359 21.9869 12.5796 21.988 12.5232C21.994 12.3605 22 12.1978 22 12.0352V12.005C22 9.59107 21.128 7.25845 19.5448 5.43717C17.9615 3.61588 15.7738 2.42869 13.3847 2.09442ZM17.8399 10.7278L12.0301 16.5356C11.6643 16.9013 11.1685 17.1067 10.6514 17.1067C10.1344 17.1067 9.6385 16.9013 9.27273 16.5356L6.16015 13.4269C5.82451 13.0542 5.64465 12.5668 5.65766 12.0652C5.67067 11.5637 5.87558 11.0863 6.23009 10.7315C6.5846 10.3768 7.06167 10.1717 7.56286 10.1587C8.06405 10.1457 8.55111 10.3257 8.92354 10.6615L10.6514 12.3966L15.0765 7.96247C15.4489 7.6266 15.936 7.44661 16.4371 7.45963C16.9383 7.47265 17.4154 7.6777 17.7699 8.03246C18.1244 8.38722 18.3293 8.86463 18.3423 9.36617C18.3554 9.86771 18.1755 10.3551 17.8399 10.7278Z"/>',
  },
  {
    key: 'telegram',
    name: 'تلگرام',
    url: 'https://t.me/DotinSchool',
    icon: '<path fill="currentColor" d="M9.78 18.65l.28-4.23 7.68-6.92c.34-.31-.07-.46-.52-.19L7.74 13.3 3.64 12c-.88-.25-.89-.86.2-1.3l15.97-6.16c.73-.33 1.43.18 1.15 1.3l-2.72 12.81c-.19.91-.74 1.13-1.5.71L12.6 16.3l-1.99 1.93c-.23.23-.42.42-.83.42z"/>',
  },
  {
    key: 'x',
    name: 'ایکس',
    url: 'https://x.com/dotinschool',
    icon: '<path fill="currentColor" d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/>',
  },
  {
    key: 'linkedin',
    name: 'لینکدین',
    url: 'https://zaya.link/linkedin',
    icon: '<path fill="currentColor" d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 1 1 0-4.125 2.062 2.062 0 0 1 0 4.125zm1.782 13.019H3.555V9h3.564v11.452z"/>',
  },
]

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
  socialDismissed = false
  clearTimeout(socialTimer)
  showSocial.value = false
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
  clearTimeout(socialTimer)
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
        <span class="g-logo-text"><b>لیگ قهرمانان</b> برنامه‌نویسی</span>
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
            <p class="g-kicker">&lt;PCL /&gt;</p>
            <h1>به <b>لیگ قهرمانان</b> برنامه‌نویسی خوش آمدی!</h1>
            <p>
              {{ toFa(qCount) }} سوال چهارگزینه‌ای تکنولوژی و برنامه‌نویسی، هر سوال {{ toFa(qSecs) }} ثانیه.
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
                <li><AppIcon name="list" :size="18" /><span><b>{{ toFa(qCount) }}</b> سوال چهارگزینه‌ای از حوزه‌ی تکنولوژی و برنامه‌نویسی</span></li>
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
                <span class="g-cat">
                  <AppIcon :name="q.category === 'برنامه‌نویسی' ? 'code' : 'cpu'" :size="13" /> {{ q.category }}
                </span>
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

          <!-- after a few seconds: logo + social links -->
          <div v-else-if="phase === 'finished' && showSocial" class="g-social">
            <img class="g-social-logo" :src="dotinLogo" alt="مدرسه داتین" width="140" height="120" />
            <h2>ممنون که در <b>لیگ قهرمانان</b> برنامه‌نویسی شرکت کردی!</h2>
            <p>ما را در شبکه‌های اجتماعی دنبال کن تا از نتایج و رویدادهای بعدی باخبر شوی</p>
            <div class="g-social-grid">
              <a
                v-for="(sl, i) in SOCIALS"
                :key="sl.key"
                :href="sl.url"
                target="_blank"
                rel="noopener noreferrer"
                class="g-social-link"
                :class="'s-' + sl.key"
                :style="{ animationDelay: 0.35 + i * 0.09 + 's' }"
              >
                <span class="g-social-ico">
                  <svg viewBox="0 0 24 24" width="28" height="28" aria-hidden="true" v-html="sl.icon" />
                </span>
                <span class="g-social-name">{{ sl.name }}</span>
              </a>
            </div>
            <div class="g-social-actions">
              <button type="button" class="g-btn g-btn-ghost" @click="backToResult">
                <AppIcon name="list" :size="18" /> مشاهده‌ی دوباره‌ی نتیجه
              </button>
              <button v-if="st.testMode" type="button" class="g-btn g-btn-replay" :disabled="busy" @click="replay">
                <AppIcon name="rotate-ccw" :size="18" /> بازی دوباره (حالت تست)
              </button>
            </div>
          </div>

          <!-- result: only how many were right; the answers stay on screen -->
          <div v-else-if="phase === 'finished' && st.result" class="g-result">
            <div class="g-card g-result-card">
              <img class="g-result-logo" :src="dotinLogo" alt="مدرسه داتین" width="96" height="82" />
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
  --g-bg0: #020d08;
  --g-bg1: #063a24;
  --g-glass: rgba(6, 32, 21, 0.72);
  --g-glass-2: rgba(10, 48, 32, 0.7);
  --g-line: rgba(60, 207, 141, 0.22);
  --g-line-strong: rgba(60, 207, 141, 0.45);
  --g-text: #eaf6ef;
  --g-dim: #a7c4b5;
  --g-faint: #6f9483;
  --g-cyan: #3ccf8d;
  --g-blue: #006839;
  --g-violet: #f68b1f;
  --g-green: #006839; /* Dotin brand green */
  --g-orange: #f68b1f; /* Dotin brand orange */
  --g-ok: #10b981;
  --g-bad: #f43f5e;
  --g-warn: #f68b1f;
  /* accent text (logo "Game", kicker, option letters, phone field) uses the
     same IRANYekanX as everything else, so Persian never falls back to a
     system font */
  --g-mono: var(--font-ui);

  position: relative;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  color: var(--g-text);
  background:
    radial-gradient(120% 70% at 50% -10%, #0a4a2e 0%, transparent 60%),
    radial-gradient(90% 60% at 100% 110%, rgba(246, 139, 31, 0.16) 0%, transparent 60%),
    linear-gradient(180deg, var(--g-bg1) 0%, var(--g-bg0) 75%);
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
    linear-gradient(rgba(60, 207, 141, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(60, 207, 141, 0.07) 1px, transparent 1px);
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
  background: rgba(0, 104, 57, 0.55);
  top: -110px;
  inset-inline-end: -90px;
}
.gl2 {
  width: 280px;
  height: 280px;
  background: rgba(246, 139, 31, 0.22);
  bottom: 8%;
  inset-inline-start: -130px;
}
.gl3 {
  width: 220px;
  height: 220px;
  background: rgba(0, 104, 57, 0.2);
  bottom: -80px;
  inset-inline-end: 5%;
}
.g-scan {
  position: absolute;
  inset-inline: 0;
  top: 0;
  height: 120px;
  background: linear-gradient(180deg, transparent, rgba(60, 207, 141, 0.06), transparent);
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
  background: linear-gradient(135deg, #0a8f54, var(--g-green));
  color: #fff;
  box-shadow: 0 0 18px rgba(60, 207, 141, 0.4);
}
.g-logo-text {
  font-size: 1.05rem;
  font-weight: 700;
  white-space: nowrap;
}
.g-logo-text b {
  font-family: var(--g-mono);
  font-weight: 700;
  color: var(--g-orange);
  text-shadow: 0 0 12px rgba(246, 139, 31, 0.55);
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
  filter: drop-shadow(0 0 18px rgba(60, 207, 141, 0.45));
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
  /* one line on every screen: shrink with the viewport instead of wrapping */
  font-size: clamp(0.95rem, 4.6vw, 1.4rem);
  white-space: nowrap;
  font-weight: 800;
  margin: 0 0 8px;
}
.g-hero h1 b {
  font-family: var(--g-mono);
  color: var(--g-orange);
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
  background: rgba(2, 14, 9, 0.65);
  color: var(--g-text);
  font-family: inherit;
  font-size: 16px; /* >= 16px: stops iOS zooming into the field */
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.g-field input:focus {
  border-color: var(--g-cyan);
  box-shadow: 0 0 0 3px rgba(60, 207, 141, 0.2);
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
  -webkit-box-shadow: inset 0 0 0 1000px #052016 !important;
  box-shadow: inset 0 0 0 1000px #052016 !important;
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
  background: linear-gradient(135deg, #ff9d3d, #f68b1f 45%, #e2730a);
  color: #fff;
  box-shadow: 0 0 22px rgba(60, 207, 141, 0.35);
}
.g-btn-start {
  min-height: 64px;
  font-size: 1.25rem;
  background: linear-gradient(135deg, #0a8f54, #006839);
  color: #fff;
  box-shadow: 0 0 0 1px rgba(60, 207, 141, 0.6), 0 0 34px rgba(60, 207, 141, 0.45);
  margin-bottom: 10px;
  animation: g-glow 2.4s ease-in-out infinite;
}
.g-btn-start:disabled {
  animation: none;
}

.g-btn-replay {
  background: rgba(246, 139, 31, 0.14);
  border: 1px dashed rgba(246, 139, 31, 0.6);
  color: #ffb877;
}
.g-test-badge {
  padding: 2px 10px;
  border-radius: 999px;
  border: 1px dashed rgba(246, 139, 31, 0.6);
  color: #ffb877;
  font-size: 0.72rem;
  font-weight: 700;
  white-space: nowrap;
}

/* ---------- follow-us screen ---------- */
.g-social {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  padding: 20px 0 10px;
  text-align: center;
}
.g-social-logo {
  width: 140px;
  height: auto;
  filter: drop-shadow(0 0 26px rgba(60, 207, 141, 0.5));
  animation: g-logo-in 0.9s cubic-bezier(0.34, 1.56, 0.64, 1) both, g-float 3.2s ease-in-out 0.9s infinite;
}
.g-social h2 {
  color: var(--g-text);
  font-size: 1.3rem;
  font-weight: 900;
  margin: 6px 0 0;
  animation: g-rise 0.6s 0.2s both;
}
.g-social h2 b {
  font-family: var(--g-mono);
  color: var(--g-orange);
}
.g-social p {
  color: var(--g-dim);
  font-size: 0.92rem;
  max-width: 340px;
  margin: 0;
  animation: g-rise 0.6s 0.3s both;
}
.g-social-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  width: 100%;
  max-width: 420px;
  margin-top: 10px;
}
.g-social-link {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: var(--g-text);
  text-decoration: none;
  animation: g-pop-up 0.55s cubic-bezier(0.34, 1.56, 0.64, 1) both;
}
.g-social-link:hover {
  opacity: 1;
}
.g-social-ico {
  display: grid;
  place-items: center;
  width: 58px;
  height: 58px;
  border-radius: 18px;
  color: #fff;
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.35);
  transition: transform 0.15s;
}
.g-social-link:active .g-social-ico {
  transform: scale(0.92);
}
@media (hover: hover) {
  .g-social-link:hover .g-social-ico {
    transform: translateY(-4px);
  }
}
.s-instagram .g-social-ico {
  background: radial-gradient(circle at 30% 107%, #fdf497 0%, #fd5949 45%, #d6249f 60%, #285aeb 90%);
}
.s-bale .g-social-ico {
  background: #00b894;
}
.s-telegram .g-social-ico {
  background: linear-gradient(135deg, #37aee2, #1e96c8);
}
.s-x .g-social-ico {
  background: #000;
  border: 1px solid rgba(255, 255, 255, 0.25);
}
.s-linkedin .g-social-ico {
  background: #0a66c2;
}
.g-social-name {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--g-dim);
}
.g-social-actions {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  max-width: 380px;
  margin-top: 18px;
  animation: g-rise 0.6s 0.9s both;
}
.g-btn-ghost {
  background: var(--g-glass);
  border: 1px solid var(--g-line);
  color: var(--g-text);
}
@keyframes g-logo-in {
  0% {
    opacity: 0;
    transform: scale(0.4) rotate(-8deg);
  }
  100% {
    opacity: 1;
    transform: none;
  }
}
@keyframes g-pop-up {
  0% {
    opacity: 0;
    transform: translateY(24px) scale(0.6);
  }
  100% {
    opacity: 1;
    transform: none;
  }
}
@media (max-width: 360px) {
  .g-social-grid {
    gap: 6px;
  }
  .g-social-ico {
    width: 50px;
    height: 50px;
    border-radius: 15px;
  }
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
  background: linear-gradient(135deg, #0a8f54, var(--g-green));
  color: #fff;
  font-weight: 800;
  font-size: 1.1rem;
  box-shadow: 0 0 16px rgba(246, 139, 31, 0.45);
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
  background: rgba(246, 139, 31, 0.12);
  border: 1px solid rgba(246, 139, 31, 0.45);
  color: #ffb877;
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
  box-shadow: 0 0 10px rgba(60, 207, 141, 0.6);
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
  background: rgba(6, 32, 21, 0.9);
  border: 1px solid var(--g-line);
  overflow: hidden;
}
.g-timer-fill {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--g-blue), var(--g-cyan));
  box-shadow: 0 0 12px rgba(60, 207, 141, 0.7);
  transition: width 0.1s linear, background 0.3s;
}
.g-timer.warn .g-timer-fill {
  background: var(--g-warn);
  box-shadow: 0 0 12px rgba(246, 139, 31, 0.7);
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
  background: linear-gradient(160deg, rgba(10, 48, 32, 0.85), rgba(4, 24, 15, 0.85));
  border: 1px solid var(--g-line-strong);
  border-radius: 22px;
  padding: 32px 20px 26px;
  min-height: 150px;
  display: grid;
  place-items: center;
  text-align: center;
  box-shadow: 0 0 30px rgba(60, 207, 141, 0.12), 0 14px 34px rgba(0, 0, 0, 0.45);
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
  background: #3a2006;
  border: 1px solid var(--g-orange);
  color: #ffb877;
  font-size: 0.8rem;
  font-weight: 800;
  white-space: nowrap;
  box-shadow: 0 0 14px rgba(246, 139, 31, 0.35);
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
  background: rgba(60, 207, 141, 0.1);
  border: 1px solid rgba(60, 207, 141, 0.35);
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
  box-shadow: 0 0 18px rgba(60, 207, 141, 0.45);
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
.g-result-logo {
  display: block;
  width: 96px;
  height: auto;
  margin: 0 auto 14px;
  filter: drop-shadow(0 0 18px rgba(60, 207, 141, 0.45));
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
    radial-gradient(closest-side, #052016 80%, transparent 81%),
    conic-gradient(var(--g-cyan) var(--pct), rgba(60, 207, 141, 0.15) 0);
  box-shadow: 0 0 30px rgba(60, 207, 141, 0.3);
}
.g-score-num {
  font-size: 2.6rem;
  font-weight: 900;
  line-height: 1.1;
  color: var(--g-cyan);
  text-shadow: 0 0 14px rgba(60, 207, 141, 0.6);
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
  background: rgba(60, 207, 141, 0.07);
  border: 1px dashed rgba(60, 207, 141, 0.3);
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
  background: rgba(2, 14, 9, 0.5);
  border: 1px solid rgba(60, 207, 141, 0.12);
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
  border: 1px solid rgba(60, 207, 141, 0.35);
}

/* ---------- spinner ---------- */
.g-spinner {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  border: 3px solid rgba(60, 207, 141, 0.2);
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
    box-shadow: 0 0 0 1px rgba(60, 207, 141, 0.6), 0 0 24px rgba(60, 207, 141, 0.35);
  }
  50% {
    box-shadow: 0 0 0 1px rgba(60, 207, 141, 0.9), 0 0 40px rgba(60, 207, 141, 0.6);
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
