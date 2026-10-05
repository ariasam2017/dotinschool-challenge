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
    admin.toastError(e, 'ذخیره‌ی تنظیمات ناموفق بود')
  } finally {
    savingSettings.value = false
  }
}
function toggleOpen(v: boolean | string | number) {
  void saveSettings({ open: !!v })
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
    content: `نتیجه‌ی «${r.firstName} ${r.lastName}» پاک می‌شود و می‌تواند دوباره بازی کند.`,
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
    title: 'شروع مسابقه‌ی جدید',
    content: 'همه‌ی بازیکنان و نتایج حذف می‌شوند (بانک سوالات دست نمی‌خورد). این کار برگشت‌پذیر نیست.',
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

/* ---------------- CSV export ---------------- */
function exportCsv() {
  const list = board.value?.players ?? []
  const esc = (v: string | number) => `"${String(v).replace(/"/g, '""')}"`
  const head = ['رتبه', 'نام', 'نام خانوادگی', 'موبایل', 'وضعیت', 'جواب درست', 'تعداد سوال', 'زمان پاسخ (ثانیه)', 'امتیاز']
  const lines = list.map((r) =>
    [
      r.rank || '',
      r.firstName,
      r.lastName,
      '0' + r.phone,
      r.status === 'finished' ? 'پایان' : r.status === 'playing' ? 'در حال بازی' : 'ثبت‌نام',
      r.correct,
      r.total,
      (r.totalMs / 1000).toFixed(1),
      r.score,
    ]
      .map(esc)
      .join(','),
  )
  const blob = new Blob(['﻿' + [head.map(esc).join(','), ...lines].join('\r\n')], {
    type: 'text/csv;charset=utf-8',
  })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `game-results-${new Date().toISOString().slice(0, 10)}.csv`
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 2000)
}

const bankShort = computed(
  () => !!board.value && board.value.questionCount < board.value.settings.questionsPerGame,
)
</script>

<template>
  <div class="stack">
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
        <label class="f">
          <span>تعداد سوال هر بازی</span>
          <a-input-number v-model:value="sform.questionsPerGame" :min="1" :max="50" style="width: 100%" @change="markDirty" />
        </label>
        <label class="f">
          <span>زمان هر سوال (ثانیه)</span>
          <a-input-number v-model:value="sform.secondsPerQuestion" :min="5" :max="300" style="width: 100%" @change="markDirty" />
        </label>
        <div class="test-box wide">
          <div class="grow">
            <b><AppIcon name="rotate-ccw" :size="14" /> حالت تست</b>
            <p class="faint small">
              وقتی روشن است، هر بازیکن در صفحه‌ی نتیجه دکمه‌ی «بازی دوباره» دارد و می‌تواند هر چند بار بخواهد بازی کند.
              پیش از مسابقه‌ی اصلی خاموشش کنید و با «حذف همه‌ی نتایج» جدول را خالی کنید.
            </p>
          </div>
          <a-switch :checked="sform.testMode" :loading="savingSettings" @change="toggleTestMode" />
        </div>
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
          ذخیره‌ی تنظیمات
        </a-button>
      </div>
    </section>

    <!-- counters -->
    <div class="stat-grid">
      <StatCard icon="users" label="کل بازیکنان" :value="board?.stats.total ?? 0" tone="primary" sub="همه‌ی ثبت‌نام‌شده‌ها در بازی" />
      <StatCard icon="gamepad" label="در حال بازی" :value="board?.stats.playing ?? 0" tone="info" sub="همین الان مشغول پاسخ دادن" />
      <StatCard icon="check-circle" label="بازی تمام‌شده" :value="board?.stats.finished ?? 0" tone="success" sub="نتیجه‌ی نهایی ثبت شده" />
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
        <span class="section-title grow"><AppIcon name="crown" :size="16" /> نفرات برتر (محاسبه‌ی خودکار)</span>
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
        <div v-for="r in livePlayers" :key="r.id" class="live-item">
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
      <div v-else class="empty">الان کسی در حال بازی نیست.</div>
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
          placeholder="همه‌ی وضعیت‌ها"
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
        <a-button size="small" :disabled="!board?.players.length" @click="exportCsv">
          <template #icon><AppIcon name="download" :size="13" /></template>
          خروجی Excel (CSV)
        </a-button>
        <a-button size="small" danger :disabled="!board?.players.length" @click="confirmDeleteAll">
          <template #icon><AppIcon name="trash" :size="13" /></template>
          حذف همه‌ی نتایج
        </a-button>
      </div>

      <!-- phones: one card per player instead of the wide table -->
      <div class="m-list">
        <div v-for="r in rows" :key="r.id" class="m-row" :class="{ top: r.rank && r.rank <= 3 && r.correct > 0 }">
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
        <div v-if="!rows.length" class="empty">هنوز بازیکنی ثبت‌نام نکرده است.</div>
      </div>

      <a-table
        class="d-table"
        :columns="columns"
        :data-source="rows"
        row-key="id"
        size="small"
        :pagination="{ pageSize: 20, hideOnSinglePage: true }"
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
              <a-tooltip title="مشاهده‌ی پاسخ‌ها">
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
      :total-players="DEMO_NAMES.length"
      @close="demoOpen = false"
    />
    <WinnerReveal
      v-if="showOpen"
      :podium="podium"
      :names="showNames"
      :timing="showTiming"
      :total-players="board?.stats.total ?? 0"
      @close="showOpen = false"
    />

    <!-- winners announcement (big, for a projector) -->
    <a-modal v-model:open="announceOpen" :footer="null" width="860px" centered :title="null" class="announce-modal">
      <div class="announce">
        <h2><AppIcon name="trophy" :size="26" /> برندگان Game مدرسه داتین</h2>
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
