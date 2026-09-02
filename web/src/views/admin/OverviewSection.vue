<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { message } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import { useAdminStore } from '@/stores/admin'
import { useEventStore } from '@/stores/event'
import {
  setConfig,
  downloadBackup,
  restoreBackup,
  checkForUpdate,
  smsStatus,
  smsTest,
} from '@/api/client'
import type { SmsInfo, SmsTestResult } from '@/api/types'
import { toFa, fmtPhone, statusLabel } from '@/lib/fa'

import StatCard from '@/components/StatCard.vue'
import MiniAreaChart from '@/components/MiniAreaChart.vue'
import CountdownClock from '@/components/CountdownClock.vue'
import StatusPill from '@/components/StatusPill.vue'
import Chip from '@/components/Chip.vue'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const eStore = useEventStore()
const { stats, participants } = storeToRefs(admin)
const { remainMs, state: eventState, config } = storeToRefs(eStore)

onMounted(() => eStore.start())
onBeforeUnmount(() => eStore.stop())

const DURATIONS = [
  { min: 60, label: '۱ ساعت' },
  { min: 360, label: '۶ ساعت' },
  { min: 1440, label: '۲۴ ساعت' },
  { min: 4320, label: '۳ روز' },
]
const custom = reactive({ d: 0, h: 0, m: 0 })
const extendMin = ref(30)

const clockState = computed(() =>
  eventState.value === 'waiting'
    ? 'waiting'
    : eventState.value === 'paused'
      ? 'paused'
      : eventState.value === 'ended'
        ? 'ended'
        : 'running',
)
const statusText = computed(() => {
  if (eventState.value === 'waiting') return 'زمان‌بندی‌شده — هنوز برای کاربران شروع نشده است'
  if (eventState.value === 'paused') return 'رویداد متوقف است — ارسال کد برای کاربران غیرفعال است'
  if (eventState.value === 'ended') return 'زمان رویداد به پایان رسیده — ارسال کد غیرفعال است'
  return ''
})

async function apply(patch: Record<string, unknown>, ok: string) {
  try {
    await setConfig(patch as never)
    await eStore.refresh()
    message.success(ok)
  } catch (e) {
    admin.toastError(e, 'خطا در به‌روزرسانی')
  }
}
const setDuration = (min: number) =>
  apply(
    { eventEndsAt: Date.now() + min * 60000, eventStartsAt: null, eventPaused: false, pausedRemainMs: null },
    'زمان رویداد به‌روزرسانی و با پنل کاربر همگام شد',
  )
function applyCustom() {
  const total = custom.d * 1440 + custom.h * 60 + custom.m
  if (total <= 0) return message.error('حداقل یکی از فیلدهای روز/ساعت/دقیقه را پر کنید')
  setDuration(total)
}
async function addMinutes() {
  if (!extendMin.value || extendMin.value <= 0) return message.error('یک عدد معتبر برای دقیقه وارد کنید')
  const c = config.value
  if (!c) return
  const patch = c.eventPaused
    ? { pausedRemainMs: Math.max(0, c.pausedRemainMs || 0) + extendMin.value * 60000 }
    : { eventEndsAt: (c.eventEndsAt || Date.now()) + extendMin.value * 60000 }
  await apply(patch, `${toFa(extendMin.value)} دقیقه به زمان رویداد اضافه شد`)
}
async function pause() {
  const c = config.value
  if (!c || c.eventPaused) return
  await apply(
    { eventPaused: true, pausedRemainMs: Math.max(0, (c.eventEndsAt || Date.now()) - Date.now()) },
    'رویداد متوقف شد — کاربران دیگر نمی‌توانند کد ارسال کنند',
  )
}
async function resume() {
  const c = config.value
  if (!c || !c.eventPaused) return
  await apply(
    { eventPaused: false, eventEndsAt: Date.now() + Math.max(0, c.pausedRemainMs || 0), pausedRemainMs: null },
    'رویداد با همان زمان باقی‌مانده ادامه یافت',
  )
}

/* registrations over the last 12 buckets */
const regSeries = computed(() => {
  const p = [...participants.value].sort((a, b) => a.registeredAt - b.registeredAt)
  if (!p.length) return [0, 0]
  const first = p[0].registeredAt
  const last = Math.max(p[p.length - 1].registeredAt, Date.now())
  const buckets = 12
  const span = Math.max(1, last - first)
  const out = new Array(buckets).fill(0)
  for (const u of p) {
    const idx = Math.min(buckets - 1, Math.floor(((u.registeredAt - first) / span) * buckets))
    for (let i = idx; i < buckets; i++) out[i]++
  }
  return out
})

const latest = computed(() => [...participants.value].slice(-6).reverse())

/* backup / restore / update */
const restoreInput = ref<HTMLInputElement>()
async function doDownload() {
  try {
    const blob = await downloadBackup()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `dotinschool-backup-${new Date().toISOString().slice(0, 10)}.db`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    message.success('دانلود بکاپ شروع شد')
  } catch (e) {
    admin.toastError(e, 'خطا در دریافت بکاپ')
  }
}
async function doRestore(ev: Event) {
  const file = (ev.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (!confirm('این عملیات تمام داده‌های فعلی را با محتوای این فایل بکاپ جایگزین می‌کند و سرویس ری‌استارت خواهد شد. ادامه می‌دهید؟')) {
    ;(ev.target as HTMLInputElement).value = ''
    return
  }
  try {
    await restoreBackup(file)
    message.success('بازیابی انجام شد — سرویس در حال ری‌استارت است؛ چند ثانیه صبر کنید و دوباره وارد شوید')
  } catch (e) {
    admin.toastError(e, 'خطا در بازیابی بکاپ')
  } finally {
    ;(ev.target as HTMLInputElement).value = ''
  }
}
/* SMS (ippanel) diagnostics */
const sms = ref<SmsInfo | null>(null)
const smsForm = reactive({ phone: '', purpose: 'register' as 'register' | 'login' })
const smsSending = ref(false)
const smsResult = ref<SmsTestResult | null>(null)
const smsRawOpen = ref(false)

onMounted(async () => {
  try {
    sms.value = await smsStatus()
  } catch (e) {
    admin.handleAuthError(e)
  }
})

async function sendTestSms() {
  const phone = smsForm.phone.replace(/\D/g, '').replace(/^98/, '').replace(/^0/, '')
  if (!/^9\d{9}$/.test(phone)) {
    message.error('شمارهٔ موبایل معتبر وارد کنید (مثل ۹۱۲۳۴۵۶۷۸۹)')
    return
  }
  smsSending.value = true
  smsResult.value = null
  try {
    smsResult.value = await smsTest({ phone, purpose: smsForm.purpose })
    if (smsResult.value.ok) message.success('پیامک آزمایشی با موفقیت ارسال شد')
    else message.error('ارسال ناموفق بود — جزئیات پایین را ببینید')
  } catch (e) {
    admin.toastError(e, 'ارسال پیامک آزمایشی ناموفق بود')
  } finally {
    smsSending.value = false
  }
}

const updating = ref(false)
const updateMsg = ref('')
async function doUpdate() {
  updating.value = true
  updateMsg.value = ''
  try {
    const r = await checkForUpdate()
    updateMsg.value = r.message
    if (r.updated) {
      message.success(r.message)
      setTimeout(() => location.reload(), 4000)
    } else {
      message.info(r.message)
      updating.value = false
    }
  } catch (e) {
    admin.toastError(e, 'بررسی به‌روزرسانی ناموفق بود')
    updating.value = false
  }
}
</script>

<template>
  <div class="stack">
    <div class="stat-grid">
      <StatCard icon="users" label="ثبت‌نامی" :value="toFa(stats.total)" tone="primary">
        <template #chart><MiniAreaChart :data="regSeries" :width="120" :height="34" /></template>
      </StatCard>
      <StatCard icon="code" label="در حال حل" :value="toFa(stats.started)" tone="info" />
      <StatCard icon="send" label="ارسال‌شده" :value="toFa(stats.submitted)" tone="success" />
      <StatCard icon="target" label="میانگین امتیاز" :value="toFa(stats.avg)" tone="accent" />
    </div>

    <div class="two-col">
      <section class="card card-pad clock-block">
        <span class="section-title">وضعیت رویداد</span>
        <CountdownClock
          :ms="remainMs"
          :state="clockState"
          size="xl"
          :label="eventState === 'waiting' ? 'تا شروع خودکار' : 'تا پایان رویداد'"
        />
        <Chip v-if="statusText" :tone="eventState === 'ended' ? 'danger' : 'warning'" dot>{{ statusText }}</Chip>

        <div class="dur-chips">
          <button
            v-for="d in DURATIONS"
            :key="d.min"
            type="button"
            class="dchip"
            :class="{ active: !config?.eventPaused && Math.abs(Math.round(remainMs / 60000) - d.min) < 2 }"
            @click="setDuration(d.min)"
          >
            {{ d.label }}
          </button>
        </div>

        <div class="custom-dur">
          <a-input-number v-model:value="custom.d" :min="0" addon-before="روز" />
          <a-input-number v-model:value="custom.h" :min="0" :max="23" addon-before="ساعت" />
          <a-input-number v-model:value="custom.m" :min="0" :max="59" addon-before="دقیقه" />
          <a-button type="primary" @click="applyCustom">تنظیم مدت</a-button>
        </div>

        <div class="row-wrap event-actions">
          <div class="row">
            <a-input-number v-model:value="extendMin" :min="1" addon-before="دقیقه" style="width: 150px" />
            <a-button @click="addMinutes"><template #icon><AppIcon name="plus" :size="14" /></template>افزودن</a-button>
          </div>
          <a-button v-if="eventState !== 'paused' && eventState !== 'waiting'" danger @click="pause">
            <template #icon><AppIcon name="pause" :size="14" /></template>توقف رویداد
          </a-button>
          <a-button v-if="eventState === 'paused'" type="primary" @click="resume">
            <template #icon><AppIcon name="play" :size="14" /></template>ادامه رویداد
          </a-button>
        </div>
      </section>

      <section class="card card-pad">
        <span class="section-title">آخرین شرکت‌کنندگان</span>
        <div class="scroll-x">
          <table class="mini-table">
            <thead>
              <tr><th>شناسه</th><th>نام</th><th>موبایل</th><th>وضعیت</th><th>امتیاز</th></tr>
            </thead>
            <tbody>
              <tr v-for="u in latest" :key="u.id">
                <td class="num">{{ u.publicId }}</td>
                <td>{{ u.firstName }} {{ u.lastName }}</td>
                <td class="num">{{ fmtPhone(u.phone) }}</td>
                <td><StatusPill :status="u.status" /></td>
                <td class="num">{{ u.score != null ? toFa(u.score) : '—' }}</td>
              </tr>
              <tr v-if="!latest.length"><td colspan="5" class="empty">هنوز کسی ثبت‌نام نکرده است</td></tr>
            </tbody>
          </table>
        </div>
        <p class="faint hint">به‌روزرسانی خودکار هر ۳ ثانیه — «{{ statusLabel('registered') }}» تا «ارسال‌شده».</p>
      </section>
    </div>

    <div class="two-col">
      <section class="card card-pad">
        <span class="section-title">پشتیبان‌گیری دیتابیس</span>
        <p class="faint hint">
          همه‌ی داده‌ها در یک فایل SQLite است. دانلود با روش امن <code>VACUUM INTO</code> انجام می‌شود؛
          بازیابی نسخه‌ی فعلی را قبل از جایگزینی نگه می‌دارد و سرویس را ری‌استارت می‌کند.
        </p>
        <div class="row-wrap">
          <a-button @click="doDownload"><template #icon><AppIcon name="download" :size="15" /></template>دانلود بکاپ</a-button>
          <a-button @click="restoreInput?.click()"><template #icon><AppIcon name="upload" :size="15" /></template>بازیابی از فایل</a-button>
          <input ref="restoreInput" type="file" accept=".db,.sqlite,application/octet-stream" hidden @change="doRestore" />
        </div>
      </section>

      <section class="card card-pad">
        <span class="section-title">به‌روزرسانی سامانه</span>
        <p class="faint hint">
          دریافت آخرین نسخه از گیت‌هاب و ری‌استارت سرویس. روی نصب Docker، دستور درست را نمایش می‌دهد.
        </p>
        <a-button :loading="updating" @click="doUpdate">
          <template #icon><AppIcon name="refresh" :size="15" /></template>بررسی و اعمال به‌روزرسانی
        </a-button>
        <p v-if="updateMsg" class="update-msg">{{ updateMsg }}</p>
      </section>
    </div>

    <section class="card card-pad">
      <div class="row-wrap" style="gap: 10px; margin-bottom: 8px">
        <span class="section-title">پیامک (OTP ثبت‌نام و ورود)</span>
        <Chip :tone="sms?.configured ? 'success' : 'danger'" dot>
          {{ sms?.configured ? 'کلید ippanel فعال است' : 'کلید ippanel تنظیم نشده' }}
        </Chip>
      </div>
      <p class="faint hint">
        اگر ثبت‌نام/ورود کاربر کد پیامکی نمی‌فرستد، این‌جا یک پیامک آزمایشی بفرستید تا پاسخِ دقیقِ
        سرویس ippanel (رد شدن پترن، خط فرستنده، نبودِ اعتبار …) دیده شود.
      </p>

      <div v-if="sms" class="sms-info num">
        <span>اندپوینت: <code dir="ltr">{{ sms.endpoint }}</code></span>
        <span>خط فرستنده: <code dir="ltr">{{ sms.sender || '—' }}</code></span>
        <span>پترن ثبت‌نام: <code dir="ltr">{{ sms.registerPattern || '—' }}</code></span>
        <span>پترن ورود: <code dir="ltr">{{ sms.loginPattern || '—' }}</code></span>
        <span>شمارهٔ ادمین: <code dir="ltr">{{ sms.adminPhone || '—' }}</code></span>
        <span>نوع Authorization: <code dir="ltr">{{ sms.authScheme }}</code></span>
      </div>

      <div class="row-wrap" style="margin-top: 12px; align-items: flex-end">
        <label class="sms-field">
          <span>شمارهٔ آزمایشی</span>
          <a-input v-model:value="smsForm.phone" dir="ltr" placeholder="9123456789" style="width: 180px" />
        </label>
        <label class="sms-field">
          <span>پترن</span>
          <a-select
            v-model:value="smsForm.purpose"
            style="width: 150px"
            :options="[
              { value: 'register', label: 'ثبت‌نام' },
              { value: 'login', label: 'ورود' },
            ]"
          />
        </label>
        <a-button type="primary" :loading="smsSending" @click="sendTestSms">
          <template #icon><AppIcon name="send" :size="15" /></template>ارسال پیامک آزمایشی
        </a-button>
      </div>

      <a-alert
        v-if="smsResult"
        :type="smsResult.ok ? 'success' : 'error'"
        show-icon
        style="margin-top: 12px"
        :message="smsResult.ok ? 'ارسال موفق بود' : `ارسال ناموفق (کد HTTP ${toFa(smsResult.status)})`"
      >
        <template #description>
          <div v-if="smsResult.message" style="margin-bottom: 6px">{{ smsResult.message }}</div>
          <a-button size="small" @click="smsRawOpen = !smsRawOpen">
            {{ smsRawOpen ? 'پنهان‌کردن پاسخ خام' : 'نمایش پاسخ خام سرویس' }}
          </a-button>
          <pre v-if="smsRawOpen" class="sms-raw" dir="ltr">{{ smsResult.rawBody || '(خالی)' }}</pre>
        </template>
      </a-alert>
    </section>
  </div>
</template>

<style scoped>
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
.two-col {
  display: grid;
  gap: var(--space-4);
}
@media (min-width: 960px) {
  .two-col {
    grid-template-columns: 1fr 1fr;
    align-items: start;
  }
}
.clock-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-4);
}
.dur-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
}
.dchip {
  padding: 6px 14px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text-dim);
  font-size: 0.8rem;
  font-weight: 600;
  transition: all var(--dur) var(--ease);
}
.dchip:hover {
  border-color: var(--primary);
  color: var(--primary);
}
.dchip.active {
  background: var(--primary);
  color: var(--on-primary);
  border-color: var(--primary);
}
.custom-dur {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
}
.custom-dur :deep(.ant-input-number) {
  width: 110px;
}
.event-actions {
  justify-content: center;
  gap: 10px;
}
.mini-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
}
.mini-table th {
  text-align: start;
  padding: 8px 10px;
  color: var(--text-faint);
  font-weight: 600;
  border-bottom: 1px solid var(--border);
}
.mini-table td {
  padding: 9px 10px;
  border-bottom: 1px solid var(--border);
}
.mini-table .empty,
.hint {
  color: var(--text-faint);
}
.hint {
  font-size: 0.74rem;
  margin: 10px 0 0;
}
.update-msg {
  margin-top: 10px;
  font-size: 0.8rem;
  color: var(--text-dim);
}
.sms-info {
  display: grid;
  gap: 6px;
  font-size: 0.76rem;
  color: var(--text-dim);
  margin-top: 6px;
}
.sms-info code {
  font-size: 0.9em;
  background: var(--surface-2);
  padding: 1px 6px;
  border-radius: 6px;
}
.sms-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 0.76rem;
  color: var(--text-dim);
}
.sms-raw {
  margin: 8px 0 0;
  background: var(--code-bg);
  color: var(--code-text);
  border-radius: var(--radius-sm);
  padding: 10px 12px;
  font-size: 0.74rem;
  line-height: 1.6;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
