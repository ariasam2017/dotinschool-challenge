<script setup lang="ts">
import { ref, computed } from 'vue'
import { message } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import { useAdminStore } from '@/stores/admin'
import { reviewCode, updateParticipant } from '@/api/client'
import type { Participant } from '@/api/types'
import { toFa, fmtPhone } from '@/lib/fa'
import Gauge from '@/components/Gauge.vue'
import MarkdownLite from '@/components/MarkdownLite.vue'
import Chip from '@/components/Chip.vue'
import StatusPill from '@/components/StatusPill.vue'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const { participants } = storeToRefs(admin)

const submitters = computed(() => participants.value.filter((u) => u.code).slice().reverse())
const selectedId = ref<string | null>(null)
const selected = computed(() => participants.value.find((u) => u.id === selectedId.value) || null)

const running = ref(false)
const report = ref('')
const reportModel = ref('')
const reportScore = ref<number | null>(null)

function pick(u: Participant) {
  selectedId.value = u.id
  report.value = ''
  reportScore.value = u.aiScore
  reportModel.value = ''
}

function aiColor(pct: number): string {
  return pct >= 65 ? 'var(--danger)' : pct >= 35 ? 'var(--warning)' : 'var(--success)'
}
function aiLabel(pct: number): string {
  return pct >= 65
    ? 'احتمال بالای تولید با هوش مصنوعی'
    : pct >= 35
      ? 'نامشخص — نیاز به بررسی دستی'
      : 'احتمالا نوشته خودِ کاربر'
}

async function runCheck() {
  const u = selected.value
  if (!u?.code) return message.error('این شرکت‌کننده کدی ارسال نکرده است')
  running.value = true
  report.value = ''
  try {
    const fname = `${u.firstName}_${u.lastName}`.replace(/\s+/g, '') + '.java'
    const { review, aiScore, model } = await reviewCode(fname, u.code)
    report.value = review
    reportModel.value = model
    reportScore.value = aiScore
    if (aiScore != null) {
      try {
        await updateParticipant(u.id, { aiScore })
      } catch (e) {
        admin.handleAuthError(e)
      }
      await admin.refreshParticipants()
    }
    message.success('بررسی کد این شرکت‌کننده انجام شد')
  } catch (e) {
    admin.toastError(e, 'بررسی ناموفق بود')
  } finally {
    running.value = false
  }
}

async function flag(kind: 'human' | 'suspicious') {
  const u = selected.value
  if (!u) return
  try {
    await updateParticipant(u.id, { aiFlag: kind })
    await admin.refreshParticipants()
    message.success(kind === 'human' ? 'کاربر به‌عنوان انسانی تأیید شد' : 'کاربر به‌عنوان مشکوک علامت‌گذاری شد')
  } catch (e) {
    admin.toastError(e, 'خطا در علامت‌گذاری')
  }
}
</script>

<template>
  <div class="ai-layout">
    <aside class="card list-col">
      <div class="list-head section-title">کدهای ارسال‌شده</div>
      <div class="list-body">
        <button
          v-for="u in submitters"
          :key="u.id"
          type="button"
          class="sub-row"
          :class="{ active: u.id === selectedId }"
          @click="pick(u)"
        >
          <div class="sub-main">
            <span class="sub-name">{{ u.firstName }} {{ u.lastName }}</span>
            <span class="sub-id num">{{ u.publicId }} · {{ fmtPhone(u.phone) }}</span>
          </div>
          <div class="sub-tags">
            <StatusPill :status="u.status" />
            <Chip
              v-if="u.aiScore != null"
              :tone="u.aiScore >= 65 ? 'danger' : u.aiScore >= 35 ? 'warning' : 'success'"
              dot
            >٪{{ toFa(u.aiScore) }}</Chip>
          </div>
        </button>
        <div v-if="!submitters.length" class="empty">هنوز کدی ارسال نشده است</div>
      </div>
    </aside>

    <section class="card card-pad detail-col">
      <div v-if="!selected" class="placeholder">
        <AppIcon name="wand" :size="32" />
        <p>یک شرکت‌کننده را از فهرست کناری انتخاب کنید</p>
      </div>

      <template v-else>
        <div class="detail-head">
          <div>
            <div class="d-name">{{ selected.firstName }} {{ selected.lastName }}</div>
            <div class="d-id num faint">{{ selected.publicId }} · {{ fmtPhone(selected.phone) }}</div>
          </div>
          <a-button type="primary" :loading="running" @click="runCheck">
            <template #icon><AppIcon name="wand" :size="15" /></template>
            {{ reportScore != null ? 'اجرای مجدد بررسی' : 'اجرای بررسی هوش مصنوعی' }}
          </a-button>
        </div>

        <pre class="code-preview" dir="ltr">{{ selected.code }}</pre>

        <div v-if="reportScore != null" class="score-row">
          <Gauge :percent="reportScore" :color="aiColor(reportScore)" :size="88" />
          <div>
            <div class="score-label" :style="{ color: aiColor(reportScore) }">{{ aiLabel(reportScore) }}</div>
            <div class="faint tiny">درصد تخمینی «تولید با هوش مصنوعی» برای این کد</div>
          </div>
        </div>

        <div v-if="running" class="placeholder">
          <a-spin />
          <p>هوش مصنوعی در حال بررسی کد است — تا یک دقیقه ممکن است طول بکشد</p>
        </div>

        <MarkdownLite v-if="report" :source="report" class="report" />

        <div v-if="report" class="report-actions">
          <a-button size="small" @click="flag('human')">تأیید انسانی بودن</a-button>
          <a-button size="small" danger @click="flag('suspicious')">علامت‌گذاری به‌عنوان مشکوک</a-button>
          <span class="faint tiny">مدل: {{ reportModel || '—' }}</span>
        </div>
      </template>
    </section>
  </div>
</template>

<style scoped>
.ai-layout {
  display: grid;
  gap: var(--space-4);
}
@media (min-width: 980px) {
  .ai-layout {
    grid-template-columns: 320px 1fr;
    align-items: start;
  }
}
.list-col {
  display: flex;
  flex-direction: column;
  max-height: calc(100dvh - var(--header-h) - 48px);
}
.list-head {
  padding: var(--space-4);
  border-bottom: 1px solid var(--border);
}
.list-body {
  overflow-y: auto;
  padding: var(--space-2);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.sub-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: none;
  background: transparent;
  border-radius: var(--radius-sm);
  text-align: start;
  transition: background var(--dur) var(--ease);
}
.sub-row:hover {
  background: var(--surface-2);
}
.sub-row.active {
  background: var(--primary-soft);
}
.sub-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.sub-name {
  font-size: 0.84rem;
  font-weight: 600;
}
.sub-id {
  font-size: 0.68rem;
  color: var(--text-faint);
}
.sub-tags {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-end;
}
.empty,
.placeholder {
  padding: 32px;
  text-align: center;
  color: var(--text-faint);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.detail-col {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  min-height: 320px;
}
.detail-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.d-name {
  font-weight: 800;
  font-size: 1rem;
}
.d-id {
  font-size: 0.72rem;
}
.code-preview {
  background: var(--code-bg);
  color: var(--code-text);
  border-radius: var(--radius-sm);
  padding: 14px;
  font-family: var(--font-mono);
  font-size: 0.78rem;
  line-height: 1.7;
  max-height: 300px;
  overflow: auto;
  direction: ltr;
  text-align: left;
  white-space: pre;
}
.score-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.score-label {
  font-weight: 700;
  font-size: 0.9rem;
}
.tiny {
  font-size: 0.72rem;
}
.report-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
</style>
