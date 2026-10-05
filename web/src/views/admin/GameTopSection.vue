<script setup lang="ts">
// The current top three, big and at a glance — auto-ranked by correct answers,
// then total answer time — with everything the admin needs to announce them.
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { gameBoard, fmtSecs, type GameBoard, type BoardRow } from '@/api/game'
import { toFa, pad2 } from '@/lib/fa'
import AppIcon from '@/components/AppIcon.vue'
import WinnerReveal from '@/components/WinnerReveal.vue'

const admin = useAdminStore()
const board = ref<GameBoard | null>(null)
let poller: number | undefined

async function refresh() {
  try {
    board.value = await gameBoard()
  } catch (e) {
    admin.handleAuthError(e)
  }
}
onMounted(() => {
  void refresh()
  poller = window.setInterval(refresh, 2000)
})
onBeforeUnmount(() => clearInterval(poller))

const MEDALS = ['🥇', '🥈', '🥉']
const PLACE = ['نفر اول', 'نفر دوم', 'نفر سوم']
const podium = computed(() => board.value?.podium ?? [])
const stats = computed(() => board.value?.stats)

function phoneText(p: string): string {
  return /^9\d{9}$/.test(p) ? `0${p.slice(0, 3)} ${p.slice(3, 6)} ${p.slice(6)}` : p
}
function clock(ts: number): string {
  const d = new Date(ts)
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
}
function playMs(r: BoardRow): number {
  return r.finishedAt && r.startedAt ? r.finishedAt - r.startedAt : 0
}

/* announcement show, straight from here */
const showOpen = ref(false)
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
</script>

<template>
  <div class="stack">
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow"><AppIcon name="crown" :size="16" /> ۳ نفر برتر مسابقه</span>
        <span v-if="stats" class="faint small">
          {{ toFa(stats.finished) }} بازی تمام‌شده از {{ toFa(stats.total) }} بازیکن
          <template v-if="stats.playing"> — {{ toFa(stats.playing) }} نفر هنوز در حال بازی</template>
        </span>
        <a-button type="primary" :disabled="!podium.length" @click="showOpen = true">
          <template #icon><AppIcon name="trophy" :size="14" /></template>
          اعلام نتایج (نمایش زنده)
        </a-button>
      </div>
      <p class="faint small rule">
        رتبه‌بندی خودکار: ۱) تعداد جواب درست — بیشتر بهتر؛ ۲) در تساوی، مجموع زمان پاسخ — سریع‌تر بهتر.
        این فهرست هر ۲ ثانیه به‌روز می‌شود<template v-if="stats?.playing">؛ تا وقتی کسی در حال بازی است ممکن است تغییر کند</template>.
      </p>

      <div v-if="podium.length" class="top-list">
        <div v-for="(r, i) in podium" :key="r.id" class="top-card" :class="'t' + (i + 1)">
          <div class="top-head">
            <span class="top-medal">{{ MEDALS[i] }}</span>
            <div class="grow">
              <span class="top-place">{{ PLACE[i] }}</span>
              <div class="top-name">{{ r.firstName }} {{ r.lastName }}</div>
              <span class="phone faint">{{ phoneText(r.phone) }}</span>
            </div>
          </div>
          <div class="top-stats">
            <div class="ts">
              <b class="num">{{ toFa(r.correct) }} از {{ toFa(r.total) }}</b>
              <small>جواب درست</small>
            </div>
            <div class="ts">
              <b class="num">{{ fmtSecs(r.totalMs) }}</b>
              <small>ثانیه زمان پاسخ</small>
            </div>
            <div class="ts score">
              <b class="num">{{ toFa(r.score) }}</b>
              <small>امتیاز</small>
            </div>
          </div>
          <div class="top-times faint">
            <span>شروع: <span class="ltr-time">{{ toFa(clock(r.startedAt)) }}</span></span>
            <span>پایان: <span class="ltr-time">{{ toFa(clock(r.finishedAt)) }}</span></span>
            <span v-if="playMs(r)">کل بازی: {{ fmtSecs(playMs(r)) }} ثانیه</span>
          </div>
        </div>
      </div>
      <div v-else class="empty">
        هنوز کسی بازی را با دست‌کم یک جواب درست تمام نکرده است.
      </div>
    </section>

    <WinnerReveal
      v-if="showOpen"
      :podium="podium"
      :names="showNames"
      :timing="showTiming"
      @close="showOpen = false"
    />
  </div>
</template>

<style scoped>
.card-hd .section-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.small {
  font-size: 0.78rem;
}
.rule {
  margin-top: calc(-1 * var(--space-2));
}
.top-list {
  display: grid;
  gap: var(--space-3);
  grid-template-columns: repeat(3, 1fr);
}
.top-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-2);
}
.top-card.t1 {
  border-color: color-mix(in srgb, #f59e0b 60%, transparent);
  background: color-mix(in srgb, #f59e0b 12%, var(--surface));
  box-shadow: 0 0 0 1px color-mix(in srgb, #f59e0b 30%, transparent);
}
.top-card.t2 {
  border-color: color-mix(in srgb, #94a3b8 60%, transparent);
}
.top-card.t3 {
  border-color: color-mix(in srgb, #b45309 50%, transparent);
}
.top-head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.top-medal {
  font-size: 2.8rem;
  line-height: 1;
}
.top-place {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--text-dim);
}
.top-name {
  font-size: 1.2rem;
  font-weight: 800;
}
.top-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 6px;
}
.ts {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 4px;
  border-radius: var(--radius-sm);
  background: var(--surface);
  border: 1px solid var(--border);
  line-height: 1.3;
}
.ts b {
  font-size: 1.15rem;
  font-weight: 800;
}
.ts small {
  font-size: 0.7rem;
  color: var(--text-faint);
  text-align: center;
}
.ts.score b {
  color: var(--primary-bright);
}
.top-times {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  font-size: 0.75rem;
}
.phone,
.ltr-time {
  direction: ltr;
  unicode-bidi: isolate;
  display: inline-block;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.empty {
  padding: 28px;
  color: var(--text-faint);
  text-align: center;
}
@media (max-width: 1000px) {
  .top-list {
    grid-template-columns: 1fr;
  }
}
</style>
