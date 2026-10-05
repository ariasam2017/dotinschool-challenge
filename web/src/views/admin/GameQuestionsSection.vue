<script setup lang="ts">
// Quiz question bank: author four-option questions, bulk import/export
// (JSON or Excel CSV), and load the built-in technology bank. Every player is
// dealt a random subset, so a bigger bank means fewer repeats between players.
import { ref, reactive, computed, onMounted } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import {
  listGameQuestions,
  createGameQuestion,
  updateGameQuestion,
  deleteGameQuestions,
  importGameQuestions,
  addDefaultGameQuestions,
  type GameQuestion,
  type GameQuestionInput,
} from '@/api/game'
import { toFa } from '@/lib/fa'
import Chip from '@/components/Chip.vue'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const list = ref<GameQuestion[]>([])
const loading = ref(false)
const LETTERS = ['الف', 'ب', 'ج', 'د']

async function load() {
  loading.value = true
  try {
    list.value = await listGameQuestions()
  } catch (e) {
    admin.toastError(e, 'دریافت سوالات ناموفق بود')
  } finally {
    loading.value = false
  }
}
onMounted(load)

/* ---------------- filters ---------------- */
const search = ref('')
const category = ref<string | undefined>(undefined)
const categories = computed(() => [...new Set(list.value.map((q) => q.category))].sort())
const filtered = computed(() => {
  const s = search.value.trim().toLowerCase()
  return list.value.filter((q) => {
    if (category.value && q.category !== category.value) return false
    if (!s) return true
    return q.text.toLowerCase().includes(s) || q.options.some((o) => o.toLowerCase().includes(s))
  })
})
const selected = ref<string[]>([])
const rowSelection = computed(() => ({
  selectedRowKeys: selected.value,
  onChange: (keys: (string | number)[]) => (selected.value = keys.map(String)),
}))

const columns = [
  { title: 'سوال', key: 'text' },
  { title: 'گزینه‌ها', key: 'options', width: 380 },
  { title: 'دسته', key: 'category', width: 120 },
  { title: '', key: 'actions', width: 96 },
]

/* ---------------- editor ---------------- */
const editor = reactive({
  open: false,
  saving: false,
  id: '' as string,
  category: 'تکنولوژی',
  text: '',
  options: ['', '', '', ''] as string[],
  correct: 0,
})
function openNew() {
  Object.assign(editor, { open: true, id: '', text: '', options: ['', '', '', ''], correct: 0 })
  if (category.value) editor.category = category.value
}
function openEdit(q: GameQuestion) {
  Object.assign(editor, {
    open: true,
    id: q.id,
    category: q.category,
    text: q.text,
    options: [...q.options],
    correct: q.correct,
  })
}
async function saveEditor(another = false) {
  const payload: GameQuestionInput = {
    category: editor.category.trim(),
    text: editor.text.trim(),
    options: editor.options.map((o) => o.trim()) as GameQuestionInput['options'],
    correct: editor.correct,
  }
  if (!payload.text) return message.error('متن سوال را بنویسید')
  if (payload.options.some((o) => !o)) return message.error('هر چهار گزینه را پر کنید')
  editor.saving = true
  try {
    if (editor.id) await updateGameQuestion(editor.id, payload)
    else await createGameQuestion(payload)
    message.success(editor.id ? 'سوال ویرایش شد' : 'سوال اضافه شد')
    await load()
    if (another && !editor.id) {
      Object.assign(editor, { text: '', options: ['', '', '', ''], correct: 0 })
    } else {
      editor.open = false
    }
  } catch (e) {
    admin.toastError(e, 'ذخیره‌ی سوال ناموفق بود')
  } finally {
    editor.saving = false
  }
}

/* ---------------- delete ---------------- */
function confirmDelete(ids: string[]) {
  if (!ids.length) return
  Modal.confirm({
    title: ids.length === 1 ? 'حذف سوال' : `حذف ${toFa(ids.length)} سوال`,
    content: 'سوال از بانک حذف می‌شود. بازی‌هایی که قبلاً انجام شده‌اند تغییری نمی‌کنند.',
    okText: 'حذف',
    cancelText: 'انصراف',
    okType: 'danger',
    async onOk() {
      try {
        const r = await deleteGameQuestions(ids)
        selected.value = selected.value.filter((k) => !ids.includes(k))
        await load()
        message.success(`${toFa(r.deleted)} سوال حذف شد`)
      } catch (e) {
        admin.toastError(e, 'حذف ناموفق بود')
      }
    },
  })
}

/* ---------------- built-in bank ---------------- */
const addingDefaults = ref(false)
async function addDefaults() {
  addingDefaults.value = true
  try {
    const r = await addDefaultGameQuestions()
    await load()
    message.success(r.added ? `${toFa(r.added)} سوال تکنولوژی به بانک اضافه شد` : 'همه‌ی سوالات پیش‌فرض از قبل در بانک هستند')
  } catch (e) {
    admin.toastError(e, 'انجام نشد')
  } finally {
    addingDefaults.value = false
  }
}

/* ---------------- import / export ---------------- */
const fileInput = ref<HTMLInputElement | null>(null)
const importState = reactive({ open: false, saving: false, replace: false, rows: [] as GameQuestionInput[], name: '' })

function download(name: string, body: string, type: string) {
  const a = document.createElement('a')
  a.href = URL.createObjectURL(new Blob([body], { type }))
  a.download = name
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 2000)
}
function exportJson() {
  const rows = list.value.map(({ category, text, options, correct }) => ({ category, text, options, correct }))
  download(`game-questions-${new Date().toISOString().slice(0, 10)}.json`, JSON.stringify({ questions: rows }, null, 2), 'application/json')
}
function csvTemplate() {
  const body =
    '﻿سوال,گزینه ۱,گزینه ۲,گزینه ۳,گزینه ۴,شماره گزینه درست (۱ تا ۴),دسته\r\n' +
    '"کدام شرکت سازنده‌ی اندروید است؟",اپل,گوگل,سامسونگ,مایکروسافت,2,تکنولوژی\r\n'
  download('game-questions-template.csv', body, 'text/csv;charset=utf-8')
}

/** Minimal RFC-4180 CSV parser (quotes, escaped quotes, CRLF). */
function parseCsv(text: string): string[][] {
  const rows: string[][] = []
  let row: string[] = []
  let cell = ''
  let quoted = false
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (quoted) {
      if (ch === '"' && text[i + 1] === '"') {
        cell += '"'
        i++
      } else if (ch === '"') quoted = false
      else cell += ch
    } else if (ch === '"') quoted = true
    else if (ch === ',' || ch === '\t') {
      row.push(cell)
      cell = ''
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && text[i + 1] === '\n') i++
      row.push(cell)
      rows.push(row)
      row = []
      cell = ''
    } else cell += ch
  }
  if (cell || row.length) {
    row.push(cell)
    rows.push(row)
  }
  return rows.filter((r) => r.some((c) => c.trim()))
}

const faToEn = (s: string) => s.replace(/[۰-۹]/g, (d) => String('۰۱۲۳۴۵۶۷۸۹'.indexOf(d)))

function fromCsv(text: string): GameQuestionInput[] {
  const rows = parseCsv(text.replace(/^﻿/, ''))
  // skip a header row (its 6th column is not a 1..4 number)
  if (rows.length && !/^[1-4]$/.test(faToEn((rows[0][5] || '').trim()))) rows.shift()
  return rows.map((r, i) => {
    const n = Number(faToEn((r[5] || '').trim()))
    if (!(n >= 1 && n <= 4)) throw new Error(`ردیف ${toFa(i + 1)}: شماره‌ی گزینه‌ی درست باید ۱ تا ۴ باشد`)
    return {
      text: (r[0] || '').trim(),
      options: [r[1], r[2], r[3], r[4]].map((o) => (o || '').trim()) as GameQuestionInput['options'],
      correct: n - 1,
      category: (r[6] || '').trim() || 'تکنولوژی',
    }
  })
}

function fromJson(text: string): GameQuestionInput[] {
  const data = JSON.parse(text)
  const arr = Array.isArray(data) ? data : data?.questions
  if (!Array.isArray(arr)) throw new Error('ساختار فایل JSON معتبر نیست')
  return arr.map((q: any, i: number) => {
    if (!q || typeof q.text !== 'string' || !Array.isArray(q.options) || q.options.length !== 4)
      throw new Error(`سوال ${toFa(i + 1)}: باید text و چهار options داشته باشد`)
    return {
      text: q.text,
      options: q.options.map((o: unknown) => String(o ?? '')) as GameQuestionInput['options'],
      correct: Number(q.correct) || 0,
      category: typeof q.category === 'string' ? q.category : 'تکنولوژی',
    }
  })
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  try {
    const text = await file.text()
    const rows = /\.json$/i.test(file.name) ? fromJson(text) : fromCsv(text)
    if (!rows.length) throw new Error('فایل هیچ سوالی ندارد')
    Object.assign(importState, { open: true, rows, name: file.name, replace: false })
  } catch (err) {
    message.error(err instanceof Error ? err.message : 'خواندن فایل ناموفق بود')
  }
}

async function runImport() {
  importState.saving = true
  try {
    const r = await importGameQuestions(importState.rows, importState.replace)
    importState.open = false
    await load()
    message.success(
      `${toFa(r.added)} سوال اضافه شد` + (r.skipped ? ` — ${toFa(r.skipped)} سوال تکراری نادیده گرفته شد` : ''),
    )
  } catch (e) {
    admin.toastError(e, 'ورود سوالات ناموفق بود')
  } finally {
    importState.saving = false
  }
}
</script>

<template>
  <div class="stack">
    <section class="card card-pad">
      <div class="card-hd">
        <span class="section-title grow">
          بانک سوالات بازی — <span class="num">{{ toFa(list.length) }}</span> سوال
        </span>
        <a-button :loading="addingDefaults" @click="addDefaults">
          <template #icon><AppIcon name="database" :size="14" /></template>
          افزودن بانک سوالات تکنولوژی
        </a-button>
        <a-button type="primary" @click="openNew">
          <template #icon><AppIcon name="plus" :size="14" /></template>
          سوال جدید
        </a-button>
      </div>

      <div class="toolbar">
        <a-input v-model:value="search" placeholder="جست‌وجو در سوال و گزینه‌ها" allow-clear style="max-width: 300px">
          <template #prefix><AppIcon name="search" :size="15" /></template>
        </a-input>
        <a-select
          v-model:value="category"
          placeholder="همه‌ی دسته‌ها"
          allow-clear
          style="width: 160px"
          :options="categories.map((c) => ({ value: c, label: c }))"
        />
        <div class="grow" />
        <span class="faint small">{{ toFa(filtered.length) }} سوال</span>
        <a-dropdown>
          <a-button size="small">
            <template #icon><AppIcon name="upload" :size="13" /></template>
            ورود / خروج
          </a-button>
          <template #overlay>
            <a-menu>
              <a-menu-item key="import" @click="fileInput?.click()">ورود از فایل (Excel CSV یا JSON)</a-menu-item>
              <a-menu-item key="tpl" @click="csvTemplate">دریافت نمونه‌ی فایل CSV</a-menu-item>
              <a-menu-item key="export" :disabled="!list.length" @click="exportJson">خروجی JSON همه‌ی سوالات</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
        <a-button v-if="selected.length" size="small" danger @click="confirmDelete(selected)">
          <template #icon><AppIcon name="trash" :size="13" /></template>
          حذف {{ toFa(selected.length) }} سوال
        </a-button>
        <input ref="fileInput" type="file" accept=".csv,.json,.txt,text/csv,application/json" hidden @change="onFile" />
      </div>

      <a-table
        :columns="columns"
        :data-source="filtered"
        :loading="loading"
        row-key="id"
        size="small"
        :row-selection="rowSelection"
        :pagination="{ pageSize: 15, hideOnSinglePage: true, showSizeChanger: false }"
        :scroll="{ x: 860 }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'text'">
            <span class="q-text">{{ record.text }}</span>
          </template>
          <template v-else-if="column.key === 'options'">
            <div class="opts">
              <span v-for="(o, i) in record.options" :key="i" class="opt" :class="{ right: i === record.correct }">
                <AppIcon v-if="i === record.correct" name="check" :size="12" />{{ o }}
              </span>
            </div>
          </template>
          <template v-else-if="column.key === 'category'"><Chip tone="info">{{ record.category }}</Chip></template>
          <template v-else-if="column.key === 'actions'">
            <div class="row-actions">
              <a-button size="small" @click="openEdit(record as GameQuestion)">
                <template #icon><AppIcon name="pencil" :size="13" /></template>
              </a-button>
              <a-button size="small" danger @click="confirmDelete([record.id])">
                <template #icon><AppIcon name="trash" :size="13" /></template>
              </a-button>
            </div>
          </template>
        </template>
        <template #emptyText>
          <div class="empty">
            سوالی پیدا نشد. با «سوال جدید» یا «افزودن بانک سوالات تکنولوژی» شروع کنید.
          </div>
        </template>
      </a-table>
    </section>

    <!-- editor -->
    <a-modal v-model:open="editor.open" :title="editor.id ? 'ویرایش سوال' : 'سوال جدید'" width="640px">
      <div class="stack">
        <label class="f">
          <span>دسته</span>
          <a-auto-complete
            v-model:value="editor.category"
            :options="categories.map((c) => ({ value: c }))"
            placeholder="مثلاً تکنولوژی"
          />
        </label>
        <label class="f">
          <span>متن سوال</span>
          <a-textarea v-model:value="editor.text" :rows="3" :maxlength="600" show-count placeholder="سوال را بنویسید…" />
        </label>
        <div class="f">
          <span>گزینه‌ها — گزینه‌ی درست را با دایره‌ی کنارش مشخص کنید</span>
          <a-radio-group v-model:value="editor.correct" class="opt-editor">
            <div v-for="i in 4" :key="i" class="opt-row" :class="{ right: editor.correct === i - 1 }">
              <a-radio :value="i - 1" />
              <span class="opt-letter">{{ LETTERS[i - 1] }}</span>
              <a-input v-model:value="editor.options[i - 1]" :maxlength="200" :placeholder="`گزینه‌ی ${LETTERS[i - 1]}`" />
            </div>
          </a-radio-group>
        </div>
        <p class="faint small">ترتیب گزینه‌ها برای هر بازیکن به‌صورت تصادفی جابه‌جا می‌شود.</p>
      </div>
      <template #footer>
        <a-button @click="editor.open = false">انصراف</a-button>
        <a-button v-if="!editor.id" :loading="editor.saving" @click="saveEditor(true)">ذخیره و سوال بعدی</a-button>
        <a-button type="primary" :loading="editor.saving" @click="saveEditor()">ذخیره</a-button>
      </template>
    </a-modal>

    <!-- import preview -->
    <a-modal
      v-model:open="importState.open"
      title="ورود سوالات از فایل"
      ok-text="ورود سوالات"
      cancel-text="انصراف"
      :confirm-loading="importState.saving"
      width="640px"
      @ok="runImport"
    >
      <div class="stack">
        <p>
          فایل <b>{{ importState.name }}</b> شامل <b class="num">{{ toFa(importState.rows.length) }}</b> سوال است.
          سوالاتی که متنشان از قبل در بانک هست، تکراری حساب شده و اضافه نمی‌شوند.
        </p>
        <a-checkbox v-model:checked="importState.replace">
          جایگزینی کامل بانک (همه‌ی سوالات فعلی حذف شوند)
        </a-checkbox>
        <div class="preview">
          <div v-for="(q, i) in importState.rows.slice(0, 5)" :key="i" class="preview-item">
            <b>{{ toFa(i + 1) }}. {{ q.text }}</b>
            <span class="faint small">درست: {{ q.options[q.correct] }}</span>
          </div>
          <span v-if="importState.rows.length > 5" class="faint small">و {{ toFa(importState.rows.length - 5) }} سوال دیگر…</span>
        </div>
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
.small {
  font-size: 0.78rem;
}
.q-text {
  font-weight: 600;
}
.opts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
}
.opt {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--surface-2);
  font-size: 0.78rem;
  overflow-wrap: anywhere;
}
.opt.right {
  background: var(--success-soft);
  color: var(--success);
  font-weight: 700;
}
.row-actions {
  display: flex;
  gap: 6px;
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
.opt-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
.opt-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
}
.opt-row.right {
  border-color: var(--success);
  background: var(--success-soft);
}
.opt-row :deep(.ant-radio-wrapper) {
  margin-inline-end: 0;
}
.opt-letter {
  width: 28px;
  flex: none;
  font-weight: 700;
  color: var(--text-dim);
  font-size: 0.8rem;
}
.preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-2);
}
.preview-item {
  display: flex;
  flex-direction: column;
}
</style>
