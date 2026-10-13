<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { systemStats } from '@/api/client'
import type { SystemStats } from '@/api/types'
import { toFa } from '@/lib/fa'
import Gauge from '@/components/Gauge.vue'
import AppIcon from '@/components/AppIcon.vue'
import Chip from '@/components/Chip.vue'

const admin = useAdminStore()

/* ---- live host resources (CPU / RAM / Swap), polled ---- */
const sys = ref<SystemStats | null>(null)
let sysTimer: number | undefined
async function refreshSys() {
  try {
    sys.value = await systemStats()
  } catch (e) {
    admin.handleAuthError(e)
  }
}
onMounted(() => {
  void refreshSys()
  // The backend now samples in the background every 1s and just serves the
  // cache (no per-request blocking), so polling at the same 1s cadence is
  // cheap and gives a genuinely live reading instead of a stale one.
  sysTimer = window.setInterval(refreshSys, 1000)
})
onBeforeUnmount(() => {
  if (sysTimer) clearInterval(sysTimer)
})

const GIB = 1024 ** 3
const MIB = 1024 ** 2
function fmtBytes(n: number): string {
  if (!n) return `${toFa(0)}`
  if (n >= GIB) return `${toFa((n / GIB).toFixed(1)).replace('.', '٫')} گیگ`
  if (n >= MIB) return `${toFa(Math.round(n / MIB))} مگ`
  return `${toFa(Math.round(n / 1024))} کیلو`
}
function fmtUptime(sec: number): string {
  if (!sec) return '—'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d) return `${toFa(d)} روز و ${toFa(h)} ساعت`
  if (h) return `${toFa(h)} ساعت و ${toFa(m)} دقیقه`
  return `${toFa(m)} دقیقه`
}
const resColor = (p: number) =>
  p >= 90 ? 'var(--danger)' : p >= 70 ? 'var(--warning)' : 'var(--primary)'
/** Persian digits with the Persian decimal separator. */
const faDec = (n: number) => toFa(String(n)).replace('.', '٫')
</script>

<template>
  <div class="stack">
    <section class="card card-pad res-card">
      <div class="card-hd">
        <span class="section-title">منابع سرور</span>
        <Chip :tone="sys ? 'success' : 'neutral'" dot>{{ sys ? 'زنده' : 'در حال دریافت…' }}</Chip>
      </div>
      <p class="faint hint res-intro">
        مصرف لحظه‌ایِ ماشینی که سامانه روی آن اجرا می‌شود — مانند پنل‌های x-ui / سنایی. اگر «پردازنده» یا
        «حافظه» مدام نزدیک ۱۰۰٪ بماند، سرور برای بار رویداد کوچک است و باید ارتقا یابد.
      </p>
      <div class="res-grid">
        <div class="res-item">
          <Gauge :percent="sys?.cpu.percent ?? 0" :color="resColor(sys?.cpu.percent ?? 0)" :size="104" />
          <div class="res-meta">
            <span class="res-name"><AppIcon name="cpu" :size="14" />پردازنده</span>
            <span class="res-detail num">{{ toFa(sys?.cpu.cores ?? 0) }} هسته</span>
          </div>
        </div>
        <div class="res-item">
          <Gauge :percent="sys?.mem.percent ?? 0" :color="resColor(sys?.mem.percent ?? 0)" :size="104" />
          <div class="res-meta">
            <span class="res-name"><AppIcon name="server" :size="14" />حافظه (RAM)</span>
            <span class="res-detail num">
              {{ fmtBytes(sys?.mem.used ?? 0) }} / {{ fmtBytes(sys?.mem.total ?? 0) }}
            </span>
          </div>
        </div>
        <div class="res-item">
          <Gauge :percent="sys?.swap.percent ?? 0" :color="resColor(sys?.swap.percent ?? 0)" :size="104" />
          <div class="res-meta">
            <span class="res-name"><AppIcon name="database" :size="14" />Swap</span>
            <span class="res-detail num">
              {{ fmtBytes(sys?.swap.used ?? 0) }} / {{ fmtBytes(sys?.swap.total ?? 0) }}
            </span>
          </div>
        </div>
      </div>
      <p class="faint hint res-foot">
        <span>آپ‌تایم سرور: <b class="num">{{ fmtUptime(sys?.uptime ?? 0) }}</b></span>
        <span v-if="(sys?.load?.[0] ?? 0) > 0" dir="ltr">
          load: <b class="num">{{ faDec(sys!.load[0]) }} / {{ faDec(sys!.load[1]) }} / {{ faDec(sys!.load[2]) }}</b>
        </span>
        <span>پردازش‌های همزمان: <b class="num">{{ toFa(sys?.go.goroutines ?? 0) }}</b></span>
        <span>حافظه خودِ برنامه: <b class="num">{{ fmtBytes(sys?.go.alloc ?? 0) }}</b></span>
      </p>
      <p class="faint hint">به‌روزرسانی خودکار هر ۱ ثانیه — همه اعداد بالا همین لحظه از سرور خوانده می‌شوند.</p>
    </section>
  </div>
</template>

<style scoped>
.hint {
  font-size: 0.74rem;
  margin: 10px 0 0;
  color: var(--text-faint);
}
.res-intro {
  margin: 0 0 var(--space-3);
}
.res-grid {
  display: grid;
  gap: var(--space-4);
  grid-template-columns: 1fr;
}
@media (min-width: 620px) {
  .res-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}
.res-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface-2);
}
.res-meta {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  text-align: center;
}
.res-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text);
}
.res-detail {
  font-size: 0.74rem;
  color: var(--text-dim);
}
.res-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 6px var(--space-4);
  margin: var(--space-3) 0 0;
  font-size: 0.74rem;
}
.res-foot b {
  color: var(--text-dim);
  font-weight: 700;
}
</style>
