<script setup lang="ts">
// Live quiz-game board: settings, the auto-ranked podium (1st–3rd), and every
// player's score and answer time. Polled every 1.5 s while the page is open.
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { message, Modal, notification } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import {
  gameBoard,
  saveGameSettings,
  gamePlayerReview,
  resetGamePlayer,
  deleteGamePlayer,
  deleteAllGamePlayers,
  fmtSecs,
  difficultyPlan,
  downloadGameReport,
  mixPlan,
  DIFFICULTY_LABEL,
  type GameMix,
  type GameBoard,
  type BoardRow,
  type GameReviewItem,
} from '@/api/game'
import { toFa, pad2, digitsOnly } from '@/lib/fa'
import StatCard from '@/components/StatCard.vue'
import Chip from '@/components/Chip.vue'
import AppIcon from '@/components/AppIcon.vue'
import WinnerReveal from '@/components/WinnerReveal.vue'

const admin = useAdminStore()
const board = ref<GameBoard | null>(null)
let poller: number | undefined

async function refresh() {
  try {
    board.value = await gameBoard()
    if (!settingsDirty.value) syncSettingsForm()
    if (!mixDirty.value) syncMixDraft()
  } catch (e) {
    admin.handleAuthError(e)
  }
}

onMounted(() => {
  void refresh()
  poller = window.setInterval(refresh, 1500)
})
onBeforeUnmount(() => clearInterval(poller))

/* ---------------- settings ---------------- */
const sform = reactive({
  open: true,
  questionsPerGame: 3,
  secondsPerQuestion: 20,
  finishMessage: '',
  revealThirdSec: 6,
  revealSecondSec: 9,
  revealFirstSec: 15,
  revealGapSec: 4,
  testMode: false,
  socialDelaySec: 8,
  mixOn: true,
  mix: [] as GameMix[],
})
const settingsDirty = ref(false)
const savingSettings = ref(false)
function syncSettingsForm() {
  const s = board.value?.settings
  if (!s) return
  Object.assign(sform, s)
}
function markDirty() {
  settingsDirty.value = true
}
async function saveSettings(patch?: Partial<typeof sform>) {
  if (patch) Object.assign(sform, patch)
  savingSettings.value = true
  try {
    await saveGameSettings({ ...sform })
    settingsDirty.value = false
    await refresh()
    message.success('تنظیمات بازی ذخیره شد')
  } catch (e) {
    admin.toastError(e, 'ذخیره تنظیمات ناموفق بود')
  } finally {
    savingSettings.value = false
  }
}
function toggleOpen(v: boolean | string | number) {
  void saveSettings({ open: !!v })
}
/* the big start / close control at the top of the page */
function confirmOpen(open: boolean) {
  Modal.confirm({
    title: open ? 'شروع بازی' : 'بستن بازی',
    content: open
      ? 'بازی همین حالا برای همه باز می‌شود و کسانی که روی صفحه انتظار هستند، چند ثانیه بعد خودکار وارد ثبت‌نام می‌شوند.'
      : 'کسی دیگر نمی‌تواند ثبت‌نام یا بازی را شروع کند و صفحه انتظار نمایش داده می‌شود. کسانی که الان وسط بازی هستند، بازی‌شان را تمام می‌کنند.',
    okText: open ? 'بله، شروع کن' : 'بله، ببند',
    okButtonProps: open ? {} : { danger: true },
    cancelText: 'انصراف',
    onOk: () => saveSettings({ open }),
  })
}
/* questions per game: one-tap presets + a custom count, saved immediately */
const QC_PRESETS = [3, 5, 10, 15, 20]
const qcCustom = ref<number | undefined>(undefined)
/* seconds to answer each question: presets + custom, saved immediately */
const QT_PRESETS = [10, 15, 20, 30, 45, 60]
const qtCustom = ref<number | undefined>(undefined)
async function setQuestionTime(n: number) {
  if (n < 5 || n > 300) return message.error('زمان هر سوال باید بین ۵ تا ۳۰۰ ثانیه باشد')
  const s = board.value?.settings
  if (s) Object.assign(sform, s)
  await saveSettings({ secondsPerQuestion: n })
  qtCustom.value = undefined
}

async function setQuestionCount(n: number) {
  const bank = board.value?.questionCount ?? 0
  if (n < 1 || n > 50) return message.error('تعداد سوال باید بین ۱ تا ۵۰ باشد')
  if (n > bank) return message.error(`بانک فقط ${toFa(bank)} سوال دارد — اول سوال اضافه کنید`)
  // keep any other unsaved edits out of this one-click save
  const s = board.value?.settings
  if (s) Object.assign(sform, s)
  await saveSettings({ questionsPerGame: n })
  qcCustom.value = undefined
}
/* ---------------- category mix ---------------- */
// the draft the admin edits; synced from the server while untouched
const mixDraft = ref<GameMix[]>([])
const mixDirty = ref(false)
function syncMixDraft() {
  const m = board.value?.settings.mix
  if (m) mixDraft.value = m.map((r) => ({ ...r }))
}
function cancelMix() {
  mixDirty.value = false
  syncMixDraft()
}
const mixOn = computed(() => board.value?.settings.mixOn ?? sform.mixOn)
const mixTotal = computed(() => mixDraft.value.reduce((n, r) => n + (r.easy || 0) + (r.hard || 0), 0))
const stockOf = (cat: string) => board.value?.categories?.find((c) => c.category === cat) ?? { easy: 0, hard: 0 }
const categoryOptions = computed(() => {
  const used = new Set(mixDraft.value.map((r) => r.category))
  return (board.value?.categories ?? []).map((c) => ({
    value: c.category,
    label: `${c.category} (${toFa(c.easy)} آسان، ${toFa(c.hard)} سخت)`,
    disabled: used.has(c.category),
  }))
})
/** the order every player gets the questions in */
const mixOrder = computed(() => mixPlan(mixDraft.value.filter((r) => r.category)))
function addMixRow() {
  const free = categoryOptions.value.find((o) => !o.disabled)
  mixDraft.value.push({ category: free?.value ?? '', easy: 1, hard: 1 })
  mixDirty.value = true
}
function removeMixRow(i: number) {
  mixDraft.value.splice(i, 1)
  mixDirty.value = true
}
async function saveMix(on = mixOn.value) {
  const mix = mixDraft.value.map((r) => ({ category: r.category, easy: r.easy || 0, hard: r.hard || 0 }))
  if (on) {
    if (mix.some((r) => !r.category && r.easy + r.hard > 0)) return message.error('دسته هر ردیف را انتخاب کنید')
    if (mixTotal.value < 1 || mixTotal.value > 50) return message.error('جمع سوالات ترکیب باید بین ۱ تا ۵۰ باشد')
  }
  const s = board.value?.settings
  if (s) Object.assign(sform, s)
  await saveSettings({ mixOn: on, mix })
  mixDirty.value = false
  syncMixDraft()
}
function toggleMix(v: boolean | string | number) {
  void saveMix(!!v)
}

function toggleTestMode(v: boolean | string | number) {
  void saveSettings({ testMode: !!v })
}

/* ---------------- podium + announcements ---------------- */
const podium = computed(() => board.value?.podium ?? [])
const MEDALS = ['🥇', '🥈', '🥉']
const PLACE = ['نفر اول', 'نفر دوم', 'نفر سوم']

// Tell the admin whenever the top three change (not on the first load).
let lastPodiumKey: string | null = null
watch(podium, (p) => {
  const key = p.map((r) => r.id).join(',')
  if (lastPodiumKey !== null && key !== lastPodiumKey && p.length) {
    notification.success({
      message: 'جدول نفرات برتر به‌روز شد',
      description: p.map((r, i) => `${MEDALS[i]} ${r.firstName} ${r.lastName} — ${toFa(r.score)} امتیاز`).join('\n'),
      duration: 6,
      style: { whiteSpace: 'pre-line' },
    })
  }
  lastPodiumKey = key
})

const announceOpen = ref(false)

/* animated winner show */
const showOpen = ref(false)

/* demo of the show with made-up players — nothing is read from or written to the DB */
const demoOpen = ref(false)
const DEMO_NAMES = [
  'علی رضایی', 'سارا محمدی', 'امیرحسین کریمی', 'نگار حسینی', 'مهدی صادقی', 'زهرا احمدی',
  'رضا قاسمی', 'یاسمن نوری', 'حسین رحیمی', 'فاطمه عباسی', 'محمد کاظمی', 'الهام موسوی',
  'سینا جعفری', 'پریا اکبری', 'آرش طاهری', 'مریم یزدانی',
]
function demoRow(rank: number, name: string, correct: number, totalMs: number, score: number): BoardRow {
  const [firstName, ...rest] = name.split(' ')
  return {
    rank, id: 'demo-' + rank, firstName, lastName: rest.join(' '), phone: '', status: 'finished',
    createdAt: 0, startedAt: 0, finishedAt: 0, correct, answered: 3, total: 3, totalMs, score,
  }
}
const DEMO_PODIUM: BoardRow[] = [
  demoRow(1, 'امیرحسین کریمی', 3, 14_300, 375),
  demoRow(2, 'سارا محمدی', 3, 22_800, 361),
  demoRow(3, 'مهدی صادقی', 2, 18_100, 269),
]
const showNames = computed(() => (board.value?.players ?? []).map((r) => `${r.firstName} ${r.lastName}`))
const showTiming = computed(() => {
  const st = board.value?.settings
  return {
    third: st?.revealThirdSec || 6,
    second: st?.revealSecondSec || 9,
    first: st?.revealFirstSec || 15,
    gap: st?.revealGapSec ?? 4,
  }
})
/** Display order on the podium: 2nd, 1st, 3rd. */
/** Every finished player in rank order (top 10) for the announcement table. */
const finishedRanked = computed(() =>
  (board.value?.players ?? []).filter((r) => r.status === 'finished').slice(0, 10),
)
const podiumOrder = computed(() => {
  const p = podium.value
  return [p[1], p[0], p[2]].map((r, i) => ({ r, place: [1, 0, 2][i] })).filter((x) => x.r)
})

/* ---------------- formatting ---------------- */
/** "0936 065 2125" — shown inside an LTR-isolated span so RTL never reorders it. */
function phoneText(p: string): string {
  return /^9\d{9}$/.test(p) ? `0${p.slice(0, 3)} ${p.slice(3, 6)} ${p.slice(6)}` : p
}
/** "HH:MM:SS" wall-clock time of an epoch-ms timestamp. */
function clock(ts: number): string {
  const d = new Date(ts)
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
}
/** "۱ دقیقه و ۵ ثانیه" style duration. */
function duration(ms: number): string {
  const t = Math.max(0, Math.floor(ms / 1000))
  const m = Math.floor(t / 60)
  const sec = t % 60
  return m ? `${toFa(m)} دقیقه و ${toFa(sec)} ثانیه` : `${toFa(sec)} ثانیه`
}

/* ---------------- live players ---------------- */
// server clock, ticked locally between polls so elapsed times run smoothly
const now = ref(Date.now())
let clockOffset = 0
const ticker = window.setInterval(() => (now.value = Date.now() + clockOffset), 1000)
onBeforeUnmount(() => clearInterval(ticker))
watch(board, (b) => {
  if (b) {
    clockOffset = b.serverNow - Date.now()
    now.value = b.serverNow
  }
})
const livePlayers = computed(() => (board.value?.players ?? []).filter((r) => r.status === 'playing'))
const waitingPlayers = computed(() => (board.value?.players ?? []).filter((r) => r.status === 'registered'))

/* ---------------- table ---------------- */
const search = ref('')
const statusFilter = ref<string | undefined>(undefined)
const rows = computed(() => {
  const q = search.value.trim().toLowerCase()
  const qd = digitsOnly(search.value)
  return (board.value?.players ?? []).filter((r) => {
    if (statusFilter.value && r.status !== statusFilter.value) return false
    if (!q) return true
    return `${r.firstName} ${r.lastName}`.toLowerCase().includes(q) || (!!qd && r.phone.includes(qd))
  })
})

/* ---------------- pagination: 5 players per page ---------------- */
const PAGE_SIZE = 5
const page = ref(1)
const livePage = ref(1)
const pageOf = <T,>(list: T[], p: number) => list.slice((p - 1) * PAGE_SIZE, p * PAGE_SIZE)
const pagedRows = computed(() => pageOf(rows.value, page.value))
const pagedLive = computed(() => pageOf(livePlayers.value, livePage.value))
// a new search/filter starts from page 1
watch([search, statusFilter], () => (page.value = 1))
// the live poll can shrink a list — never leave the view on an empty page
watch(
  () => rows.value.length,
  (n) => (page.value = Math.min(page.value, Math.max(1, Math.ceil(n / PAGE_SIZE)))),
)
watch(
  () => livePlayers.value.length,
  (n) => (livePage.value = Math.min(livePage.value, Math.max(1, Math.ceil(n / PAGE_SIZE)))),
)
const tablePagination = computed(() => ({
  current: page.value,
  pageSize: PAGE_SIZE,
  total: rows.value.length,
  hideOnSinglePage: true,
  showSizeChanger: false,
  showTotal: (total: number) => `${toFa(total)} نفر`,
  onChange: (p: number) => (page.value = p),
}))

const columns = [
  { title: 'رتبه', key: 'rank', width: 70 },
  { title: 'نام', key: 'name', width: 160 },
  { title: 'موبایل', key: 'phone', width: 140 },
  { title: 'وضعیت', key: 'status', width: 170 },
  { title: 'جواب درست', key: 'correct', width: 100 },
  { title: 'زمان پاسخ', key: 'time', width: 120 },
  { title: 'امتیاز', key: 'score', width: 80 },
  { title: 'شروع بازی', key: 'started', width: 100 },
  { title: 'پایان بازی', key: 'finished', width: 100 },
  { title: '', key: 'actions', width: 120 },
]

function statusTone(s: string): 'neutral' | 'info' | 'success' {
  return s === 'finished' ? 'success' : s === 'playing' ? 'info' : 'neutral'
}
function statusText(r: BoardRow): string {
  if (r.status === 'finished') return 'پایان بازی'
  if (r.status === 'playing') return `در حال بازی — سوال ${toFa(Math.min(r.answered + 1, r.total))} از ${toFa(r.total)}`
  return 'ثبت‌نام‌شده'
}

/* ---------------- per-player actions ---------------- */
const review = reactive<{ open: boolean; loading: boolean; row: BoardRow | null; items: GameReviewItem[] }>({
  open: false,
  loading: false,
  row: null,
  items: [],
})
async function openReview(r: BoardRow) {
  review.open = true
  review.row = r
  review.items = []
  review.loading = true
  try {
    review.items = (await gamePlayerReview(r.id)).review
  } catch (e) {
    admin.toastError(e, 'دریافت پاسخ‌ها ناموفق بود')
  } finally {
    review.loading = false
  }
}

function confirmReset(r: BoardRow) {
  Modal.confirm({
    title: 'بازی دوباره',
    content: `نتیجه «${r.firstName} ${r.lastName}» پاک می‌شود و می‌تواند دوباره بازی کند.`,
    okText: 'پاک کردن نتیجه',
    cancelText: 'انصراف',
    okType: 'danger',
    async onOk() {
      try {
        await resetGamePlayer(r.id)
        await refresh()
        message.success('نتیجه پاک شد')
      } catch (e) {
        admin.toastError(e, 'انجام نشد')
      }
    },
  })
}
function confirmDelete(r: BoardRow) {
  Modal.confirm({
    title: 'حذف بازیکن',
    content: `«${r.firstName} ${r.lastName}» و نتیجه‌اش برای همیشه حذف می‌شود.`,
    okText: 'حذف',
    cancelText: 'انصراف',
    okType: 'danger',
    async onOk() {
      try {
        await deleteGamePlayer(r.id)
        await refresh()
        message.success('بازیکن حذف شد')
      } catch (e) {
        admin.toastError(e, 'حذف ناموفق بود')
      }
    },
  })
}
function confirmDeleteAll() {
  Modal.confirm({
    title: 'شروع مسابقه جدید',
    content: 'همه بازیکنان و نتایج حذف می‌شوند (بانک سوالات دست نمی‌خورد). این کار برگشت‌پذیر نیست.',
    okText: 'حذف همه',
    cancelText: 'انصراف',
    okType: 'danger',
    async onOk() {
      try {
        const r = await deleteAllGamePlayers()
        await refresh()
        message.success(`${toFa(r.deleted)} بازیکن حذف شد`)
      } catch (e) {
        admin.toastError(e, 'انجام نشد')
      }
    },
  })
}

/* ---------------- full Excel report ---------------- */
const reportLoading = ref(false)
async function exportReport() {
  reportLoading.value = true
  try {
    await downloadGameReport()
    message.success('گزارش کامل Excel دانلود شد')
  } catch (e) {
    admin.toastError(e, 'دریافت گزارش ناموفق بود')
  } finally {
    reportLoading.value = false
  }
}

/** "۳ آسان + ۲ سخت" for the current count */
const composition = computed(() => {
  const plan = difficultyPlan(board.value?.settings.questionsPerGame ?? sform.questionsPerGame)
  const e = plan.filter((d) => d === 'easy').length
  return { easy: e, hard: plan.length - e, order: plan.map((d) => (d === 'easy' ? 'آ' : 'س')).join(' ') }
})

const bankShort = computed(
  () => !!board.value && board.value.questionCount < board.value.settings.questionsPerGame,
)
</script>

<template>
  <div class="stack">
    <a-alert
      v-if="board?.mixedRounds"
      type="warning"
      show-icon
      message="بازیکنان در دورهای متفاوتی بازی کرده‌اند"
      description="تعداد سوال یا زمان هر سوال در وسط مسابقه تغییر کرده است؛ مقایسه جواب‌های درست بین این بازیکنان منصفانه نیست. برای یک مسابقه منصفانه، تنظیمات را قبل از شروع مشخص کنید و اگر لازم شد با «حذف همه نتایج» از نو شروع کنید."
    />

    <!-- start / close the game for everyone -->
    <section class="card card-pad gate" :class="sform.open ? 'is-open' : 'is-closed'">
      <span class="gate-ico"><AppIcon :name="sform.open ? 'play' : 'clock'" :size="26" /></span>
      <div class="grow">
        <b class="gate-title">{{ sform.open ? 'بازی در جریان است' : 'بازی بسته است' }}</b>
        <p class="faint small">
          <template v-if="sform.open">بازیکنان می‌توانند ثبت‌نام کنند و بازی را شروع کنند.</template>
          <template v-else>
            بازیکنان فقط صفحه انتظار با لوگو را می‌بینند. با «شروع بازی»، همه چند ثانیه بعد خودکار وارد می‌شوند.
            <template v-if="waitingPlayers.length"> ({{ toFa(waitingPlayers.length) }} نفر ثبت‌نام‌کرده منتظرند)</template>
          </template>
        </p>
      </div>
      <a-button v-if="!sform.open" type="primary" size="large" class="gate-btn" :loading="savingSettings" @click="confirmOpen(true)">
        <AppIcon name="play" :size="18" /> شروع بازی
      </a-button>
      <a-button v-else danger size="large" class="gate-btn" :loading="savingSettings" @click="confirmOpen(false)">
        <AppIcon name="pause" :size="18" /> بستن بازی
      </a-button>
    </section>

    <!-- number of questions per game -->
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow"><AppIcon name="list" :size="16" /> سوالات مسابقه</span>
      </div>
      <div class="mix-switch">
        <a-switch :checked="mixOn" :loading="savingSettings" @change="toggleMix" />
        <div>
          <b>ترکیب ثابت سوالات بر اساس دسته</b>
          <div class="faint small">
            هر بازیکن دقیقا همین تعداد سوال آسان و سخت از هر دسته می‌گیرد — اول همه آسان‌ها، بعد سخت‌ها، دسته‌ها یکی‌درمیان.
          </div>
        </div>
      </div>

      <template v-if="mixOn">
        <div class="mix-table">
          <div class="mix-head">
            <span>دسته</span><span>آسان</span><span>سخت</span><span>جمع</span><span />
          </div>
          <div v-for="(r, i) in mixDraft" :key="i" class="mix-row">
            <a-select
              v-model:value="r.category"
              :options="categoryOptions"
              placeholder="انتخاب دسته"
              @change="mixDirty = true"
            />
            <div class="mix-cell">
              <a-input-number v-model:value="r.easy" :min="0" :max="50" @change="mixDirty = true" />
              <span v-if="r.category && r.easy > stockOf(r.category).easy" class="mix-warn">
                بانک فقط {{ toFa(stockOf(r.category).easy) }} آسان دارد
              </span>
            </div>
            <div class="mix-cell">
              <a-input-number v-model:value="r.hard" :min="0" :max="50" @change="mixDirty = true" />
              <span v-if="r.category && r.hard > stockOf(r.category).hard" class="mix-warn">
                بانک فقط {{ toFa(stockOf(r.category).hard) }} سخت دارد
              </span>
            </div>
            <b class="num">{{ toFa((r.easy || 0) + (r.hard || 0)) }}</b>
            <a-button size="small" danger :disabled="mixDraft.length < 2" @click="removeMixRow(i)">
              <template #icon><AppIcon name="trash" :size="13" /></template>
            </a-button>
          </div>
        </div>
        <div class="mix-actions">
          <a-button size="small" :disabled="categoryOptions.every((o) => o.disabled)" @click="addMixRow">
            <template #icon><AppIcon name="plus" :size="13" /></template>
            افزودن دسته
          </a-button>
          <span class="grow" />
          <span>جمع هر بازی: <b class="num qc-current">{{ toFa(mixTotal) }} سوال</b></span>
          <a-button v-if="mixDirty" size="small" @click="cancelMix">انصراف</a-button>
          <a-button type="primary" size="small" :disabled="!mixDirty" :loading="savingSettings" @click="saveMix()">
            ذخیره ترکیب
          </a-button>
        </div>
        <ol class="mix-order">
          <li v-for="(sl, i) in mixOrder" :key="i" :class="sl.difficulty">
            {{ toFa(i + 1) }}. {{ sl.category }} · {{ DIFFICULTY_LABEL[sl.difficulty] }}
          </li>
        </ol>
      </template>

      <template v-else>
      <div class="qc-label">
        <span>تعداد سوال هر بازی</span>
        <span class="faint small">
          فعلی: <b class="num qc-current">{{ toFa(board?.settings.questionsPerGame ?? sform.questionsPerGame) }} سوال</b>
        </span>
      </div>
      <div class="qc-row">
        <button
          v-for="n in QC_PRESETS"
          :key="n"
          type="button"
          class="qc-btn"
          :class="{ on: (board?.settings.questionsPerGame ?? sform.questionsPerGame) === n }"
          :disabled="savingSettings || n > (board?.questionCount ?? 0)"
          @click="setQuestionCount(n)"
        >
          <b class="num">{{ toFa(n) }}</b>
          <small>سوال</small>
        </button>
        <div class="qc-custom">
          <a-input-number v-model:value="qcCustom" :min="1" :max="50" placeholder="تعداد دلخواه" style="width: 130px" />
          <a-button :disabled="!qcCustom || savingSettings" :loading="savingSettings" @click="setQuestionCount(qcCustom!)">
            اعمال
          </a-button>
        </div>
      </div>

      <p class="qc-mix">
        <AppIcon name="list" :size="13" />
        هر بازی: <b>{{ toFa(composition.easy) }} سوال آسان + {{ toFa(composition.hard) }} سوال سخت</b>
        <span class="faint">— به این ترتیب: {{ composition.order }}</span>
        <span class="faint">(بانک: {{ toFa(board?.easyCount ?? 0) }} آسان، {{ toFa(board?.hardCount ?? 0) }} سخت)</span>
      </p>
      </template>

      <div class="qc-label qc-gap">
        <span>زمان پاسخ به هر سوال</span>
        <span class="faint small">
          فعلی: <b class="num qc-current">{{ toFa(board?.settings.secondsPerQuestion ?? sform.secondsPerQuestion) }} ثانیه</b>
        </span>
      </div>
      <div class="qc-row">
        <button
          v-for="n in QT_PRESETS"
          :key="n"
          type="button"
          class="qc-btn"
          :class="{ on: (board?.settings.secondsPerQuestion ?? sform.secondsPerQuestion) === n }"
          :disabled="savingSettings"
          @click="setQuestionTime(n)"
        >
          <b class="num">{{ toFa(n) }}</b>
          <small>ثانیه</small>
        </button>
        <div class="qc-custom">
          <a-input-number v-model:value="qtCustom" :min="5" :max="300" placeholder="ثانیه دلخواه" style="width: 130px" />
          <a-button :disabled="!qtCustom || savingSettings" :loading="savingSettings" @click="setQuestionTime(qtCustom!)">
            اعمال
          </a-button>
        </div>
      </div>

      <p class="faint small qc-note">
        هر بازیکن این تعداد سوال تصادفی از بانک ({{ toFa(board?.questionCount ?? 0) }} سوال) می‌گیرد.
        تغییر تعداد یا زمان فقط روی بازی‌هایی اثر دارد که از این به بعد شروع می‌شوند؛ کسی که الان در حال بازی است با همان تنظیم قبلی ادامه می‌دهد.
        <template v-if="board">
          مدت هر بازی حداکثر {{ toFa(Math.ceil((board.settings.questionsPerGame * board.settings.secondsPerQuestion) / 60)) }} دقیقه است.
        </template>
      </p>
    </section>

    <!-- settings -->
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow">تنظیمات بازی</span>
        <span class="open-toggle">
          <span :class="sform.open ? 'ok-text' : 'faint'">{{ sform.open ? 'بازی باز است' : 'بازی بسته است' }}</span>
          <a-switch :checked="sform.open" :loading="savingSettings" @change="toggleOpen" />
        </span>
      </div>
      <div class="settings-grid">
        <div class="test-box wide">
          <div class="grow">
            <b><AppIcon name="rotate-ccw" :size="14" /> حالت تست</b>
            <p class="faint small">
              وقتی روشن است، هر بازیکن در صفحه نتیجه دکمه «بازی دوباره» دارد و می‌تواند هر چند بار بخواهد بازی کند.
              پیش از مسابقه اصلی خاموشش کنید و با «حذف همه نتایج» جدول را خالی کنید.
            </p>
          </div>
          <a-switch :checked="sform.testMode" :loading="savingSettings" @change="toggleTestMode" />
        </div>
        <label class="f wide">
          <span>مدت نمایش نتیجه به بازیکن پیش از صفحه شبکه‌های اجتماعی (ثانیه)</span>
          <a-input-number v-model:value="sform.socialDelaySec" :min="2" :max="120" addon-after="ثانیه" style="width: 220px" @change="markDirty" />
        </label>
        <label class="f wide">
          <span>پیام پایان بازی (به بازیکن نمایش داده می‌شود)</span>
          <a-textarea v-model:value="sform.finishMessage" :rows="2" :maxlength="500" @change="markDirty" />
        </label>
      </div>
      <div class="settings-foot">
        <span class="faint small">
          بانک سوالات: <b class="num">{{ toFa(board?.questionCount ?? 0) }}</b> سوال
          <template v-if="bankShort"> — <span class="warn-text">کمتر از تعداد سوال هر بازی است</span></template>
        </span>
        <div class="grow" />
        <a-button v-if="settingsDirty" @click="settingsDirty = false; syncSettingsForm()">انصراف</a-button>
        <a-button type="primary" :loading="savingSettings" :disabled="!settingsDirty" @click="saveSettings()">
          ذخیره تنظیمات
        </a-button>
      </div>
    </section>

    <!-- counters -->
    <div class="stat-grid">
      <StatCard icon="users" label="کل بازیکنان" :value="board?.stats.total ?? 0" tone="primary" sub="همه ثبت‌نام‌شده‌ها در بازی" />
      <StatCard icon="gamepad" label="در حال بازی" :value="board?.stats.playing ?? 0" tone="info" sub="همین الان مشغول پاسخ دادن" />
      <StatCard icon="check-circle" label="بازی تمام‌شده" :value="board?.stats.finished ?? 0" tone="success" sub="نتیجه نهایی ثبت شده" />
      <StatCard
        icon="target"
        label="میانگین امتیاز"
        :value="Math.round(board?.stats.avgScore ?? 0)"
        tone="accent"
        :sub="`میانگین جواب درست: ${toFa((board?.stats.avgCorrect ?? 0).toFixed(1))}`"
      />
    </div>

    <!-- announcement timing -->
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow"><AppIcon name="clock" :size="16" /> زمان‌بندی اعلام نتایج</span>
        <a-button v-if="settingsDirty" size="small" @click="settingsDirty = false; syncSettingsForm()">انصراف</a-button>
        <a-button type="primary" size="small" :loading="savingSettings" :disabled="!settingsDirty" @click="saveSettings()">
          ذخیره
        </a-button>
      </div>
      <p class="faint small rule">
        برای هر نفر، اسامی به مدتی که اینجا تعیین می‌کنید می‌چرخند و بعد روی «؟» می‌ایستند؛
        با زدن Enter (یا لمس کادر روی موبایل) نام اعلام می‌شود.
      </p>
      <div class="reveal-grid">
        <label class="rf">
          <span>🥉 زمان تا اعلام نفر سوم</span>
          <a-input-number v-model:value="sform.revealThirdSec" :min="2" :max="120" addon-after="ثانیه" style="width: 100%" @change="markDirty" />
        </label>
        <label class="rf">
          <span>🥈 زمان تا اعلام نفر دوم</span>
          <a-input-number v-model:value="sform.revealSecondSec" :min="2" :max="120" addon-after="ثانیه" style="width: 100%" @change="markDirty" />
        </label>
        <label class="rf">
          <span>🥇 زمان تا اعلام نفر اول</span>
          <a-input-number v-model:value="sform.revealFirstSec" :min="2" :max="120" addon-after="ثانیه" style="width: 100%" @change="markDirty" />
        </label>
        <label class="rf">
          <span>⏸ مکث بعد از هر معرفی</span>
          <a-input-number v-model:value="sform.revealGapSec" :min="0" :max="60" addon-after="ثانیه" style="width: 100%" @change="markDirty" />
        </label>
      </div>
    </section>

    <!-- podium -->
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow"><AppIcon name="crown" :size="16" /> نفرات برتر (محاسبه خودکار)</span>
        <a-button :disabled="!podium.length" @click="announceOpen = true">
          <template #icon><AppIcon name="list" :size="14" /></template>
          جدول نتایج
        </a-button>
        <a-button @click="demoOpen = true">
          <template #icon><AppIcon name="play" :size="14" /></template>
          دمو اعلام نتایج
        </a-button>
        <a-button type="primary" :disabled="!podium.length" @click="showOpen = true">
          <template #icon><AppIcon name="trophy" :size="14" /></template>
          اعلام نتایج (نمایش زنده)
        </a-button>
      </div>
      <p class="faint small rule">
        ملاک رتبه: ۱) تعداد جواب درست — بیشتر بهتر؛ ۲) در تساوی، مجموع زمان پاسخ — سریع‌تر بهتر.
        امتیاز = ۱۰۰ برای هر جواب درست + تا ۹۹ امتیاز سرعت.
      </p>
      <div v-if="podium.length" class="podium-list">
        <div v-for="(r, i) in podium" :key="r.id" class="podium-item" :class="'p' + (i + 1)">
          <span class="medal">{{ MEDALS[i] }}</span>
          <div class="grow">
            <div class="p-name">{{ r.firstName }} {{ r.lastName }}</div>
            <div class="p-metrics">
              <span class="metric"><AppIcon name="check-circle" :size="13" /> {{ toFa(r.correct) }} از {{ toFa(r.total) }} درست</span>
              <span class="metric"><AppIcon name="clock" :size="13" /> {{ fmtSecs(r.totalMs) }} ثانیه</span>
            </div>
          </div>
          <span class="p-score"><b class="num">{{ toFa(r.score) }}</b><small>امتیاز</small></span>
        </div>
      </div>
      <div v-else class="empty">هنوز کسی بازی را تمام نکرده است.</div>
    </section>

    <!-- playing right now -->
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow"><i class="live-dot" /> شرکت‌کنندگانِ در حال بازی (زنده)</span>
        <Chip tone="info">{{ toFa(livePlayers.length) }} نفر در حال بازی</Chip>
        <Chip>{{ toFa(waitingPlayers.length) }} نفر در انتظار شروع</Chip>
      </div>
      <div v-if="livePlayers.length" class="live-grid">
        <div v-for="r in pagedLive" :key="r.id" class="live-item">
          <div class="live-top">
            <b>{{ r.firstName }} {{ r.lastName }}</b>
            <span class="phone">{{ phoneText(r.phone) }}</span>
          </div>
          <div class="live-progress">
            <span
              v-for="i in r.total"
              :key="i"
              class="lp-seg"
              :class="{ done: i <= r.answered, cur: i === r.answered + 1 }"
            />
          </div>
          <div class="live-meta">
            <span>سوال {{ toFa(Math.min(r.answered + 1, r.total)) }} از {{ toFa(r.total) }}</span>
            <span>{{ toFa(r.correct) }} درست تا الان</span>
          </div>
          <div class="live-meta faint">
            <span>شروع: <span class="ltr-time">{{ toFa(clock(r.startedAt)) }}</span></span>
            <span>{{ duration(now - r.startedAt) }} گذشته</span>
          </div>
        </div>
      </div>
      <a-pagination
        v-if="livePlayers.length > PAGE_SIZE"
        v-model:current="livePage"
        class="pager"
        :total="livePlayers.length"
        :page-size="PAGE_SIZE"
        :show-size-changer="false"
        size="small"
      />
      <div v-if="!livePlayers.length" class="empty">الان کسی در حال بازی نیست.</div>
      <div v-if="waitingPlayers.length" class="waiting">
        <span class="faint small">ثبت‌نام کرده و هنوز شروع نکرده:</span>
        <Chip v-for="r in waitingPlayers" :key="r.id">{{ r.firstName }} {{ r.lastName }}</Chip>
      </div>
    </section>

    <!-- all players -->
    <section class="card card-pad">
      <div class="toolbar">
        <a-input v-model:value="search" placeholder="جست‌وجو: نام یا موبایل" allow-clear style="max-width: 280px">
          <template #prefix><AppIcon name="search" :size="15" /></template>
        </a-input>
        <a-select
          v-model:value="statusFilter"
          placeholder="همه وضعیت‌ها"
          allow-clear
          style="width: 160px"
          :options="[
            { value: 'finished', label: 'پایان بازی' },
            { value: 'playing', label: 'در حال بازی' },
            { value: 'registered', label: 'ثبت‌نام‌شده' },
          ]"
        />
        <div class="grow" />
        <span class="faint small">{{ toFa(rows.length) }} نفر</span>
        <a-button size="small" type="primary" :disabled="!board?.players.length" :loading="reportLoading" @click="exportReport">
          <template #icon><AppIcon name="download" :size="13" /></template>
          گزارش کامل Excel
        </a-button>
        <a-button size="small" danger :disabled="!board?.players.length" @click="confirmDeleteAll">
          <template #icon><AppIcon name="trash" :size="13" /></template>
          حذف همه نتایج
        </a-button>
      </div>

      <!-- phones: one card per player instead of the wide table -->
      <div class="m-list">
        <div v-for="r in pagedRows" :key="r.id" class="m-row" :class="{ top: r.rank && r.rank <= 3 && r.correct > 0 }">
          <span class="m-rank">
            <template v-if="r.rank && r.rank <= 3 && r.correct > 0">{{ MEDALS[r.rank - 1] }}</template>
            <template v-else-if="r.rank">{{ toFa(r.rank) }}</template>
            <template v-else>—</template>
          </span>
          <div class="m-body">
            <div class="m-name">{{ r.firstName }} {{ r.lastName }}</div>
            <div class="m-sub">
              <span class="phone">{{ phoneText(r.phone) }}</span>
              <Chip :tone="statusTone(r.status)" dot>{{ statusText(r) }}</Chip>
            </div>
            <div v-if="r.status !== 'registered'" class="m-metrics">
              <span><AppIcon name="check-circle" :size="12" /> {{ toFa(r.correct) }} از {{ toFa(r.total) }}</span>
              <span><AppIcon name="clock" :size="12" /> {{ fmtSecs(r.totalMs) }} ثانیه</span>
              <span v-if="r.startedAt" class="faint">شروع <span class="ltr-time">{{ toFa(clock(r.startedAt)) }}</span></span>
            </div>
          </div>
          <div class="m-side">
            <b v-if="r.status === 'finished'" class="m-score num">{{ toFa(r.score) }}</b>
            <small v-if="r.status === 'finished'" class="faint">امتیاز</small>
            <div class="m-actions">
              <a-button size="small" :disabled="r.status === 'registered'" @click="openReview(r)">
                <template #icon><AppIcon name="eye" :size="13" /></template>
              </a-button>
              <a-button size="small" :disabled="r.status === 'registered'" @click="confirmReset(r)">
                <template #icon><AppIcon name="rotate-ccw" :size="13" /></template>
              </a-button>
              <a-button size="small" danger @click="confirmDelete(r)">
                <template #icon><AppIcon name="trash" :size="13" /></template>
              </a-button>
            </div>
          </div>
        </div>
        <a-pagination
          v-if="rows.length > PAGE_SIZE"
          v-model:current="page"
          class="pager"
          :total="rows.length"
          :page-size="PAGE_SIZE"
          :show-size-changer="false"
          size="small"
        />
        <div v-if="!rows.length" class="empty">هنوز بازیکنی ثبت‌نام نکرده است.</div>
      </div>

      <a-table
        class="d-table"
        :columns="columns"
        :data-source="rows"
        row-key="id"
        size="small"
        :pagination="tablePagination"
        :scroll="{ x: 1200 }"
        :row-class-name="(r: BoardRow) => (r.rank && r.rank <= 3 ? 'top-row' : '')"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'rank'">
            <span v-if="record.rank && record.rank <= 3" class="medal-sm">{{ MEDALS[record.rank - 1] }}</span>
            <span v-else-if="record.rank" class="num">{{ toFa(record.rank) }}</span>
            <span v-else class="faint">—</span>
          </template>
          <template v-else-if="column.key === 'name'">{{ record.firstName }} {{ record.lastName }}</template>
          <template v-else-if="column.key === 'phone'"><span class="phone">{{ phoneText(record.phone) }}</span></template>
          <template v-else-if="column.key === 'status'">
            <Chip :tone="statusTone(record.status)" dot>{{ statusText(record as BoardRow) }}</Chip>
          </template>
          <template v-else-if="column.key === 'correct'">
            <span v-if="record.status !== 'registered'" class="num">{{ toFa(record.correct) }} از {{ toFa(record.total) }}</span>
            <span v-else class="faint">—</span>
          </template>
          <template v-else-if="column.key === 'time'">
            <span v-if="record.answered" class="num">{{ fmtSecs(record.totalMs) }} ثانیه</span>
            <span v-else class="faint">—</span>
          </template>
          <template v-else-if="column.key === 'score'">
            <b v-if="record.status === 'finished'" class="num">{{ toFa(record.score) }}</b>
            <span v-else class="faint">—</span>
          </template>
          <template v-else-if="column.key === 'started'">
            <span v-if="record.startedAt" class="ltr-time">{{ toFa(clock(record.startedAt)) }}</span>
            <span v-else class="faint">—</span>
          </template>
          <template v-else-if="column.key === 'finished'">
            <span v-if="record.finishedAt" class="ltr-time">{{ toFa(clock(record.finishedAt)) }}</span>
            <span v-else class="faint">—</span>
          </template>
          <template v-else-if="column.key === 'actions'">
            <div class="row-actions">
              <a-tooltip title="مشاهده پاسخ‌ها">
                <a-button size="small" :disabled="record.status === 'registered'" @click="openReview(record as BoardRow)">
                  <template #icon><AppIcon name="eye" :size="13" /></template>
                </a-button>
              </a-tooltip>
              <a-tooltip title="پاک کردن نتیجه و بازی دوباره">
                <a-button size="small" :disabled="record.status === 'registered'" @click="confirmReset(record as BoardRow)">
                  <template #icon><AppIcon name="rotate-ccw" :size="13" /></template>
                </a-button>
              </a-tooltip>
              <a-tooltip title="حذف">
                <a-button size="small" danger @click="confirmDelete(record as BoardRow)">
                  <template #icon><AppIcon name="trash" :size="13" /></template>
                </a-button>
              </a-tooltip>
            </div>
          </template>
        </template>
        <template #emptyText>
          <div class="empty">هنوز بازیکنی ثبت‌نام نکرده است.</div>
        </template>
      </a-table>
    </section>

    <!-- player answers -->
    <a-modal
      v-model:open="review.open"
      :title="review.row ? `پاسخ‌های ${review.row.firstName} ${review.row.lastName}` : ''"
      :footer="null"
      width="640px"
    >
      <a-spin :spinning="review.loading">
        <div class="review-list">
          <div v-for="(it, i) in review.items" :key="i" class="review-item">
            <div class="review-hd">
              <b>سوال {{ toFa(i + 1) }}</b>
              <Chip v-if="it.choice === -2" tone="info">در انتظار پاسخ</Chip>
              <Chip v-else-if="it.isCorrect" tone="success">درست، {{ fmtSecs(it.ms ?? 0) }} ث</Chip>
              <Chip v-else-if="it.choice === -1" tone="warning">وقت تمام شد</Chip>
              <Chip v-else tone="danger">اشتباه، {{ fmtSecs(it.ms ?? 0) }} ث</Chip>
            </div>
            <p class="review-q">{{ it.text }}</p>
            <ul class="review-opts">
              <li
                v-for="(o, oi) in it.options"
                :key="oi"
                :class="{ correct: oi === it.correct, wrong: oi === it.choice && !it.isCorrect }"
              >
                {{ o }}
              </li>
            </ul>
          </div>
        </div>
      </a-spin>
    </a-modal>

    <WinnerReveal
      v-if="demoOpen"
      demo
      :podium="DEMO_PODIUM"
      :names="DEMO_NAMES"
      :timing="showTiming"
      @close="demoOpen = false"
    />
    <WinnerReveal
      v-if="showOpen"
      :podium="podium"
      :names="showNames"
      :timing="showTiming"
      :report-loading="reportLoading"
      @close="showOpen = false"
      @report="exportReport"
    />

    <!-- winners announcement (big, for a projector) -->
    <a-modal v-model:open="announceOpen" :footer="null" width="860px" centered :title="null" class="announce-modal">
      <div class="announce">
        <h2><AppIcon name="trophy" :size="26" /> برندگان لیگ قهرمانان برنامه‌نویسی</h2>
        <p class="announce-rule">
          رتبه‌بندی بر اساس <b>تعداد جواب درست</b> و سپس <b>سرعت پاسخ‌گویی</b> (زمان کمتر) انجام شده است
        </p>
        <div class="stage">
          <div v-for="x in podiumOrder" :key="x.r!.id" class="step" :class="'s' + (x.place + 1)">
            <span class="step-medal">{{ MEDALS[x.place] }}</span>
            <span class="step-name">{{ x.r!.firstName }} {{ x.r!.lastName }}</span>
            <div class="step-stats">
              <span class="sstat">
                <b class="num">{{ toFa(x.r!.correct) }} از {{ toFa(x.r!.total) }}</b>
                <small>جواب درست</small>
              </span>
              <span class="sstat">
                <b class="num">{{ fmtSecs(x.r!.totalMs) }}</b>
                <small>ثانیه</small>
              </span>
              <span class="sstat score">
                <b class="num">{{ toFa(x.r!.score) }}</b>
                <small>امتیاز</small>
              </span>
            </div>
            <div class="step-block">{{ PLACE[x.place] }}</div>
          </div>
        </div>
        <table v-if="finishedRanked.length" class="announce-table">
          <thead>
            <tr><th>رتبه</th><th>نام</th><th>جواب درست</th><th>زمان پاسخ (ثانیه)</th><th>امتیاز</th></tr>
          </thead>
          <tbody>
            <tr v-for="r in finishedRanked" :key="r.id" :class="{ top: r.rank <= 3 && r.correct > 0 }">
              <td>{{ r.rank <= 3 && r.correct > 0 ? MEDALS[r.rank - 1] : toFa(r.rank) }}</td>
              <td>{{ r.firstName }} {{ r.lastName }}</td>
              <td class="num">{{ toFa(r.correct) }} از {{ toFa(r.total) }}</td>
              <td class="num">{{ fmtSecs(r.totalMs) }}</td>
              <td class="num"><b>{{ toFa(r.score) }}</b></td>
            </tr>
          </tbody>
        </table>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
.card-hd .section-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.gate {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  border-inline-start: 4px solid var(--warning, #f68b1f);
}
.gate.is-open {
  border-inline-start-color: var(--success);
}
.gate-ico {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--warning, #f68b1f) 15%, transparent);
  color: var(--warning, #f68b1f);
}
.gate.is-open .gate-ico {
  background: color-mix(in srgb, var(--success) 15%, transparent);
  color: var(--success);
}
.gate-title {
  font-size: 1.05rem;
}
.gate .faint {
  margin: 4px 0 0;
}
.gate-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 150px;
  justify-content: center;
}
.open-toggle {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 0.82rem;
  font-weight: 600;
}
.ok-text {
  color: var(--success);
}
.warn-text {
  color: var(--warning);
  font-weight: 600;
}
.small {
  font-size: 0.78rem;
}
.settings-grid {
  display: grid;
  gap: var(--space-3);
  grid-template-columns: 1fr 1fr;
}
.settings-grid .wide {
  grid-column: 1 / -1;
}
.mix-switch {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: var(--space-4);
}
.mix-table {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.mix-head,
.mix-row {
  display: grid;
  grid-template-columns: minmax(160px, 2fr) 1fr 1fr 48px 36px;
  gap: 8px;
  align-items: start;
}
.mix-head {
  font-size: 0.78rem;
  font-weight: 700;
  color: var(--text-dim);
}
.mix-row > b {
  padding-top: 5px;
  text-align: center;
}
.mix-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.mix-cell :deep(.ant-input-number) {
  width: 100%;
}
.mix-warn {
  font-size: 0.72rem;
  color: var(--warning);
}
.mix-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 10px;
  font-size: 0.85rem;
}
.mix-order {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 12px 0 0;
  padding: 0;
  list-style: none;
}
.mix-order li {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 0.75rem;
  background: var(--success-soft);
  color: var(--success);
}
.mix-order li.hard {
  background: var(--warning-soft);
  color: var(--warning);
}
@media (max-width: 700px) {
  .mix-head {
    display: none;
  }
  .mix-row {
    grid-template-columns: 1fr 1fr;
  }
  .mix-row > .ant-select {
    grid-column: 1 / -1;
  }
}
.qc-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.qc-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 72px;
  padding: 8px 12px;
  border-radius: var(--radius);
  border: 1px solid var(--border-strong);
  background: var(--surface-2);
  color: var(--text);
  line-height: 1.3;
  transition: border-color var(--dur) var(--ease), background var(--dur) var(--ease);
}
.qc-btn b {
  font-size: 1.4rem;
  font-weight: 800;
}
.qc-btn small {
  font-size: 0.72rem;
  color: var(--text-dim);
}
.qc-btn:hover:not(:disabled) {
  border-color: var(--primary-bright);
}
.qc-btn.on {
  border-color: var(--primary-bright);
  background: var(--primary-soft);
  box-shadow: 0 0 0 1px var(--primary-bright);
}
.qc-btn.on b {
  color: var(--primary-bright);
}
.qc-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.qc-custom {
  display: flex;
  gap: 6px;
  margin-inline-start: auto;
}
.qc-current {
  color: var(--primary-bright);
}
.qc-mix {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin: 10px 0 0;
  font-size: 0.8rem;
}
.qc-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-dim);
}
.qc-gap {
  margin-top: var(--space-4);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border);
}
.qc-note {
  margin: var(--space-3) 0 0;
}
@media (max-width: 700px) {
  .qc-btn {
    flex: 1;
    min-width: 0;
  }
  .qc-custom {
    width: 100%;
    margin-inline-start: 0;
  }
}
.test-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  border-radius: var(--radius);
  border: 1px dashed color-mix(in srgb, var(--warning) 60%, transparent);
  background: var(--warning-soft);
}
.test-box b {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.test-box p {
  margin: 4px 0 0;
}
.pager {
  display: flex;
  justify-content: center;
  margin-top: var(--space-3);
}
.m-list {
  display: none;
}
@media (max-width: 700px) {
  .d-table {
    display: none;
  }
  .m-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .toolbar :deep(.ant-input-affix-wrapper),
  .toolbar :deep(.ant-select) {
    max-width: none !important;
    width: 100% !important;
  }
}
.m-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--surface-2);
}
.m-row.top {
  border-color: color-mix(in srgb, #f59e0b 55%, transparent);
  background: color-mix(in srgb, #f59e0b 8%, var(--surface));
}
.m-rank {
  width: 28px;
  flex: none;
  text-align: center;
  font-size: 1.3rem;
  font-weight: 800;
  line-height: 1.6;
}
.m-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.m-name {
  font-weight: 700;
}
.m-sub,
.m-metrics {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  font-size: 0.78rem;
  color: var(--text-dim);
}
.m-metrics span {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}
.m-side {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.m-score {
  font-size: 1.3rem;
  color: var(--primary-bright);
  line-height: 1.2;
}
.m-actions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
}
.reveal-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-2);
}
.rf {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.rf > span {
  font-size: 0.8rem;
  color: var(--text-dim);
  font-weight: 600;
}
@media (max-width: 700px) {
  .reveal-grid {
    grid-template-columns: 1fr 1fr;
  }
}
.settings-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: var(--space-4);
}
.f {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.f > span {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-dim);
}
.stat-grid {
  display: grid;
  gap: var(--space-3);
  grid-template-columns: repeat(2, 1fr);
}
@media (min-width: 900px) {
  .stat-grid {
    grid-template-columns: repeat(4, 1fr);
  }
}
.rule {
  margin-top: calc(-1 * var(--space-2));
}
.podium-list {
  display: grid;
  gap: var(--space-2);
}
@media (min-width: 900px) {
  .podium-list {
    grid-template-columns: repeat(3, 1fr);
  }
}
.podium-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--surface-2);
}
.podium-item.p1 {
  border-color: color-mix(in srgb, #f59e0b 55%, transparent);
  background: color-mix(in srgb, #f59e0b 12%, var(--surface));
}
.podium-item.p2 {
  border-color: color-mix(in srgb, #94a3b8 60%, transparent);
}
.podium-item.p3 {
  border-color: color-mix(in srgb, #b45309 45%, transparent);
}
.medal {
  font-size: 1.9rem;
  line-height: 1;
}
.p-name {
  font-weight: 700;
}
.p-score {
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1.2;
}
.p-score b {
  font-size: 1.4rem;
  font-weight: 800;
  color: var(--primary-bright);
}
.p-score small {
  font-size: 0.7rem;
  color: var(--text-faint);
}
.p-metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 4px;
}
.metric {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: var(--surface);
  border: 1px solid var(--border);
  font-size: 0.75rem;
  color: var(--text-dim);
}
/* phone numbers / clock times are LTR runs inside RTL text */
.phone,
.ltr-time {
  direction: ltr;
  unicode-bidi: isolate;
  display: inline-block;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.live-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--success);
  box-shadow: 0 0 0 0 color-mix(in srgb, var(--success) 60%, transparent);
  animation: live-pulse 1.8s infinite;
}
@keyframes live-pulse {
  70% {
    box-shadow: 0 0 0 7px transparent;
  }
}
.live-grid {
  display: grid;
  gap: var(--space-3);
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
}
.live-item {
  padding: 12px 14px;
  border-radius: var(--radius);
  border: 1px solid color-mix(in srgb, var(--info) 40%, var(--border));
  background: var(--info-soft);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.live-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.live-top .phone {
  font-size: 0.78rem;
  color: var(--text-dim);
}
.live-progress {
  display: flex;
  gap: 4px;
}
.lp-seg {
  flex: 1;
  height: 6px;
  border-radius: 3px;
  background: var(--surface-3);
}
.lp-seg.done {
  background: var(--info);
}
.lp-seg.cur {
  background: color-mix(in srgb, var(--info) 45%, transparent);
  animation: seg-blink 1s ease-in-out infinite;
}
@keyframes seg-blink {
  50% {
    opacity: 0.4;
  }
}
.live-meta {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 0.78rem;
}
.waiting {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: var(--space-3);
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-bottom: var(--space-4);
}
.row-actions {
  display: flex;
  gap: 6px;
}
.medal-sm {
  font-size: 1.25rem;
}
:deep(.top-row td) {
  background: color-mix(in srgb, #f59e0b 6%, transparent);
}
.empty {
  padding: 24px;
  color: var(--text-faint);
  text-align: center;
}
.review-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 60px;
}
.review-item {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
}
.review-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.review-q {
  font-weight: 600;
  margin-bottom: 8px;
}
.review-opts {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 6px;
  grid-template-columns: 1fr 1fr;
}
.review-opts li {
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  font-size: 0.85rem;
}
.review-opts li.correct {
  background: var(--success-soft);
  color: var(--success);
  font-weight: 700;
  outline: 1px solid var(--success);
}
.review-opts li.wrong {
  background: var(--danger-soft);
  color: var(--danger);
  font-weight: 700;
  outline: 1px solid var(--danger);
}

/* ---- announcement ---- */
.announce {
  text-align: center;
  padding: 8px 0 0;
}
.announce h2 {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 1.5rem;
  font-weight: 900;
  margin-bottom: 24px;
}
.stage {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 14px;
}
.step {
  flex: 1;
  max-width: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  animation: rise 0.6s cubic-bezier(0.22, 1, 0.36, 1) both;
}
.step.s1 {
  animation-delay: 0.5s;
}
.step.s2 {
  animation-delay: 0.25s;
}
.step-medal {
  font-size: 3rem;
  line-height: 1.1;
}
.step.s1 .step-medal {
  font-size: 4rem;
}
.step-name {
  font-weight: 800;
  font-size: 1.1rem;
}
.step.s1 .step-name {
  font-size: 1.3rem;
}
.announce-rule {
  margin: -14px 0 22px;
  color: var(--text-dim);
}
.step-stats {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}
.sstat {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 58px;
  padding: 4px 8px;
  border-radius: 10px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  line-height: 1.3;
}
.sstat b {
  font-size: 1.05rem;
  font-weight: 800;
}
.sstat small {
  font-size: 0.68rem;
  color: var(--text-faint);
}
.sstat.score b {
  color: var(--primary-bright);
}
.announce-table {
  width: 100%;
  margin-top: 22px;
  border-collapse: collapse;
  font-size: 0.9rem;
}
.announce-table th {
  padding: 8px;
  background: var(--surface-2);
  color: var(--text-dim);
  font-weight: 700;
  text-align: center;
}
.announce-table td {
  padding: 8px;
  border-bottom: 1px solid var(--border);
  text-align: center;
}
.announce-table tr.top td {
  background: color-mix(in srgb, #f59e0b 8%, transparent);
  font-weight: 700;
}
.step-meta {
  font-size: 0.85rem;
}
.step-block {
  width: 100%;
  margin-top: 10px;
  display: grid;
  place-items: center;
  border-radius: 14px 14px 0 0;
  color: #fff;
  font-weight: 800;
  font-size: 1rem;
}
.step.s1 .step-block {
  height: 150px;
  background: linear-gradient(180deg, #fbbf24, #d97706);
}
.step.s2 .step-block {
  height: 110px;
  background: linear-gradient(180deg, #cbd5e1, #64748b);
}
.step.s3 .step-block {
  height: 80px;
  background: linear-gradient(180deg, #f59e0b, #92400e);
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@media (max-width: 600px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
  .review-opts {
    grid-template-columns: 1fr;
  }
  .stage {
    gap: 6px;
  }
  .step-name {
    font-size: 0.9rem;
  }
  .step.s1 .step-name {
    font-size: 1rem;
  }
}
</style>
