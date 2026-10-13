<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { storeToRefs } from 'pinia'
import { useAdminStore } from '@/stores/admin'
import { deleteParticipant, updateParticipant } from '@/api/client'
import type { Participant } from '@/api/types'
import { toFa, fmtPhone, fmtTime, digitsOnly } from '@/lib/fa'
import StatusPill from '@/components/StatusPill.vue'
import Chip from '@/components/Chip.vue'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const { participants } = storeToRefs(admin)

const search = ref('')
const statusFilter = ref<string | undefined>(undefined)
const selectedKeys = ref<string[]>([])
const refreshing = ref(false)

async function manualRefresh() {
  refreshing.value = true
  try {
    await admin.refreshParticipants()
    message.success('فهرست شرکت‌کنندگان به‌روزرسانی شد')
  } finally {
    refreshing.value = false
  }
}

const rowSelection = computed(() => ({
  selectedRowKeys: selectedKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedKeys.value = keys.map(String)
  },
}))

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  const qDigits = digitsOnly(search.value)
  return participants.value
    .filter((u) => {
      if (statusFilter.value && u.status !== statusFilter.value) return false
      if (!q) return true
      const name = `${u.firstName} ${u.lastName}`.toLowerCase()
      return (
        name.includes(q) ||
        u.publicId.toLowerCase().includes(q) ||
        (!!qDigits && u.phone.includes(qDigits))
      )
    })
    .slice()
    .reverse()
})

const columns = [
  { title: 'شناسه', key: 'publicId', width: 96 },
  { title: 'نام', key: 'name' },
  { title: 'موبایل', key: 'phone', width: 150 },
  { title: 'زمان ثبت‌نام', key: 'time', width: 90 },
  { title: 'وضعیت', key: 'status', width: 110 },
  { title: 'امتیاز', key: 'score', width: 80 },
  { title: 'زمان اجرا', key: 'exec', width: 110 },
  { title: '٪ هوش مصنوعی', key: 'ai', width: 130 },
  { title: '', key: 'actions', width: 150 },
]

function aiTone(pct: number): 'success' | 'warning' | 'danger' {
  return pct >= 65 ? 'danger' : pct >= 35 ? 'warning' : 'success'
}

/* ---- edit name ---- */
const editModal = reactive({ open: false, id: '', first: '', last: '', saving: false })
function openEdit(u: Participant) {
  editModal.open = true
  editModal.id = u.id
  editModal.first = u.firstName
  editModal.last = u.lastName
}
async function saveEdit() {
  if (!editModal.first.trim() || !editModal.last.trim())
    return message.error('نام و نام‌خانوادگی نمی‌توانند خالی باشند')
  editModal.saving = true
  try {
    await updateParticipant(editModal.id, {
      firstName: editModal.first.trim(),
      lastName: editModal.last.trim(),
    })
    await admin.refreshParticipants()
    editModal.open = false
    message.success('نام شرکت‌کننده به‌روزرسانی شد')
  } catch (e) {
    admin.toastError(e, 'خطا در ویرایش نام')
  } finally {
    editModal.saving = false
  }
}

/* ---- delete ---- */
function confirmDelete(ids: string[]) {
  const targets = participants.value.filter((u) => ids.includes(u.id))
  if (!targets.length) return
  Modal.confirm({
    title: targets.length === 1 ? 'حذف شرکت‌کننده' : `حذف ${toFa(targets.length)} شرکت‌کننده`,
    content:
      targets.length === 1
        ? `«${targets[0].firstName} ${targets[0].lastName}» (${targets[0].publicId}) برای همیشه حذف می‌شود.`
        : `${targets.map((u) => `${u.firstName} ${u.lastName}`).join('، ')} حذف می‌شوند.`,
    okText: 'حذف',
    cancelText: 'انصراف',
    okType: 'danger',
    async onOk() {
      const res = await Promise.allSettled(ids.map((id) => deleteParticipant(id)))
      const ok = res.filter((r) => r.status === 'fulfilled').length
      const fail = res.length - ok
      selectedKeys.value = selectedKeys.value.filter((k) => !ids.includes(k))
      await admin.refreshParticipants()
      if (ok) message.success(`${toFa(ok)} شرکت‌کننده حذف شد`)
      if (fail) message.error(`${toFa(fail)} مورد حذف نشد`)
    },
  })
}
</script>

<template>
  <div class="stack">
    <section class="card card-pad">
      <div class="toolbar">
        <a-input
          v-model:value="search"
          placeholder="جست‌وجو: نام، موبایل یا شناسه"
          allow-clear
          style="max-width: 320px"
        >
          <template #prefix><AppIcon name="search" :size="15" /></template>
        </a-input>
        <a-select
          v-model:value="statusFilter"
          placeholder="همه وضعیت‌ها"
          allow-clear
          style="width: 170px"
          :options="[
            { value: 'registered', label: 'ثبت‌نام‌شده' },
            { value: 'started', label: 'در حال حل' },
            { value: 'submitted', label: 'ارسال‌شده' },
          ]"
        />
        <div class="grow" />
        <span class="faint count-note">{{ toFa(filtered.length) }} از {{ toFa(participants.length) }}</span>
        <a-button size="small" :loading="refreshing" @click="manualRefresh">
          <template #icon><AppIcon name="refresh" :size="13" /></template>
          به‌روزرسانی
        </a-button>
        <a-button v-if="selectedKeys.length" danger @click="confirmDelete(selectedKeys)">
          <template #icon><AppIcon name="trash" :size="14" /></template>
          حذف {{ toFa(selectedKeys.length) }} مورد انتخاب‌شده
        </a-button>
      </div>

      <a-table
        :columns="columns"
        :data-source="filtered"
        row-key="id"
        size="small"
        :pagination="{ pageSize: 15, hideOnSinglePage: true }"
        :row-selection="rowSelection"
        :scroll="{ x: 900 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'publicId'"><span class="num">{{ record.publicId }}</span></template>
          <template v-else-if="column.key === 'name'">{{ record.firstName }} {{ record.lastName }}</template>
          <template v-else-if="column.key === 'phone'"><span class="num">{{ fmtPhone(record.phone) }}</span></template>
          <template v-else-if="column.key === 'time'"><span class="num">{{ fmtTime(record.registeredAt) }}</span></template>
          <template v-else-if="column.key === 'status'"><StatusPill :status="record.status" /></template>
          <template v-else-if="column.key === 'score'">
            <span class="num">{{ record.score != null ? toFa(record.score) : '—' }}</span>
          </template>
          <template v-else-if="column.key === 'exec'">
            <span
              class="num"
              :title="record.execMs != null ? `عمق تودرتویی حلقه‌ها: ${toFa(record.execDepth || 0)} — تخمین ایستا` : ''"
            >{{ record.execMs != null ? toFa(record.execMs) + ' ms' : '—' }}</span>
          </template>
          <template v-else-if="column.key === 'ai'">
            <Chip v-if="record.aiScore != null" :tone="aiTone(record.aiScore)" dot>
              هوش مصنوعی ٪{{ toFa(record.aiScore) }}
            </Chip>
            <span v-else>—</span>
          </template>
          <template v-else-if="column.key === 'actions'">
            <div class="row-actions">
              <a-button size="small" @click="openEdit(record as Participant)">
                <template #icon><AppIcon name="pencil" :size="13" /></template>ویرایش
              </a-button>
              <a-button size="small" danger @click="confirmDelete([record.id])">
                <template #icon><AppIcon name="trash" :size="13" /></template>حذف
              </a-button>
            </div>
          </template>
        </template>
        <template #emptyText>
          <div class="empty">شرکت‌کننده‌ای مطابق فیلتر پیدا نشد</div>
        </template>
      </a-table>
    </section>

    <a-modal
      v-model:open="editModal.open"
      title="ویرایش نام شرکت‌کننده"
      :confirm-loading="editModal.saving"
      ok-text="ذخیره"
      cancel-text="انصراف"
      @ok="saveEdit"
    >
      <div class="stack">
        <label class="f"><span>نام</span><a-input v-model:value="editModal.first" /></label>
        <label class="f"><span>نام خانوادگی</span><a-input v-model:value="editModal.last" /></label>
      </div>
    </a-modal>
  </div>
</template>

<style scoped>
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
  white-space: nowrap;
}
.count-note {
  font-size: 0.74rem;
}
.empty {
  padding: 24px;
  color: var(--text-faint);
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
</style>
