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
  exportParticipants,
  importParticipants,
  checkForUpdate,
  smsStatus,
  smsTest,
  getSmsSettings,
  saveSmsSettings,
} from '@/api/client'
import type {
  SmsInfo,
  SmsTestResult,
  SmsSettings,
  ParticipantExportFile,
} from '@/api/types'
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

/* registrations in the last 15 minutes — drives the "ثبت‌نامی" card trend badge */
const recentReg = computed(() => {
  const cut = Date.now() - 15 * 60_000
  return participants.value.filter((u) => u.registeredAt >= cut).length
})

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

/* portable participant transfer — server to server, no restart */
const partInput = ref<HTMLInputElement>()
async function doExportParticipants() {
  try {
    const blob = await exportParticipants()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `dotinschool-participants-${new Date().toISOString().slice(0, 10)}.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    message.success('خروجی شرکت‌کنندگان دانلود شد')
  } catch (e) {
    admin.toastError(e, 'خطا در گرفتن خروجی شرکت‌کنندگان')
  }
}
async function doImportParticipants(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  let parsed: ParticipantExportFile
  try {
    parsed = JSON.parse(await file.text())
  } catch {
    message.error('فایل انتخاب‌شده JSON معتبر نیست')
    input.value = ''
    return
  }
  const n = Array.isArray(parsed?.participants) ? parsed.participants.length : 0
  if (
    !confirm(
      `همهٔ شرکت‌کنندگان فعلی این سامانه پاک و با ${toFa(n)} شرکت‌کنندهٔ این فایل جایگزین می‌شوند. ` +
        'سرویس ری‌استارت نمی‌شود. ادامه می‌دهید؟',
    )
  ) {
    input.value = ''
    return
  }
  try {
    const r = await importParticipants(parsed)
    message.success(`انتقال انجام شد — ${toFa(r.count)} شرکت‌کننده جایگزین شد`)
    await admin.refreshParticipants()
  } catch (e) {
    admin.toastError(e, 'خطا در ورودی شرکت‌کنندگان')
  } finally {
    input.value = ''
  }
}
/* SMS (ippanel) diagnostics */
const sms = ref<SmsInfo | null>(null)
const smsForm = reactive({ phone: '', purpose: 'register' as 'register' | 'login' })
const smsSending = ref(false)
const smsResult = ref<SmsTestResult | null>(null)
const smsRawOpen = ref(false)

async function refreshSmsStatus() {
  try {
    sms.value = await smsStatus()
  } catch (e) {
    admin.handleAuthError(e)
  }
}

/* SMS (ippanel) panel-managed settings — so the admin can paste the API key
   here after moving the app to a new server, with no .env edit / rebuild. */
const smsSettingsForm = reactive({
  apiKey: '',
  sender: '',
  baseUrl: '',
  registerPattern: '',
  loginPattern: '',
  adminNotifyPattern: '',
  adminPhone: '',
  authScheme: '',
})
const smsSettingsHasKey = ref(false)
const smsSettingsKeyMask = ref('')
const smsSettingsSaving = ref(false)

function applySmsSettings(v: SmsSettings) {
  smsSettingsForm.apiKey = ''
  smsSettingsForm.sender = v.sender
  smsSettingsForm.baseUrl = v.baseUrl
  smsSettingsForm.registerPattern = v.registerPattern
  smsSettingsForm.loginPattern = v.loginPattern
  smsSettingsForm.adminNotifyPattern = v.adminNotifyPattern
  smsSettingsForm.adminPhone = v.adminPhone
  smsSettingsForm.authScheme = v.authScheme
  smsSettingsHasKey.value = v.hasKey
  smsSettingsKeyMask.value = v.keyMask
}

async function refreshSmsSettings() {
  try {
    applySmsSettings(await getSmsSettings())
  } catch (e) {
    admin.handleAuthError(e)
  }
}

async function saveSmsSettingsForm() {
  smsSettingsSaving.value = true
  try {
    const saved = await saveSmsSettings({
      apiKey: smsSettingsForm.apiKey.trim() || undefined,
      sender: smsSettingsForm.sender.trim(),
      baseUrl: smsSettingsForm.baseUrl.trim(),
      registerPattern: smsSettingsForm.registerPattern.trim(),
      loginPattern: smsSettingsForm.loginPattern.trim(),
      adminNotifyPattern: smsSettingsForm.adminNotifyPattern.trim(),
      adminPhone: smsSettingsForm.adminPhone.trim(),
      authScheme: smsSettingsForm.authScheme.trim(),
    })
    applySmsSettings(saved)
    message.success('تنظیمات پیامک ذخیره شد')
    await refreshSmsStatus()
  } catch (e) {
    admin.toastError(e, 'خطا در ذخیرهٔ تنظیمات پیامک')
  } finally {
    smsSettingsSaving.value = false
  }
}

onMounted(() => {
  void refreshSmsStatus()
  void refreshSmsSettings()
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
      <StatCard
        icon="users"
        label="ثبت‌نامی"
        :value="stats.total"
        tone="primary"
        :trend="recentReg > 0 ? 'up' : 'flat'"
        :trend-text="recentReg > 0 ? toFa(recentReg) : ''"
        :trend-title="recentReg > 0 ? `${toFa(recentReg)} ثبت‌نام در ۱۵ دقیقهٔ اخیر` : 'در ۱۵ دقیقهٔ اخیر ثبت‌نام تازه‌ای نبوده'"
      >
        <template #chart>
          <MiniAreaChart :data="regSeries" :height="42" color="var(--primary)" />
          <span class="chart-cap faint">روند تجمعی ثبت‌نام از آغاز رویداد تا این لحظه</span>
        </template>
      </StatCard>
      <StatCard
        icon="code"
        label="در حال حل"
        :value="stats.started"
        tone="info"
        sub="کاربرانی که صورت مسئله را باز کرده‌اند و هنوز پاسخ نداده‌اند"
      />
      <StatCard
        icon="send"
        label="ارسال‌شده"
        :value="stats.submitted"
        tone="success"
        sub="کاربرانی که پاسخ نهایی خود را ثبت کرده‌اند"
      />
      <StatCard
        icon="target"
        label="میانگین امتیاز"
        :value="stats.avg"
        tone="accent"
        sub="میانگین نمرهٔ همهٔ کاربرانِ نمره‌خورده (از ۱۰۰)"
      />
    </div>

    <p class="faint grid-note">
      این چهار کارت وضعیت زندهٔ رویداد را نشان می‌دهند و هر ۳ ثانیه به‌روزرسانی می‌شوند؛ با هر تغییر،
      عدد به‌صورت شمارشی جابه‌جا می‌شود و کارت یک لحظه برجسته می‌شود. نمودار زیر «ثبت‌نامی» شمار تجمعی
      ثبت‌نام‌ها را در ۱۲ بازهٔ زمانیِ برابر رسم می‌کند: بالا رفتنِ خط یعنی ثبت‌نام تازه، و بخش صافِ خط
      یعنی در آن بازه کسی ثبت‌نام نکرده است.
    </p>

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
        <div class="card-hd"><span class="section-title">آخرین شرکت‌کنندگان</span></div>
        <div class="scroll-x">
          <table class="data-table">
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
        <div class="card-hd"><span class="section-title">پشتیبان‌گیری دیتابیس</span></div>
        <p class="faint hint">
          کل دیتابیس در یک فایل SQLite است. دانلود با روش امن <code>VACUUM INTO</code> انجام می‌شود.
          این بکاپ را می‌توانید روی یک سرور تازه و خام هم بازیابی کنید: پیش از جایگزینی، فایل باز و
          بررسی می‌شود و اگر سالم یا سازگار نباشد بازیابی انجام نمی‌شود و داده‌های فعلی دست‌نخورده
          می‌ماند. یک نسخهٔ ایمن از دیتابیس فعلی هم قبل از جایگزینی نگه داشته می‌شود، بعد سرویس
          ری‌استارت می‌شود.
        </p>
        <div class="row-wrap">
          <a-button @click="doDownload"><template #icon><AppIcon name="download" :size="15" /></template>دانلود بکاپ</a-button>
          <a-button @click="restoreInput?.click()"><template #icon><AppIcon name="upload" :size="15" /></template>بازیابی از فایل .db</a-button>
          <input ref="restoreInput" type="file" accept=".db,.sqlite,application/octet-stream" hidden @change="doRestore" />
        </div>
        <p class="faint hint warn-note">
          <AppIcon name="alert-triangle" :size="13" />
          برای نتیجهٔ مطمئن، بهتر است سرور مبدأ و مقصد روی یک نسخه از سامانه باشند. برای جابه‌جایی فقط
          شرکت‌کنندگان بدون ری‌استارت، از «انتقال شرکت‌کنندگان» پایین استفاده کنید.
        </p>

        <div class="sub-divider" />
        <div class="card-hd"><span class="section-title">انتقال شرکت‌کنندگان (بین سرورها)</span></div>
        <p class="faint hint">
          فقط ردیف‌های شرکت‌کنندگان را به‌صورت یک فایل <code>.json</code> جابه‌جا می‌کند — تعداد
          ثبت‌نامی‌ها، در حال حل‌ها و ارسال‌شده‌ها منتقل می‌شود. «ورودی» جدول شرکت‌کنندگان سامانهٔ
          مقصد را <b>کامل پاک و جایگزین</b> می‌کند و سرویس ری‌استارت نمی‌شود.
        </p>
        <div class="row-wrap">
          <a-button @click="doExportParticipants">
            <template #icon><AppIcon name="download" :size="15" /></template>خروجی شرکت‌کنندگان
          </a-button>
          <a-button danger @click="partInput?.click()">
            <template #icon><AppIcon name="upload" :size="15" /></template>ورودی از فایل (جایگزینی کامل)
          </a-button>
          <input ref="partInput" type="file" accept="application/json,.json" hidden @change="doImportParticipants" />
        </div>
      </section>

      <section class="card card-pad">
        <div class="card-hd"><span class="section-title">به‌روزرسانی سامانه</span></div>
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
      <div class="card-hd">
        <span class="section-title">پیامک (OTP ثبت‌نام و ورود)</span>
        <Chip :tone="sms?.configured ? 'success' : 'danger'" dot>
          {{ sms?.configured ? 'کلید ippanel فعال است' : 'کلید ippanel تنظیم نشده' }}
        </Chip>
      </div>
      <p class="faint hint">
        اگر روی سرور جدیدی (لینوکس یا ویندوز) بالا آورده‌اید، کلید API ippanel را همین‌جا وارد و
        ذخیره کنید — نیازی به ویرایش <code>.env</code> یا build دوباره نیست. فیلدهای خالی از مقدار
        پیش‌فرض/<code>.env</code> استفاده می‌کنند.
      </p>

      <div class="sms-settings-grid">
        <label class="sms-field">
          <span>کلید API ippanel</span>
          <a-input-password
            v-model:value="smsSettingsForm.apiKey"
            dir="ltr"
            autocomplete="off"
            :placeholder="smsSettingsHasKey ? smsSettingsKeyMask + ' — برای تغییر، کلید جدید بزنید' : 'کلید API را وارد کنید'"
          />
        </label>
        <label class="sms-field">
          <span>خط فرستنده (اختیاری)</span>
          <a-input v-model:value="smsSettingsForm.sender" dir="ltr" placeholder="پیش‌فرض .env" />
        </label>
        <label class="sms-field">
          <span>آدرس اندپوینت (اختیاری)</span>
          <a-input v-model:value="smsSettingsForm.baseUrl" dir="ltr" placeholder="https://edge.ippanel.com/v1/api/send" />
        </label>
        <label class="sms-field">
          <span>نوع Authorization (اختیاری)</span>
          <a-select
            v-model:value="smsSettingsForm.authScheme"
            :allow-clear="true"
            placeholder="پیش‌فرض .env"
            :options="[
              { value: 'raw', label: 'raw (خودِ کلید)' },
              { value: 'accesskey', label: 'AccessKey' },
              { value: 'apikey', label: 'apikey' },
              { value: 'bearer', label: 'Bearer' },
            ]"
          />
        </label>
        <label class="sms-field">
          <span>پترن ثبت‌نام (اختیاری)</span>
          <a-input v-model:value="smsSettingsForm.registerPattern" dir="ltr" placeholder="پیش‌فرض .env" />
        </label>
        <label class="sms-field">
          <span>پترن ورود (اختیاری)</span>
          <a-input v-model:value="smsSettingsForm.loginPattern" dir="ltr" placeholder="پیش‌فرض .env" />
        </label>
        <label class="sms-field">
          <span>پترن اطلاع‌رسانی ادمین (اختیاری)</span>
          <a-input v-model:value="smsSettingsForm.adminNotifyPattern" dir="ltr" placeholder="پیش‌فرض .env" />
        </label>
        <label class="sms-field">
          <span>شمارهٔ ادمین (اختیاری)</span>
          <a-input v-model:value="smsSettingsForm.adminPhone" dir="ltr" placeholder="9XXXXXXXXX" />
        </label>
      </div>

      <div class="row-wrap" style="margin-top: 10px">
        <a-button type="primary" :loading="smsSettingsSaving" @click="saveSmsSettingsForm">
          <template #icon><AppIcon name="check" :size="15" /></template>ذخیره تنظیمات پیامک
        </a-button>
      </div>

      <div v-if="sms" class="sms-info num">
        <span>اندپوینت مؤثر: <code dir="ltr">{{ sms.endpoint }}</code></span>
        <span>خط فرستنده مؤثر: <code dir="ltr">{{ sms.sender || '—' }}</code></span>
        <span>پترن ثبت‌نام مؤثر: <code dir="ltr">{{ sms.registerPattern || '—' }}</code></span>
        <span>پترن ورود مؤثر: <code dir="ltr">{{ sms.loginPattern || '—' }}</code></span>
        <span>شمارهٔ ادمین مؤثر: <code dir="ltr">{{ sms.adminPhone || '—' }}</code></span>
        <span>نوع Authorization مؤثر: <code dir="ltr">{{ sms.authScheme }}</code></span>
        <span>منبع تنظیمات: <code dir="ltr">{{ sms.source === 'panel' ? 'پنل' : sms.source === 'env' ? '.env' : '—' }}</code></span>
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
  border-color: var(--primary-bright);
  color: var(--primary-bright);
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
  white-space: pre-line;
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
.sms-settings-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: 1fr;
  margin-top: 4px;
}
@media (min-width: 720px) {
  .sms-settings-grid {
    grid-template-columns: 1fr 1fr;
  }
}
.sms-settings-grid .sms-field > span {
  font-weight: 600;
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

/* caution note under the .db restore button */
.warn-note {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  color: var(--warning);
}
.warn-note :deep(svg) {
  flex: none;
  margin-top: 2px;
}
/* separates the two blocks inside the backup card */
.sub-divider {
  height: 1px;
  background: var(--border);
  margin: var(--space-4) calc(-1 * var(--space-5));
}

/* caption under the registration sparkline inside the "ثبت‌نامی" card */
.chart-cap {
  font-size: 0.68rem;
  line-height: 1.5;
}
.grid-note {
  font-size: 0.74rem;
  line-height: 1.7;
  margin: -4px 0 0;
}

</style>
