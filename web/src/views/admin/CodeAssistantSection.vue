<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import {
  getAiSettings,
  saveAiSettings,
  testAiProvider,
  detectAiProvider,
  aiStatus,
  reviewCode,
} from '@/api/client'
import type { AiPreset } from '@/api/types'
import { toFa } from '@/lib/fa'
import Gauge from '@/components/Gauge.vue'
import MarkdownLite from '@/components/MarkdownLite.vue'
import Chip from '@/components/Chip.vue'
import AppIcon from '@/components/AppIcon.vue'
import JavaEditor from '@/components/JavaEditor.vue'

const admin = useAdminStore()

interface Row {
  id: string
  label: string
  baseUrl: string
  model: string
  apiKey: string // freshly typed; '' means "keep stored"
  keyMask: string
  hasKey: boolean
  detectMsg: string
}

const rows = ref<Row[]>([])
const activeId = ref('')
const presets = ref<AiPreset[]>([])
const envNote = ref('')
const savedIds = ref<Set<string>>(new Set())
const saving = ref(false)

const status = reactive({ configured: false, label: '', model: '' })

const SIGNATURES: { re: RegExp; label: string; baseUrl: string; model: string; sure: boolean }[] = [
  { re: /^sk-or-/, label: 'OpenRouter', baseUrl: 'https://openrouter.ai/api/v1', model: 'meta-llama/llama-3.1-8b-instruct:free', sure: true },
  { re: /^gsk_[A-Za-z0-9]/, label: 'Groq', baseUrl: 'https://api.groq.com/openai/v1', model: 'llama-3.3-70b-versatile', sure: true },
  { re: /^AIza[0-9A-Za-z_-]{10,}$/, label: 'Gemini (سازگار با OpenAI)', baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai', model: 'gemini-2.0-flash', sure: true },
  { re: /^sk-ant-/, label: 'Anthropic (سازگار با OpenAI)', baseUrl: 'https://api.anthropic.com/v1', model: 'claude-3-5-haiku-latest', sure: true },
  { re: /^sk-(proj|svcacct)-/, label: 'OpenAI (ChatGPT)', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini', sure: true },
  { re: /^sk-[A-Za-z0-9_-]{20,}$/, label: 'OpenAI (ChatGPT)', baseUrl: 'https://api.openai.com/v1', model: 'gpt-4o-mini', sure: false },
]

function newId() {
  return typeof crypto?.randomUUID === 'function'
    ? crypto.randomUUID()
    : 'new-' + Date.now() + '-' + Math.random().toString(16).slice(2)
}

async function reload() {
  try {
    const data = await getAiSettings()
    presets.value = data.presets || []
    activeId.value = data.activeId
    savedIds.value = new Set(data.providers.map((p) => p.id))
    rows.value = data.providers.map((p) => ({
      id: p.id,
      label: p.label,
      baseUrl: p.baseUrl,
      model: p.model,
      apiKey: '',
      keyMask: p.keyMask,
      hasKey: p.hasKey,
      detectMsg: '',
    }))
    envNote.value =
      data.env.present && !data.providers.length
        ? `یک کلید از فایل .env هم فعال است (${data.env.model}) — تا وقتی این‌جا سرویسی ثبت نکنید از همان استفاده می‌شود.`
        : ''
  } catch (e) {
    admin.toastError(e, 'خطا در بارگذاری سرویس‌ها')
  }
}
async function refreshStatus() {
  try {
    const st = await aiStatus()
    status.configured = st.configured
    status.label = st.label || ''
    status.model = st.model
  } catch (e) {
    admin.handleAuthError(e)
  }
}

onMounted(() => {
  void reload()
  void refreshStatus()
})

function addRow() {
  if (rows.value.length >= 20) return message.error('حداکثر ۲۰ سرویس می‌توانید ثبت کنید')
  const id = newId()
  rows.value.push({ id, label: '', baseUrl: '', model: '', apiKey: '', keyMask: '', hasKey: false, detectMsg: '' })
  if (rows.value.length === 1) activeId.value = id
}
function removeRow(id: string) {
  rows.value = rows.value.filter((r) => r.id !== id)
  if (activeId.value === id && rows.value.length) activeId.value = rows.value[0].id
  message.info('سرویس حذف شد — برای اعمال «ذخیره سرویس‌ها» را بزنید')
}
function applyPreset(row: Row) {
  const p = presets.value.find((x) => x.label === row.label.trim())
  if (!p) return
  if (!row.baseUrl.trim()) row.baseUrl = p.baseUrl
  if (!row.model.trim()) row.model = p.model
}

const detectTimers: Record<string, number> = {}
function onKeyInput(row: Row) {
  const k = row.apiKey.trim()
  if (k.length < 12 || k.startsWith('•')) return
  const sig = SIGNATURES.find((s) => s.re.test(k))
  if (sig) {
    if (sig.sure || !row.label) row.label = sig.label
    if (sig.sure || !row.baseUrl) row.baseUrl = sig.baseUrl
    if (sig.sure || !row.model) row.model = sig.model
  }
  row.detectMsg = 'در حال تشخیص سرویسِ این کلید…'
  clearTimeout(detectTimers[row.id])
  detectTimers[row.id] = window.setTimeout(async () => {
    try {
      const d = await detectAiProvider(k)
      if (row.apiKey.trim() !== k) return
      if (d.match) {
        row.baseUrl = d.match.baseUrl
        if (!row.model) row.model = d.match.model
        if (d.method === 'probe' || !row.label) row.label = d.match.label
        row.detectMsg =
          (d.method === 'probe' ? 'با تست واقعی شناسایی شد: ' : 'شناسایی شد: ') +
          d.match.label +
          ' — حالا «ذخیره سرویس‌ها» را بزنید'
      } else if (d.reason === 'rejected-everywhere') {
        row.detectMsg = 'این کلید را نه OpenAI، نه GapGPT و نه DeepSeek قبول نکردند — کلید اشتباه است یا حسابش شارژ ندارد.'
      } else {
        row.detectMsg = 'تشخیص خودکار نشد — «آدرس پایه» را دستی وارد کنید.'
      }
    } catch (e) {
      admin.handleAuthError(e)
      row.detectMsg = ''
    }
  }, 700)
}

async function save() {
  const uname = (admin.username || '').trim().toLowerCase()
  if (uname && rows.value.some((r) => r.model.trim().toLowerCase() === uname)) {
    message.error(
      '«نام مدل» با نام کاربری پنل یکی است — مرورگر آن را خودکار پر کرده. نام مدل درست را وارد کنید (مثل gpt-4o-mini) یا این فیلد را خالی بگذارید.',
    )
    return
  }
  saving.value = true
  try {
    const payload = {
      activeId: activeId.value,
      providers: rows.value.map((r) => ({
        id: r.id,
        label: r.label.trim(),
        baseUrl: r.baseUrl.trim(),
        model: r.model.trim(),
        ...(r.apiKey.trim() ? { apiKey: r.apiKey.trim() } : {}),
      })),
    }
    const saved = await saveAiSettings(payload)
    activeId.value = saved.activeId
    savedIds.value = new Set(saved.providers.map((p) => p.id))
    rows.value = saved.providers.map((p) => ({
      id: p.id, label: p.label, baseUrl: p.baseUrl, model: p.model,
      apiKey: '', keyMask: p.keyMask, hasKey: p.hasKey, detectMsg: '',
    }))
    message.success('سرویس‌های هوش مصنوعی ذخیره شد')
    await refreshStatus()
  } catch (e) {
    admin.toastError(e, 'خطا در ذخیره‌ی سرویس‌ها')
  } finally {
    saving.value = false
  }
}

const testingId = ref('')
async function test(row: Row) {
  if (row.apiKey.trim() || !savedIds.value.has(row.id)) {
    message.info('برای تست، اول «ذخیره سرویس‌ها» را بزنید')
    return
  }
  testingId.value = row.id
  try {
    const r = await testAiProvider(row.id)
    message.success(`اتصال موفق — پاسخ مدل ${r.model}: «${r.sample}»`)
  } catch (e) {
    admin.toastError(e, 'تست ناموفق بود')
  } finally {
    testingId.value = ''
  }
}

/* ---- code review tool ---- */
const crCode = ref('')
const crFilename = ref('Main.java')
const crRunning = ref(false)
const crReport = ref('')
const crScore = ref<number | null>(null)
const crMeta = ref('')
const crFileInput = ref<HTMLInputElement>()

function crColor(pct: number): string {
  return pct >= 65 ? 'var(--danger)' : pct >= 35 ? 'var(--warning)' : 'var(--success)'
}
function onJavaFile(ev: Event) {
  const file = (ev.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 200000) {
    message.error('حجم فایل بیش از حد است (سقف ۲۰۰ کیلوبایت)')
    ;(ev.target as HTMLInputElement).value = ''
    return
  }
  const reader = new FileReader()
  reader.onerror = () => message.error('خطا در خواندن فایل')
  reader.onload = () => {
    crCode.value = String(reader.result || '')
    crFilename.value = file.name || 'Main.java'
    crMeta.value = 'فایل بارگذاری شد: ' + crFilename.value
  }
  reader.readAsText(file)
}
async function runReview() {
  if (!crCode.value.trim()) return message.error('ابتدا یک فایل .java انتخاب کنید یا کد را وارد کنید')
  crRunning.value = true
  crReport.value = ''
  const t0 = Date.now()
  try {
    const { review, aiScore, model, usage } = await reviewCode(crFilename.value || 'Main.java', crCode.value)
    crReport.value = review
    crScore.value = aiScore
    const secs = Math.max(1, Math.round((Date.now() - t0) / 1000))
    crMeta.value =
      `مدل: ${model || '—'} · ${toFa(secs)} ثانیه` +
      (usage?.total_tokens ? ` · ${toFa(usage.total_tokens)} توکن` : '')
    message.success('بررسی کد کامل شد')
  } catch (e) {
    admin.toastError(e, 'بررسی کد ناموفق بود')
  } finally {
    crRunning.value = false
  }
}
function clearReview() {
  crCode.value = ''
  crFilename.value = 'Main.java'
  crMeta.value = ''
  crReport.value = ''
  crScore.value = null
}

const presetOptions = computed(() => presets.value.map((p) => ({ value: p.label })))
</script>

<template>
  <div class="stack">
    <section class="card card-pad">
      <div class="row-wrap head-row">
        <span class="section-title">سرویس‌های هوش مصنوعی</span>
        <Chip v-if="status.configured" tone="success" dot>
          سرویس فعال: {{ status.label ? status.label + ' · ' : '' }}{{ status.model }}
        </Chip>
        <Chip v-else tone="danger" dot>هیچ سرویس فعالی تنظیم نشده</Chip>
      </div>

      <a-alert
        v-if="!status.configured"
        type="warning"
        show-icon
        banner
        message="یک سرویس اضافه کنید، کلید API را وارد کنید، «ذخیره سرویس‌ها» را بزنید و آن را «فعال» کنید."
        style="margin-bottom: 12px"
      />
      <p v-if="envNote" class="faint hint">{{ envNote }}</p>

      <!-- soaks up browser password-manager autofill so the saved login
           username never lands in «نام مدل» / «کلید API» below -->
      <div class="af-trap" aria-hidden="true">
        <input type="text" tabindex="-1" autocomplete="username" />
        <input type="password" tabindex="-1" autocomplete="current-password" />
      </div>

      <a-radio-group v-model:value="activeId" class="prov-list">
        <div v-for="row in rows" :key="row.id" class="prov card">
          <a-radio :value="row.id" class="prov-active">فعال</a-radio>
          <div class="prov-grid">
            <label class="f"><span>نام سرویس</span>
              <a-auto-complete v-model:value="row.label" :options="presetOptions" @change="applyPreset(row)">
                <a-input placeholder="مثلاً GapGPT" autocomplete="off" />
              </a-auto-complete>
            </label>
            <label class="f"><span>آدرس پایه</span>
              <a-input v-model:value="row.baseUrl" dir="ltr" autocomplete="off" placeholder="https://api.openai.com/v1" />
            </label>
            <label class="f"><span>نام مدل</span>
              <a-input v-model:value="row.model" dir="ltr" autocomplete="off" name="ai-model-name" placeholder="gpt-4o-mini" />
            </label>
            <label class="f"><span>کلید API</span>
              <a-input-password
                v-model:value="row.apiKey"
                dir="ltr"
                autocomplete="new-password"
                name="ai-provider-key"
                :placeholder="row.hasKey ? row.keyMask + ' — برای تغییر، کلید جدید بزنید' : 'کلید API را وارد کنید'"
                @input="onKeyInput(row)"
              />
            </label>
          </div>
          <p v-if="row.detectMsg" class="detect-msg faint">{{ row.detectMsg }}</p>
          <div class="prov-actions">
            <a-button size="small" :loading="testingId === row.id" @click="test(row)">تست</a-button>
            <a-button size="small" danger @click="removeRow(row.id)">حذف</a-button>
          </div>
        </div>
      </a-radio-group>

      <div v-if="!rows.length" class="empty">هنوز سرویسی ثبت نشده — «افزودن سرویس» را بزنید.</div>

      <div class="row-wrap" style="margin-top: 14px">
        <a-button @click="addRow"><template #icon><AppIcon name="plus" :size="14" /></template>افزودن سرویس</a-button>
        <a-button type="primary" :loading="saving" @click="save">ذخیره سرویس‌ها</a-button>
      </div>
    </section>

    <section class="card card-pad">
      <span class="section-title">دستیار بررسی کد (مستقل از شرکت‌کننده)</span>
      <p class="faint hint">یک فایل <code>.java</code> آپلود کنید یا کد را بچسبانید.</p>

      <div class="row-wrap">
        <a-button @click="crFileInput?.click()">
          <template #icon><AppIcon name="upload" :size="14" /></template>انتخاب فایل .java
        </a-button>
        <a-button v-if="crCode" @click="clearReview">پاک‌کردن</a-button>
        <input ref="crFileInput" type="file" accept=".java,text/plain" hidden @change="onJavaFile" />
      </div>

      <div class="cr-editor">
        <JavaEditor v-model="crCode" :disabled="crRunning" />
      </div>

      <div class="row-wrap" style="margin-top: 10px">
        <a-button type="primary" :loading="crRunning" @click="runReview">
          <template #icon><AppIcon name="wand" :size="15" /></template>اجرای بررسی
        </a-button>
        <span v-if="crMeta" class="faint tiny">{{ crMeta }}</span>
      </div>

      <div v-if="crRunning" class="empty"><a-spin /> در حال تحلیل با هوش مصنوعی…</div>

      <div v-if="crScore != null" class="score-row" style="margin-top: 14px">
        <Gauge :percent="crScore" :color="crColor(crScore)" :size="84" />
        <div class="faint tiny">درصد تخمینی «تولید با هوش مصنوعی»</div>
      </div>
      <MarkdownLite v-if="crReport" :source="crReport" style="margin-top: 12px" />
    </section>
  </div>
</template>

<style scoped>
/* off-screen decoy pair — catches the browser's username/password autofill so
   it never overwrites the real «نام مدل» / «کلید API» inputs */
.af-trap {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  opacity: 0;
  pointer-events: none;
}
.head-row {
  gap: 12px;
  margin-bottom: 10px;
}
.hint {
  font-size: 0.76rem;
  margin: 4px 0 10px;
}
.prov-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  width: 100%;
}
.prov {
  padding: var(--space-4);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.prov-active {
  font-weight: 600;
}
.prov-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: 1fr;
}
@media (min-width: 720px) {
  .prov-grid {
    grid-template-columns: 1fr 1fr;
  }
}
.f {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.f > span {
  font-size: 0.76rem;
  font-weight: 600;
  color: var(--text-dim);
}
.detect-msg {
  font-size: 0.74rem;
  color: var(--info);
}
.prov-actions {
  display: flex;
  gap: 8px;
}
.empty {
  padding: 20px;
  text-align: center;
  color: var(--text-faint);
}
.cr-editor {
  height: 420px;
  margin-top: 10px;
}
.cr-editor :deep(.java-editor) {
  height: 100%;
}
.score-row {
  display: flex;
  align-items: center;
  gap: 14px;
}
.tiny {
  font-size: 0.72rem;
}
</style>
