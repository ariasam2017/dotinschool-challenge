<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import { useAdminStore } from '@/stores/admin'
import { useEventStore } from '@/stores/event'
import { setConfig } from '@/api/client'
import { toJalali, toGregorian, jalaliToDate, jalaliMonthLength, JALALI_MONTHS } from '@/lib/jalali'
import { toFa, pad2, fmtJalaliDateTime } from '@/lib/fa'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const eStore = useEventStore()
const { config } = storeToRefs(eStore)

onMounted(() => {
  eStore.start()
  const now = new Date()
  const j = toJalali(now.getFullYear(), now.getMonth() + 1, now.getDate())
  sch.jy = j.jy
  sch.jm = j.jm
  sch.jd = j.jd
  sch.hh = Math.min(23, now.getHours() + 1)
  sch.mm = 0
})

const sch = reactive({ jy: 1403, jm: 1, jd: 1, hh: 12, mm: 0 })
const dur = reactive({ d: 0, h: 1, m: 30 })
const busy = ref(false)

const maxDay = computed(() => jalaliMonthLength(sch.jy, sch.jm))
const durationMin = computed(() => dur.d * 1440 + dur.h * 60 + dur.m)

const scheduledFor = computed(() => {
  const s = config.value?.eventStartsAt
  return s && s > Date.now() ? s : null
})

async function scheduleStart() {
  if (!sch.jy || !sch.jm || !sch.jd) return message.error('تاریخ شروع را کامل وارد کنید')
  if (sch.hh < 0 || sch.hh > 23 || sch.mm < 0 || sch.mm > 59)
    return message.error('ساعت باید بین ۰ تا ۲۳ و دقیقه بین ۰ تا ۵۹ باشد')
  const startsAt = jalaliToDate(sch.jy, sch.jm, sch.jd, sch.hh, sch.mm).getTime()
  if (Number.isNaN(startsAt)) return message.error('تاریخ نامعتبر است')
  if (startsAt <= Date.now()) return message.error('زمان شروع باید در آینده باشد')
  if (durationMin.value <= 0) return message.error('مدت‌زمان چالش پس از شروع را مشخص کنید')

  busy.value = true
  try {
    await setConfig({
      eventStartsAt: startsAt,
      eventEndsAt: startsAt + durationMin.value * 60000,
      eventPaused: false,
      pausedRemainMs: null,
    } as never)
    await eStore.refresh()
    message.success('چالش زمان‌بندی شد — در زمان مشخص‌شده خودکار برای کاربران شروع می‌شود')
  } catch (e) {
    admin.toastError(e, 'خطا در زمان‌بندی')
  } finally {
    busy.value = false
  }
}
async function forceStartNow() {
  const min = durationMin.value || 90
  busy.value = true
  try {
    await setConfig({
      eventStartsAt: null,
      eventEndsAt: Date.now() + min * 60000,
      eventPaused: false,
      pausedRemainMs: null,
    } as never)
    await eStore.refresh()
    message.success('زمان‌بندی نادیده گرفته شد — چالش همین الان برای همه باز شد')
  } catch (e) {
    admin.toastError(e, 'خطا در شروع فوری')
  } finally {
    busy.value = false
  }
}
async function cancelSchedule() {
  busy.value = true
  try {
    await setConfig({ eventStartsAt: null } as never)
    await eStore.refresh()
    message.success('زمان‌بندی لغو شد')
  } catch (e) {
    admin.toastError(e, 'خطا در لغو زمان‌بندی')
  } finally {
    busy.value = false
  }
}

/* standalone converter */
const conv = reactive({ jy: 1403, jm: 1, jd: 1 })
const convResult = computed(() => {
  try {
    const g = toGregorian(conv.jy, conv.jm, conv.jd)
    return `${g.gy}-${pad2(g.gm)}-${pad2(g.gd)}`
  } catch {
    return '—'
  }
})
const convBack = reactive({ gy: 2024, gm: 3, gd: 20 })
const convBackResult = computed(() => {
  try {
    const j = toJalali(convBack.gy, convBack.gm, convBack.gd)
    return `${j.jy}/${pad2(j.jm)}/${pad2(j.jd)}`
  } catch {
    return '—'
  }
})
</script>

<template>
  <div class="stack">
    <section class="card card-pad">
      <span class="section-title">زمان‌بندی شروع خودکار (تقویم جلالی)</span>
      <p class="faint hint">
        در لحظه تعیین‌شده، یک کرون‌جاب رویداد را خودکار برای کاربران زنده می‌کند — بدون هیچ اقدام دستی.
      </p>

      <div class="sched-grid">
        <label class="f"><span>سال</span><a-input-number v-model:value="sch.jy" :min="1300" :max="1500" /></label>
        <label class="f"><span>ماه</span>
          <a-select v-model:value="sch.jm" :options="JALALI_MONTHS.map((m, i) => ({ value: i + 1, label: m }))" />
        </label>
        <label class="f"><span>روز</span><a-input-number v-model:value="sch.jd" :min="1" :max="maxDay" /></label>
        <label class="f"><span>ساعت</span><a-input-number v-model:value="sch.hh" :min="0" :max="23" /></label>
        <label class="f"><span>دقیقه</span><a-input-number v-model:value="sch.mm" :min="0" :max="59" /></label>
      </div>

      <div class="sched-grid dur">
        <span class="dur-label">مدت‌زمان چالش پس از شروع:</span>
        <a-input-number v-model:value="dur.d" :min="0" addon-before="روز" />
        <a-input-number v-model:value="dur.h" :min="0" :max="23" addon-before="ساعت" />
        <a-input-number v-model:value="dur.m" :min="0" :max="59" addon-before="دقیقه" />
      </div>

      <div class="row-wrap actions">
        <a-button type="primary" :loading="busy" @click="scheduleStart">
          <template #icon><AppIcon name="calendar" :size="15" /></template>زمان‌بندی
        </a-button>
        <a-button :loading="busy" @click="forceStartNow">
          <template #icon><AppIcon name="play" :size="15" /></template>شروع فوری (نادیده‌گرفتن زمان‌بندی)
        </a-button>
        <a-button v-if="scheduledFor" danger :loading="busy" @click="cancelSchedule">
          <template #icon><AppIcon name="x" :size="15" /></template>لغو زمان‌بندی
        </a-button>
      </div>

      <a-alert
        v-if="scheduledFor"
        type="info"
        show-icon
        :message="`چالش زمان‌بندی شده برای: ${fmtJalaliDateTime(scheduledFor)}`"
        style="margin-top: 14px"
      />
      <p v-else class="faint hint" style="margin-top: 12px">
        در حال حاضر زمان‌بندی‌ای فعال نیست — با «شروع فوری» رویداد بلافاصله زنده می‌شود.
      </p>
    </section>

    <section class="card card-pad">
      <span class="section-title">مبدل تاریخ جلالی ↔ میلادی</span>
      <div class="conv">
        <div class="conv-row">
          <span class="faint">جلالی</span>
          <a-input-number v-model:value="conv.jy" :min="1" />
          <a-input-number v-model:value="conv.jm" :min="1" :max="12" />
          <a-input-number v-model:value="conv.jd" :min="1" :max="31" />
          <AppIcon name="chevron-left" :size="16" />
          <code class="num">{{ convResult }}</code>
        </div>
        <div class="conv-row">
          <span class="faint">میلادی</span>
          <a-input-number v-model:value="convBack.gy" :min="1" />
          <a-input-number v-model:value="convBack.gm" :min="1" :max="12" />
          <a-input-number v-model:value="convBack.gd" :min="1" :max="31" />
          <AppIcon name="chevron-left" :size="16" />
          <code class="num">{{ toFa(convBackResult) }}</code>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.hint {
  font-size: 0.76rem;
  margin: 6px 0 14px;
}
.sched-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: end;
}
.sched-grid.dur {
  margin-top: 16px;
  align-items: center;
}
.dur-label {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-dim);
}
.f {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 0.76rem;
  color: var(--text-dim);
}
.f :deep(.ant-input-number),
.f :deep(.ant-select) {
  width: 96px;
}
.actions {
  margin-top: 18px;
  gap: 10px;
}
.conv {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.conv-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.conv-row :deep(.ant-input-number) {
  width: 88px;
}
.conv-row code {
  font-size: 0.95rem;
  padding: 4px 10px;
  background: var(--surface-2);
  border-radius: 8px;
}
</style>
