<script setup lang="ts">
// Full-screen winner announcement: 3rd → 2nd → 1st. For each place a
// slot-machine roller flips through every participant's name, slowing down
// over the configured seconds while a heartbeat speeds up; a beat of silence,
// then the winner lands with a fanfare and confetti. Built for a projector.
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { fmtSecs, type BoardRow } from '@/api/game'
import { toFa } from '@/lib/fa'
import { createRevealSound, type RevealSound } from '@/lib/revealSound'
import AppIcon from './AppIcon.vue'
import dotinLogo from '@/assets/brand/dotin-school-logo-w.svg'

const props = defineProps<{
  podium: BoardRow[] // ranked, up to 3
  names: string[] // every participant, for the roller
  timing: { third: number; second: number; first: number; gap: number } // seconds
  /** sample data — shows a "دمو" badge */
  demo?: boolean
  /** the parent is downloading the Excel report */
  reportLoading?: boolean
}>()
const emit = defineEmits<{ close: []; report: [] }>()

const MEDALS = ['🥇', '🥈', '🥉']
const PLACE = ['نفر اول', 'نفر دوم', 'نفر سوم']

type Stage = 'intro' | 'rolling' | 'suspense' | 'revealed' | 'final'
const stage = ref<Stage>('intro')
const place = ref(2) // 0-based place currently on stage
const rollName = ref('')
const rollKey = ref(0)
const beatKey = ref(0)
const progress = ref(0)
const revealed = ref<number[]>([])
const muted = ref(false)
const confetti = ref<{ id: number; left: number; delay: number; dur: number; color: string; size: number; rot: number }[]>([])
const root = ref<HTMLElement | null>(null)

const order = computed(() => [2, 1, 0].filter((i) => props.podium[i]))
const winner = computed(() => props.podium[place.value])
const fullName = (r: BoardRow) => `${r.firstName} ${r.lastName}`

let sound: RevealSound | null = null
let runId = 0
let skipWait: (() => void) | null = null
let revealNow: (() => void) | null = null

const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
/** A wait the presenter can cut short with Space / the "next" button. */
function waitSkippable(ms: number) {
  return new Promise<void>((resolve) => {
    const t = setTimeout(done, ms)
    function done() {
      clearTimeout(t)
      skipWait = null
      resolve()
    }
    skipWait = done
  })
}

function namePool(): string[] {
  const pool = props.names.filter(Boolean)
  const base = pool.length ? pool : props.podium.map(fullName)
  const out: string[] = []
  while (out.length < 12) out.push(...base)
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[out[i], out[j]] = [out[j], out[i]]
  }
  return out
}

function heartbeatLoop(id: number, durMs: number, bpmFrom: number, bpmTo: number) {
  const start = performance.now()
  const beat = () => {
    if (id !== runId || (stage.value !== 'rolling' && stage.value !== 'suspense')) return
    const p = Math.min(1, (performance.now() - start) / durMs)
    sound?.heartbeat(0.6 + 0.7 * p)
    beatKey.value++
    const bpm = bpmFrom + (bpmTo - bpmFrom) * p
    setTimeout(beat, 60000 / bpm)
  }
  beat()
}

async function rollPlace(id: number, p: number) {
  place.value = p
  const durMs = (p === 0 ? props.timing.first : p === 1 ? props.timing.second : props.timing.third) * 1000
  stage.value = 'rolling'
  progress.value = 0
  const stopRiser = sound?.riser(durMs / 1000)
  heartbeatLoop(id, durMs, 66, p === 0 ? 168 : p === 1 ? 140 : 120)

  const pool = namePool()
  const start = performance.now()
  let i = 0
  for (;;) {
    if (id !== runId) return
    const t = (performance.now() - start) / durMs
    if (t >= 1) break
    progress.value = t
    rollName.value = pool[i++ % pool.length]
    rollKey.value++
    sound?.tick()
    // fast at first, then the roller drags — longer for higher places
    await sleep(50 + (p === 0 ? 720 : 560) * Math.pow(t, 2.4))
  }
  if (id !== runId) return

  // The roller always comes to rest on a *decoy*: one name drawn uniformly at
  // random from every participant, chosen without looking at the winner. So
  // the last name before "؟" carries no information — it is the winner only
  // by chance (1 in N), and never systematically excluded either (which would
  // leak just as much).
  const everyone = [...new Set(pool)]
  rollName.value = everyone[Math.floor(Math.random() * everyone.length)]
  rollKey.value++
  sound?.tick()
  await sleep(p === 0 ? 900 : 700)
  if (id !== runId) return
  stopRiser?.()
  progress.value = 1

  // hold on "؟" — heartbeat still racing — until the presenter presses
  // Enter / Space or taps the box (on a phone)
  stage.value = 'suspense'
  await new Promise<void>((resolve) => (revealNow = resolve))
  revealNow = null
  if (id !== runId) return

  stage.value = 'revealed'
  revealed.value = [...revealed.value, p]
  sound?.reveal(p === 0)
  burstConfetti(p === 0 ? 170 : 90)
}

async function run() {
  const id = ++runId
  revealed.value = []
  for (const p of order.value) {
    await rollPlace(id, p)
    if (id !== runId) return
    const last = p === order.value[order.value.length - 1]
    await waitSkippable(last ? 3500 : Math.max(1500, props.timing.gap * 1000 + 1500))
    if (id !== runId) return
  }
  stage.value = 'final'
  burstConfetti(120)
}

function start() {
  if (!sound) sound = createRevealSound()
  sound?.setMuted(muted.value)
  // full screen for the projector; a refusal (browser policy) is harmless
  root.value?.requestFullscreen?.().catch(() => {})
  void run()
}

function replay() {
  confetti.value = []
  void run()
}

function toggleMute() {
  muted.value = !muted.value
  sound?.setMuted(muted.value)
}

function close() {
  runId++
  revealNow?.()
  sound?.close()
  sound = null
  if (document.fullscreenElement) void document.exitFullscreen().catch(() => {})
  emit('close')
}

let confettiSeq = 0
function burstConfetti(n: number) {
  const colors = ['#f68b1f', '#ffb877', '#006839', '#3ccf8d', '#9be7c4', '#ffffff']
  const batch = Array.from({ length: n }, () => ({
    id: ++confettiSeq,
    left: Math.random() * 100,
    delay: Math.random() * 0.6,
    dur: 2.6 + Math.random() * 2.2,
    color: colors[Math.floor(Math.random() * colors.length)],
    size: 6 + Math.random() * 8,
    rot: Math.random() * 360,
  }))
  confetti.value = [...confetti.value, ...batch]
  const ids = new Set(batch.map((b) => b.id))
  setTimeout(() => (confetti.value = confetti.value.filter((c) => !ids.has(c.id))), 5500)
}

/** Enter / Space / tap: start, reveal the held name, or move on. */
function advance() {
  if (stage.value === 'intro') start()
  else if (stage.value === 'suspense') revealNow?.()
  else skipWait?.()
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
  else if (e.key === ' ' || e.key === 'Enter') {
    e.preventDefault()
    advance()
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  runId++
  revealNow?.()
  sound?.close()
})

</script>

<template>
  <Teleport to="body">
    <div ref="root" class="wr" :class="[`st-${stage}`, `pl-${place + 1}`]" role="dialog" aria-label="اعلام نتایج">
      <div class="wr-bg" aria-hidden="true">
        <span class="wr-grid" />
        <span class="wr-spot" />
      </div>

      <header class="wr-top">
        <span class="wr-brand">
          <img class="wr-brand-logo" :src="dotinLogo" alt="" width="34" height="29" />
          اعلام نتایج <b>لیگ قهرمانان</b> برنامه‌نویسی
          <span v-if="demo" class="wr-demo">دمو — داده‌ی نمونه</span>
        </span>
        <div class="wr-tools">
          <button type="button" class="wr-icon" :title="muted ? 'روشن کردن صدا' : 'بی‌صدا'" @click="toggleMute">
            <AppIcon :name="muted ? 'x' : 'bell'" :size="18" />
          </button>
          <button type="button" class="wr-icon" title="بستن (Esc)" @click="close">
            <AppIcon name="x" :size="18" />
          </button>
        </div>
      </header>

      <main class="wr-main">
        <!-- intro -->
        <section v-if="stage === 'intro'" class="wr-intro">
          <img class="wr-intro-logo" :src="dotinLogo" alt="مدرسه داتین" width="150" height="128" />
          <h1>لحظه‌ی اعلام برندگان</h1>
          <button type="button" class="wr-start" @click="start">
            <AppIcon name="play" :size="22" /> شروع اعلام نتایج
          </button>
        </section>

        <!-- roll / suspense / reveal -->
        <section v-else-if="stage !== 'final' && winner" class="wr-show">
          <div class="wr-place">
            <span class="wr-place-medal" :class="{ shown: stage === 'revealed' }">{{ MEDALS[place] }}</span>
            <span class="wr-place-label">{{ PLACE[place] }}</span>
          </div>

          <div
            class="wr-slot"
            :class="{ hot: (stage === 'rolling' && progress > 0.7) || stage === 'suspense', tappable: stage === 'suspense' }"
            @click="stage === 'suspense' && advance()"
          >
            <span class="wr-slot-edge top" />
            <template v-if="stage === 'rolling'">
              <span :key="rollKey" class="wr-roll-name">{{ rollName }}</span>
            </template>
            <span v-else-if="stage === 'suspense'" class="wr-q">؟</span>
            <span v-else class="wr-winner">{{ fullName(winner) }}</span>
            <span class="wr-slot-edge bottom" />
          </div>

          <div v-if="stage === 'rolling' || stage === 'suspense'" class="wr-heart">
            <span :key="beatKey" class="wr-heart-ico">❤</span>
          </div>

          <div v-else class="wr-stats">
            <div class="wr-stat">
              <b class="num">{{ toFa(winner.correct) }} از {{ toFa(winner.total) }}</b>
              <small>سوال درست</small>
            </div>
            <div class="wr-stat">
              <b class="num">{{ fmtSecs(winner.totalMs) }}</b>
              <small>ثانیه زمان پاسخ</small>
            </div>
            <div class="wr-stat gold">
              <b class="num">{{ toFa(winner.score) }}</b>
              <small>امتیاز</small>
            </div>
          </div>

          <button v-if="stage === 'revealed'" type="button" class="wr-next" @click="skipWait?.()">
            {{ place === order[order.length - 1] ? 'نمایش جمع‌بندی' : 'نفر بعدی' }}
            <AppIcon name="chevron-left" :size="18" />
          </button>
        </section>

        <!-- final podium -->
        <section v-else-if="stage === 'final'" class="wr-final">
          <img class="wr-final-logo" :src="dotinLogo" alt="مدرسه داتین" width="96" height="82" />
          <h2>برندگان لیگ قهرمانان برنامه‌نویسی</h2>
          <div class="wr-podium">
            <div v-for="p in [1, 0, 2].filter((i) => podium[i])" :key="p" class="wr-col" :class="'c' + (p + 1)">
              <span class="wr-col-medal">{{ MEDALS[p] }}</span>
              <span class="wr-col-name">{{ fullName(podium[p]) }}</span>
              <span class="wr-col-meta num">
                {{ toFa(podium[p].correct) }} از {{ toFa(podium[p].total) }} سوال درست — {{ fmtSecs(podium[p].totalMs) }} ثانیه
              </span>
              <span class="wr-col-score num">{{ toFa(podium[p].score) }} امتیاز</span>
              <div class="wr-col-block">{{ PLACE[p] }}</div>
            </div>
          </div>
          <div class="wr-final-actions">
            <button v-if="!demo" type="button" class="wr-ghost" :disabled="reportLoading" @click="emit('report')">
              <AppIcon name="download" :size="16" /> {{ reportLoading ? 'در حال آماده‌سازی…' : 'گزارش کامل Excel' }}
            </button>
            <button type="button" class="wr-ghost" @click="replay"><AppIcon name="rotate-ccw" :size="16" /> اجرای دوباره</button>
            <button type="button" class="wr-ghost" @click="close">بستن</button>
          </div>
        </section>
      </main>

      <!-- places revealed so far -->
      <footer v-if="revealed.length && stage !== 'final' && stage !== 'intro'" class="wr-strip">
        <span v-for="p in revealed" :key="p" class="wr-chip">
          {{ MEDALS[p] }} {{ PLACE[p] }}: <b>{{ fullName(podium[p]) }}</b>
        </span>
      </footer>

      <div class="wr-confetti" aria-hidden="true">
        <span
          v-for="c in confetti"
          :key="c.id"
          :style="{
            left: c.left + '%',
            animationDelay: c.delay + 's',
            animationDuration: c.dur + 's',
            background: c.color,
            width: c.size + 'px',
            height: c.size * 0.45 + 'px',
            '--rot': c.rot + 'deg',
          }"
        />
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.wr {
  --gold: #f68b1f; /* Dotin orange */
  --cyan: #3ccf8d; /* bright Dotin green */
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: flex;
  flex-direction: column;
  color: #eef8f2;
  background: radial-gradient(120% 90% at 50% 10%, #073321 0%, #020b07 70%);
  font-family: var(--font-ui);
  overflow: hidden;
}
.wr-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.wr-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(60, 207, 141, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(60, 207, 141, 0.07) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: radial-gradient(ellipse at 50% 45%, #000 25%, transparent 75%);
  -webkit-mask-image: radial-gradient(ellipse at 50% 45%, #000 25%, transparent 75%);
}
.wr-spot {
  position: absolute;
  left: 50%;
  top: 42%;
  width: 900px;
  height: 900px;
  transform: translate(-50%, -50%);
  border-radius: 50%;
  background: radial-gradient(circle, rgba(60, 207, 141, 0.16), transparent 60%);
  transition: background 0.6s;
}
.st-revealed .wr-spot,
.st-final .wr-spot {
  background: radial-gradient(circle, rgba(246, 139, 31, 0.25), transparent 60%);
}
.st-suspense .wr-spot {
  background: radial-gradient(circle, rgba(244, 63, 94, 0.14), transparent 55%);
}

.wr-top {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 24px;
}
.wr-brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  color: #b6d2c3;
}
.wr-brand b {
  color: var(--cyan);
}
.wr-demo {
  padding: 2px 10px;
  border-radius: 999px;
  border: 1px dashed rgba(246, 139, 31, 0.7);
  color: #ffb877;
  font-size: 0.78rem;
}
.wr-tools {
  display: flex;
  gap: 8px;
}
.wr-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  border: 1px solid rgba(60, 207, 141, 0.3);
  background: rgba(6, 32, 21, 0.7);
  color: #eaf6ef;
}

.wr-main {
  position: relative;
  z-index: 2;
  flex: 1;
  display: grid;
  place-items: center;
  padding: 0 20px;
}

/* ---- intro ---- */
.wr-intro {
  text-align: center;
  max-width: 720px;
}
.wr-brand-logo {
  height: 28px;
  width: auto;
}
.wr-intro-logo {
  display: block;
  width: clamp(110px, 16vw, 150px);
  height: auto;
  margin: 0 auto 22px;
  filter: drop-shadow(0 0 26px rgba(60, 207, 141, 0.5));
  animation: float 3s ease-in-out infinite;
}
.wr-final-logo {
  display: block;
  width: clamp(70px, 9vw, 96px);
  height: auto;
  margin: 0 auto 10px;
  filter: drop-shadow(0 0 22px rgba(246, 139, 31, 0.45));
}
.wr-intro h1 {
  color: #fff;
  font-size: clamp(1.8rem, 4.5vw, 3rem);
  font-weight: 900;
  margin-bottom: 10px;
}
.wr-intro p {
  color: #b6d2c3;
  font-size: 1.15rem;
}
.wr-start {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-top: 26px;
  padding: 16px 40px;
  border: 0;
  border-radius: 18px;
  font-size: 1.3rem;
  font-weight: 900;
  color: #1a1204;
  background: linear-gradient(135deg, var(--gold), #f97316);
  box-shadow: 0 0 40px rgba(246, 139, 31, 0.5);
  animation: glow 2s ease-in-out infinite;
}
.wr-slot.tappable {
  cursor: pointer;
}
.wr-hint {
  margin-top: 18px;
  font-size: 0.9rem !important;
  color: #6f9483 !important;
}

/* ---- the show ---- */
.wr-show {
  width: 100%;
  max-width: 1000px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 26px;
  text-align: center;
}
.wr-place {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}
.wr-place-medal {
  font-size: 4.5rem;
  line-height: 1;
  opacity: 0.25;
  filter: grayscale(1);
  transition: opacity 0.4s, filter 0.4s, transform 0.5s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.wr-place-medal.shown {
  opacity: 1;
  filter: none;
  transform: scale(1.25);
}
.wr-place-label {
  font-size: clamp(1.6rem, 4vw, 2.6rem);
  font-weight: 900;
  color: var(--cyan);
  text-shadow: 0 0 22px rgba(60, 207, 141, 0.5);
}
.pl-1 .wr-place-label {
  color: var(--gold);
  text-shadow: 0 0 26px rgba(246, 139, 31, 0.6);
}

.wr-slot {
  position: relative;
  width: min(92vw, 900px);
  height: clamp(110px, 18vh, 170px);
  display: grid;
  place-items: center;
  overflow: hidden;
  border-radius: 26px;
  border: 2px solid rgba(60, 207, 141, 0.45);
  background: linear-gradient(180deg, rgba(2, 12, 8, 0.9), rgba(8, 44, 28, 0.85) 50%, rgba(2, 12, 8, 0.9));
  box-shadow: 0 0 40px rgba(60, 207, 141, 0.2), inset 0 0 40px rgba(0, 0, 0, 0.6);
  transition: border-color 0.3s, box-shadow 0.3s;
}
.wr-slot.hot {
  border-color: rgba(244, 63, 94, 0.7);
  box-shadow: 0 0 50px rgba(244, 63, 94, 0.35), inset 0 0 40px rgba(0, 0, 0, 0.6);
  animation: shake 0.25s linear infinite;
}
.st-revealed .wr-slot {
  border-color: var(--gold);
  box-shadow: 0 0 70px rgba(246, 139, 31, 0.5), inset 0 0 40px rgba(0, 0, 0, 0.5);
}
.wr-slot-edge {
  position: absolute;
  inset-inline: 0;
  height: 34%;
  pointer-events: none;
  z-index: 1;
}
.wr-slot-edge.top {
  top: 0;
  background: linear-gradient(180deg, rgba(2, 12, 8, 0.95), transparent);
}
.wr-slot-edge.bottom {
  bottom: 0;
  background: linear-gradient(0deg, rgba(2, 12, 8, 0.95), transparent);
}
.wr-roll-name {
  font-size: clamp(1.8rem, 5vw, 3.4rem);
  font-weight: 900;
  color: #eaf6ef;
  white-space: nowrap;
  animation: roll-in 0.09s linear;
}
.wr-q {
  font-size: clamp(3rem, 8vw, 5rem);
  font-weight: 900;
  color: #fda4af;
  animation: q-pulse 0.8s ease-in-out infinite;
}
.wr-winner {
  font-size: clamp(2rem, 6vw, 4rem);
  font-weight: 900;
  color: var(--gold);
  text-shadow: 0 0 30px rgba(246, 139, 31, 0.7);
  white-space: nowrap;
  animation: winner-in 0.7s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.wr-heart {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  width: min(80vw, 520px);
}
.wr-heart-ico {
  font-size: 3rem;
  line-height: 1;
  color: #f43f5e;
  text-shadow: 0 0 24px rgba(244, 63, 94, 0.8);
  animation: beat 0.35s ease-out;
}
.wr-bar {
  width: 100%;
  height: 8px;
  border-radius: 999px;
  background: rgba(60, 207, 141, 0.15);
  overflow: hidden;
}
.wr-bar-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--cyan), #f43f5e);
  box-shadow: 0 0 14px rgba(244, 63, 94, 0.6);
}

.wr-stats {
  display: flex;
  gap: 14px;
  flex-wrap: wrap;
  justify-content: center;
  animation: rise 0.6s 0.3s both cubic-bezier(0.22, 1, 0.36, 1);
}
.wr-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 150px;
  padding: 14px 22px;
  border-radius: 18px;
  background: rgba(6, 32, 21, 0.8);
  border: 1px solid rgba(60, 207, 141, 0.35);
}
.wr-stat b {
  font-size: clamp(1.6rem, 3.5vw, 2.4rem);
  font-weight: 900;
  color: var(--cyan);
  line-height: 1.3;
}
.wr-stat small {
  font-size: 0.95rem;
  color: #a7c4b5;
  font-weight: 600;
}
.wr-stat.gold {
  border-color: rgba(246, 139, 31, 0.6);
}
.wr-stat.gold b {
  color: var(--gold);
}
.wr-next {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 22px;
  border-radius: 14px;
  border: 1px solid rgba(60, 207, 141, 0.4);
  background: rgba(6, 32, 21, 0.7);
  color: #eaf6ef;
  font-weight: 700;
  font-size: 1rem;
}

/* ---- strip of revealed places ---- */
.wr-strip {
  position: relative;
  z-index: 2;
  display: flex;
  justify-content: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 0 20px 26px;
}
.wr-chip {
  padding: 8px 16px;
  border-radius: 999px;
  background: rgba(6, 32, 21, 0.8);
  border: 1px solid rgba(246, 139, 31, 0.4);
  color: #c7ddd1;
  animation: rise 0.5s both;
}
.wr-chip b {
  color: #fff;
}

/* ---- final ---- */
.wr-final {
  width: 100%;
  max-width: 1000px;
  text-align: center;
}
.wr-final h2 {
  color: #fff;
  font-size: clamp(1.6rem, 4vw, 2.6rem);
  font-weight: 900;
  margin-bottom: 30px;
}
.wr-podium {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 16px;
}
.wr-col {
  flex: 1;
  max-width: 290px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  animation: rise 0.7s both cubic-bezier(0.22, 1, 0.36, 1);
}
.wr-col.c1 {
  animation-delay: 0.5s;
}
.wr-col.c2 {
  animation-delay: 0.25s;
}
.wr-col-medal {
  font-size: 3.4rem;
  line-height: 1.1;
}
.wr-col.c1 .wr-col-medal {
  font-size: 4.6rem;
}
.wr-col-name {
  font-size: clamp(1.05rem, 2.4vw, 1.6rem);
  font-weight: 900;
  color: #fff;
}
.wr-col-meta {
  font-size: 0.95rem;
  color: #a7c4b5;
}
.wr-col-score {
  font-size: 1.15rem;
  font-weight: 800;
  color: var(--gold);
}
.wr-col-block {
  width: 100%;
  margin-top: 10px;
  display: grid;
  place-items: center;
  border-radius: 16px 16px 0 0;
  color: #fff;
  font-size: 1.2rem;
  font-weight: 900;
}
.wr-col.c1 .wr-col-block {
  height: 200px;
  background: linear-gradient(180deg, #f68b1f, #b45309);
  box-shadow: 0 0 50px rgba(246, 139, 31, 0.4);
}
.wr-col.c2 .wr-col-block {
  height: 150px;
  background: linear-gradient(180deg, #cbd5e1, #475569);
}
.wr-col.c3 .wr-col-block {
  height: 110px;
  background: linear-gradient(180deg, #f68b1f, #78350f);
}
.wr-final-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 26px;
}
.wr-ghost {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 22px;
  border-radius: 14px;
  border: 1px solid rgba(60, 207, 141, 0.4);
  background: rgba(6, 32, 21, 0.7);
  color: #eaf6ef;
  font-weight: 700;
}

/* ---- confetti ---- */
.wr-confetti {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 5;
  overflow: hidden;
}
.wr-confetti span {
  position: absolute;
  top: -20px;
  border-radius: 2px;
  animation-name: fall;
  animation-timing-function: cubic-bezier(0.25, 0.6, 0.45, 1);
  animation-fill-mode: both;
}

@keyframes fall {
  from {
    transform: translate3d(0, -20px, 0) rotate(var(--rot));
    opacity: 1;
  }
  to {
    transform: translate3d(40px, 105vh, 0) rotate(calc(var(--rot) + 720deg));
    opacity: 0.8;
  }
}
@keyframes roll-in {
  from {
    transform: translateY(-60%);
    opacity: 0.3;
  }
  to {
    transform: none;
    opacity: 1;
  }
}
@keyframes winner-in {
  0% {
    transform: scale(0.4);
    opacity: 0;
    filter: blur(8px);
  }
  100% {
    transform: none;
    opacity: 1;
    filter: none;
  }
}
@keyframes q-pulse {
  50% {
    transform: scale(1.15);
    opacity: 0.6;
  }
}
@keyframes beat {
  0% {
    transform: scale(1.35);
  }
  100% {
    transform: scale(1);
  }
}
@keyframes shake {
  25% {
    transform: translate(1px, -1px);
  }
  75% {
    transform: translate(-1px, 1px);
  }
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(26px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@keyframes float {
  50% {
    transform: translateY(-8px);
  }
}
@keyframes glow {
  50% {
    box-shadow: 0 0 60px rgba(246, 139, 31, 0.8);
  }
}
@media (max-width: 640px) {
  .wr-podium {
    gap: 6px;
  }
  .wr-col-meta {
    font-size: 0.75rem;
  }
  .wr-show {
    gap: 16px;
  }
  .wr-place-medal {
    font-size: 3.2rem;
  }
  .wr-stats {
    width: 100%;
    flex-wrap: nowrap;
    gap: 6px;
  }
  .wr-stat {
    flex: 1;
    min-width: 0;
    padding: 8px 4px;
  }
  .wr-stat b {
    font-size: 1.35rem;
  }
  .wr-stat small {
    font-size: 0.7rem;
  }
  .wr-strip {
    gap: 6px;
    padding-bottom: 16px;
  }
  .wr-chip {
    padding: 4px 10px;
    font-size: 0.78rem;
  }
}
</style>
