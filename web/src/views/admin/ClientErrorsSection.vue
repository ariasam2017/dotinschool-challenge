<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import { listClientErrors, markClientErrorsSeen, deleteClientError, clearClientErrors } from '@/api/client'
import type { ClientErrorRow } from '@/api/types'
import { fmtTime } from '@/lib/fa'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const rows = ref<ClientErrorRow[]>([])
const expanded = ref<Set<string>>(new Set())
const loading = ref(false)

const CONTEXT_LABELS: Record<string, string> = {
  submit: 'ارسال کد',
  start: 'شروع چالش',
  otp: 'کد تأیید',
  register: 'ثبت‌نام',
}

async function load(markSeen = true) {
  loading.value = true
  try {
    const data = await listClientErrors()
    rows.value = data.errors
    if (markSeen && data.unseen > 0) {
      await markClientErrorsSeen()
      admin.unseenErrors = 0
    }
    await admin.refreshErrorBadge()
  } catch (e) {
    admin.toastError(e, 'خطا در دریافت خطاها')
  } finally {
    loading.value = false
  }
}
onMounted(() => load(true))

function toggle(id: string) {
  expanded.value.has(id) ? expanded.value.delete(id) : expanded.value.add(id)
  expanded.value = new Set(expanded.value)
}
async function removeOne(id: string) {
  try {
    await deleteClientError(id)
    await load(false)
  } catch (e) {
    admin.toastError(e, 'حذف نشد')
  }
}
function clearAll() {
  Modal.confirm({
    title: 'پاک‌کردن همه خطاها',
    content: 'همه خطاهای ثبت‌شده کاربران پاک شوند؟',
    okText: 'پاک‌کن',
    cancelText: 'انصراف',
    okType: 'danger',
    async onOk() {
      try {
        await clearClientErrors()
        await load(false)
        message.success('فهرست خطاها پاک شد')
      } catch (e) {
        admin.toastError(e, 'پاک نشد')
      }
    },
  })
}
</script>

<template>
  <section class="card card-pad">
    <div class="row-wrap head">
      <span class="section-title">خطاهای ارسال کاربران</span>
      <div class="grow" />
      <a-button size="small" :loading="loading" @click="load(false)">
        <template #icon><AppIcon name="refresh" :size="13" /></template>به‌روزرسانی
      </a-button>
      <a-button v-if="rows.length" size="small" danger @click="clearAll">
        <template #icon><AppIcon name="trash" :size="13" /></template>پاک‌کردن همه
      </a-button>
    </div>

    <div v-if="!rows.length && !loading" class="empty">
      <AppIcon name="check-circle" :size="28" />
      <p>هیچ خطای ارسالی ثبت نشده است</p>
    </div>

    <div v-else class="scroll-x">
      <table class="err-table">
        <thead>
          <tr>
            <th>زمان</th><th>کاربر</th><th>بخش</th><th>کد خطا</th><th>پیام</th><th></th>
          </tr>
        </thead>
        <tbody>
          <template v-for="e in rows" :key="e.id">
            <tr :class="{ unseen: !e.seen }">
              <td class="num">{{ fmtTime(e.createdAt) }}</td>
              <td>
                <span v-if="e.publicId" class="num pid">{{ e.publicId }}</span>
                {{ e.name || 'ناشناس' }}
              </td>
              <td>{{ CONTEXT_LABELS[e.context] || e.context || '—' }}</td>
              <td><span class="num code">{{ e.errorCode || '—' }}</span></td>
              <td class="msg">{{ e.errorMessage || '—' }}</td>
              <td class="actions">
                <a-button v-if="e.codeExcerpt" size="small" @click="toggle(e.id)">کد</a-button>
                <a-button size="small" danger @click="removeOne(e.id)">حذف</a-button>
              </td>
            </tr>
            <tr v-if="expanded.has(e.id) && e.codeExcerpt" class="code-row">
              <td colspan="6"><pre class="code-preview" dir="ltr">{{ e.codeExcerpt }}</pre></td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.head {
  gap: 8px;
  margin-bottom: var(--space-4);
}
.empty {
  padding: 40px;
  text-align: center;
  color: var(--text-faint);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}
.err-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8rem;
  min-width: 720px;
}
.err-table th {
  text-align: start;
  padding: 9px 10px;
  color: var(--text-faint);
  font-weight: 600;
  border-bottom: 1px solid var(--border);
}
.err-table td {
  padding: 10px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.err-table tr.unseen td {
  background: var(--info-soft);
}
.pid {
  font-size: 0.72rem;
  color: var(--text-faint);
  margin-inline-end: 4px;
}
.code {
  font-size: 0.75rem;
  padding: 2px 6px;
  background: var(--surface-2);
  border-radius: 6px;
}
.msg {
  max-width: 340px;
}
.actions {
  display: flex;
  gap: 6px;
  white-space: nowrap;
}
.code-preview {
  margin: 0;
  background: var(--code-bg);
  color: var(--code-text);
  border-radius: var(--radius-sm);
  padding: 12px;
  font-family: var(--font-mono);
  font-size: 0.76rem;
  line-height: 1.6;
  max-height: 300px;
  overflow: auto;
  white-space: pre;
}
</style>
