<script setup lang="ts">
import { reactive, ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { message, Modal } from 'ant-design-vue'
import {
  getAppearance,
  saveAppearanceDraft,
  publishAppearance,
  revertAppearance,
  resetAppearance,
} from '@/api/client'
import type {
  Appearance,
  HeroAppearance,
  HeaderAppearance,
  TextReplacement,
  ElementOverride,
} from '@/api/types'
import AppIcon from '@/components/AppIcon.vue'

/* ---------------------------------------------------------------- model -- */

const COLOR_KEYS = [
  'primary', 'primary-hover', 'accent',
  'bg', 'surface', 'surface-2',
  'text', 'text-dim', 'border',
  'success', 'warning', 'danger', 'info',
] as const

const COLOR_LABELS: Record<string, string> = {
  primary: 'رنگ اصلی (برند)',
  'primary-hover': 'رنگ اصلی — حالت هاور',
  accent: 'رنگ تأکید',
  bg: 'پس‌زمینه‌ی صفحه',
  surface: 'سطح کارت‌ها',
  'surface-2': 'سطح ثانویه',
  text: 'متن اصلی',
  'text-dim': 'متن کم‌رنگ',
  border: 'حاشیه',
  success: 'موفقیت',
  warning: 'هشدار',
  danger: 'خطا',
  info: 'اطلاعات',
}

const FONT_OPTIONS = [
  { value: '', label: 'پیش‌فرض (IRANYekanX)' },
  { value: "'IRANYekanX', system-ui, sans-serif", label: 'IRANYekanX' },
  { value: 'system-ui, -apple-system, sans-serif', label: 'فونت سیستم' },
  { value: 'Tahoma, Arial, sans-serif', label: 'Tahoma' },
]

function blankColors(): Record<string, string> {
  return Object.fromEntries(COLOR_KEYS.map((k) => [k, '']))
}

const form = reactive({
  light: blankColors(),
  dark: blankColors(),
  shared: { radius: '', 'font-ui': '' } as Record<string, string>,
  hero: {
    title: '',
    subtitle: '',
    showSubtitle: true,
    align: '' as '' | 'start' | 'center' | 'end',
    background: '',
    minHeight: '',
    image: '',
  },
  header: { brand: '', background: '' },
  customCss: '',
  texts: [] as { find: string; replace: string; mode: 'exact' | 'contains' }[],
  overrides: [] as Override[],
})

interface Override {
  selector: string
  text: string
  setText: boolean
  hidden: boolean
  fontSize: string
  fontWeight: string
  textAlign: string
  color: string
  background: string
  padding: string
  margin: string
  borderWidth: string
  borderColor: string
  borderRadius: string
  width: string
  maxWidth: string
  label: string
  isLeaf: boolean
  baseFontSize: string
}

function blankOverride(partial: Partial<Override>): Override {
  return {
    selector: '', text: '', setText: false, hidden: false,
    fontSize: '', fontWeight: '', textAlign: '',
    color: '', background: '', padding: '', margin: '',
    borderWidth: '', borderColor: '', borderRadius: '',
    width: '', maxWidth: '',
    label: '', isLeaf: true, baseFontSize: '',
    ...partial,
  }
}

const STYLE_KEYS: (keyof Override)[] = [
  'fontSize', 'fontWeight', 'textAlign', 'color', 'background', 'padding',
  'margin', 'borderWidth', 'borderColor', 'borderRadius', 'width', 'maxWidth',
]
function hasStyle(o: Override): boolean {
  return STYLE_KEYS.some((k) => o[k])
}

/** Bump a picked element's font-size by `dir` px (seeds from its real size). */
function nudgeFont(o: (typeof form.overrides)[number], dir: number): void {
  const raw = (o.fontSize || o.baseFontSize || '16px').trim()
  const m = /^([0-9.]+)(px|rem|em|%)$/.exec(raw)
  let px = 16
  if (m) {
    const n = parseFloat(m[1])
    px = m[2] === 'px' ? n : m[2] === '%' ? (n / 100) * 16 : n * 16
  }
  px = Math.min(96, Math.max(8, Math.round((px + dir) * 10) / 10))
  o.fontSize = px + 'px'
}

const WEIGHT_OPTIONS = [
  { value: '', label: 'پیش‌فرض' },
  { value: '300', label: 'نازک' },
  { value: '400', label: 'معمولی' },
  { value: '500', label: 'نیمه‌ضخیم' },
  { value: '600', label: 'ضخیم' },
  { value: '700', label: 'پررنگ' },
  { value: '800', label: 'سیاه' },
  { value: '900', label: 'خیلی سیاه' },
]

/* element picker */
const picking = ref(false)
const selectedOverride = ref(-1)

function sendPick(on: boolean): void {
  picking.value = on
  try {
    iframeEl.value?.contentWindow?.postMessage({ __dotin: 'pick', on }, location.origin)
  } catch {
    /* iframe not ready */
  }
}
function togglePick(): void {
  sendPick(!picking.value)
}
function removeOverride(i: number): void {
  form.overrides.splice(i, 1)
  if (selectedOverride.value === i) selectedOverride.value = -1
}
function shortSel(s: string): string {
  return s.length > 46 ? '…' + s.slice(-45) : s
}

const CSS_HINT =
  'هر چیزی که کنترل‌های بالا ندارند این‌جا با CSS بزنید. مسدود: @import، url(http…)، script، javascript:، expression. حداکثر ۲۰٬۰۰۰ نویسه.'

/* ------------------------------------------------------------ transforms -- */

function pruned(o: Record<string, string>): Record<string, string> {
  const r: Record<string, string> = {}
  for (const [k, v] of Object.entries(o)) {
    const t = (v || '').trim()
    if (t) r[k] = t
  }
  return r
}

function toAppearance(): Appearance {
  const tokens = { ...pruned(form.light), ...pruned(form.shared) }
  const dark = pruned(form.dark)

  const hero: HeroAppearance = {}
  if (form.hero.title.trim()) hero.title = form.hero.title.trim()
  if (form.hero.subtitle.trim()) hero.subtitle = form.hero.subtitle.trim()
  if (!form.hero.showSubtitle) hero.showSubtitle = false
  if (form.hero.align) hero.align = form.hero.align
  if (form.hero.background.trim()) hero.background = form.hero.background.trim()
  if (form.hero.minHeight.trim()) hero.minHeight = form.hero.minHeight.trim()
  if (form.hero.image) hero.image = form.hero.image

  const header: HeaderAppearance = {}
  if (form.header.brand.trim()) header.brand = form.header.brand.trim()
  if (form.header.background.trim()) header.background = form.header.background.trim()

  const a: Appearance = {}
  if (Object.keys(tokens).length) a.tokens = tokens
  if (Object.keys(dark).length) a.dark = dark
  const sections: NonNullable<Appearance['sections']> = {}
  if (Object.keys(hero).length) sections.hero = hero
  if (Object.keys(header).length) sections.header = header
  if (Object.keys(sections).length) a.sections = sections
  if (form.customCss.trim()) a.customCss = form.customCss.trim()

  const texts: TextReplacement[] = form.texts
    .filter((t) => t.find.trim())
    .map((t) => ({ find: t.find.trim(), replace: t.replace, mode: t.mode }))
  if (texts.length) a.texts = texts

  const overrides: ElementOverride[] = form.overrides
    .filter((o) => o.selector && (o.setText || o.hidden || o.text || hasStyle(o)))
    .map((o) => ({
      selector: o.selector,
      text: o.text,
      setText: o.setText,
      hidden: o.hidden,
      fontSize: o.fontSize,
      fontWeight: o.fontWeight,
      textAlign: o.textAlign,
      color: o.color,
      background: o.background,
      padding: o.padding,
      margin: o.margin,
      borderWidth: o.borderWidth,
      borderColor: o.borderColor,
      borderRadius: o.borderRadius,
      width: o.width,
      maxWidth: o.maxWidth,
      label: o.label,
    }))
  if (overrides.length) a.overrides = overrides

  return a
}

function fromAppearance(a?: Appearance): void {
  const t = a?.tokens || {}
  const d = a?.dark || {}
  for (const k of COLOR_KEYS) {
    form.light[k] = t[k] || ''
    form.dark[k] = d[k] || ''
  }
  form.shared.radius = t['radius'] || ''
  form.shared['font-ui'] = t['font-ui'] || ''

  const h = a?.sections?.hero || {}
  form.hero.title = h.title || ''
  form.hero.subtitle = h.subtitle || ''
  form.hero.showSubtitle = h.showSubtitle !== false
  form.hero.align = (h.align as typeof form.hero.align) || ''
  form.hero.background = h.background || ''
  form.hero.minHeight = h.minHeight || ''
  form.hero.image = h.image || ''

  const hd = a?.sections?.header || {}
  form.header.brand = hd.brand || ''
  form.header.background = hd.background || ''

  form.customCss = a?.customCss || ''

  form.texts = (a?.texts || []).map((t) => ({
    find: t.find || '',
    replace: t.replace || '',
    mode: t.mode === 'contains' ? 'contains' : 'exact',
  }))

  form.overrides = (a?.overrides || []).map((o) =>
    blankOverride({
      selector: o.selector || '',
      text: o.text || '',
      setText: !!o.setText,
      hidden: !!o.hidden,
      fontSize: o.fontSize || '',
      fontWeight: o.fontWeight || '',
      textAlign: o.textAlign || '',
      color: o.color || '',
      background: o.background || '',
      padding: o.padding || '',
      margin: o.margin || '',
      borderWidth: o.borderWidth || '',
      borderColor: o.borderColor || '',
      borderRadius: o.borderRadius || '',
      width: o.width || '',
      maxWidth: o.maxWidth || '',
      label: o.label || '',
    }),
  )
  selectedOverride.value = -1
}

/* --------------------------------------------------------------- state --- */

const loading = ref(true)
const saving = ref(false)
const publishing = ref(false)
const device = ref<'desktop' | 'mobile'>('desktop')
const activeTab = ref('settings')
const previewSrc = ref('/?preview=1')
const iframeEl = ref<HTMLIFrameElement | null>(null)

let savedDraftJson = '{}'
let publishedJson = '{}'

const currentJson = computed(() => JSON.stringify(toAppearance()))
const dirty = computed(() => currentJson.value !== savedDraftJson)
const unpublished = computed(() => dirty.value || savedDraftJson !== publishedJson)

/* --------------------------------------------------------- live preview -- */

let pushTimer: number | undefined
function pushPreview(): void {
  const win = iframeEl.value?.contentWindow
  if (!win) return
  try {
    win.postMessage({ __dotin: 'appearance', payload: toAppearance() }, location.origin)
  } catch {
    /* iframe not ready yet */
  }
}
function schedulePush(): void {
  window.clearTimeout(pushTimer)
  pushTimer = window.setTimeout(pushPreview, 120)
}

function onPreviewMessage(e: MessageEvent): void {
  if (e.origin !== location.origin) return
  const d = e.data
  if (!d || typeof d.__dotin !== 'string') return

  if (d.__dotin === 'appearance-ready') {
    pushPreview()
    if (picking.value) sendPick(true)
    return
  }

  if (d.__dotin === 'picked') {
    sendPick(false) // one element at a time
    const existing = form.overrides.findIndex((o) => o.selector === d.selector)
    if (existing >= 0) {
      selectedOverride.value = existing
    } else {
      form.overrides.push(
        blankOverride({
          selector: String(d.selector || ''),
          text: String(d.text || ''),
          label: String(d.label || ''),
          isLeaf: d.isLeaf !== false,
          baseFontSize: String(d.baseFontSize || ''),
        }),
      )
      selectedOverride.value = form.overrides.length - 1
    }
    activeTab.value = 'elements'
  }
}

function reloadPreview(): void {
  previewSrc.value = '/?preview=1&t=' + Date.now()
  picking.value = false
}

/* ------------------------------------------------------------- actions --- */

async function load(): Promise<void> {
  loading.value = true
  try {
    const { draft, published } = await getAppearance()
    fromAppearance(draft && Object.keys(draft).length ? draft : published)
    savedDraftJson = currentJson.value
    publishedJson = JSON.stringify(published || {})
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'دریافت تنظیمات ظاهر ناموفق بود')
  } finally {
    loading.value = false
  }
}

async function saveDraft(): Promise<void> {
  saving.value = true
  try {
    await saveAppearanceDraft(toAppearance())
    savedDraftJson = currentJson.value
    message.success('پیش‌نویس ذخیره شد — روی پنل کاربر هنوز اعمال نشده')
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'ذخیره‌ی پیش‌نویس ناموفق بود')
  } finally {
    saving.value = false
  }
}

function doPublish(): void {
  Modal.confirm({
    title: 'انتشار طراحی',
    content: 'تغییرات همین حالا برای همه‌ی کاربران پنل کاربر اعمال می‌شود (ظرف چند ثانیه).',
    okText: 'انتشار',
    cancelText: 'انصراف',
    onOk: async () => {
      publishing.value = true
      try {
        await saveAppearanceDraft(toAppearance())
        await publishAppearance()
        savedDraftJson = currentJson.value
        publishedJson = savedDraftJson
        message.success('منتشر شد — کاربران تا چند ثانیه‌ی دیگر تغییر را می‌بینند')
      } catch (e) {
        message.error(e instanceof Error ? e.message : 'انتشار ناموفق بود')
      } finally {
        publishing.value = false
      }
    },
  })
}

function doRevert(): void {
  Modal.confirm({
    title: 'برگرداندن به نسخه‌ی منتشرشده',
    content: 'همه‌ی تغییرات ذخیره‌نشده و پیش‌نویس دور ریخته می‌شود.',
    okText: 'برگردان',
    cancelText: 'انصراف',
    okType: 'danger',
    onOk: async () => {
      try {
        const { draft } = await revertAppearance()
        fromAppearance(draft)
        savedDraftJson = currentJson.value
        publishedJson = savedDraftJson
        schedulePush()
        message.success('به نسخه‌ی منتشرشده برگشت')
      } catch (e) {
        message.error(e instanceof Error ? e.message : 'عملیات ناموفق بود')
      }
    },
  })
}

function doReset(): void {
  Modal.confirm({
    title: 'بازنشانی به طراحی پیش‌فرض',
    content: 'همه‌ی سفارشی‌سازی‌ها (پیش‌نویس و منتشرشده) پاک می‌شود و سامانه به طراحی اصلی برمی‌گردد.',
    okText: 'بازنشانی',
    cancelText: 'انصراف',
    okType: 'danger',
    onOk: async () => {
      try {
        await resetAppearance()
        fromAppearance({})
        savedDraftJson = currentJson.value
        publishedJson = '{}'
        schedulePush()
        message.success('به طراحی پیش‌فرض بازگشت')
      } catch (e) {
        message.error(e instanceof Error ? e.message : 'بازنشانی ناموفق بود')
      }
    },
  })
}

function onImagePick(e: Event): void {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 512 * 1024) {
    message.error('حجم تصویر باید کمتر از ۵۱۲ کیلوبایت باشد')
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    form.hero.image = String(reader.result || '')
  }
  reader.readAsDataURL(file)
}

/* ------------------------------------------------------- export / import -- */

const exportOpen = ref(false)
const importOpen = ref(false)
const exportText = ref('')
const importText = ref('')

function openExport(): void {
  exportText.value = JSON.stringify(toAppearance(), null, 2)
  exportOpen.value = true
  navigator.clipboard?.writeText(exportText.value).then(
    () => message.success('در کلیپ‌بورد کپی شد'),
    () => {},
  )
}

function applyImport(): void {
  try {
    const parsed = JSON.parse(importText.value)
    if (!parsed || typeof parsed !== 'object') throw new Error('bad')
    fromAppearance(parsed as Appearance)
    savedDraftJson = '' // force "unpublished" until saved
    schedulePush()
    importOpen.value = false
    importText.value = ''
    message.success('اعمال شد — برای ماندگاری «ذخیره پیش‌نویس» و «انتشار» را بزن')
  } catch {
    message.error('JSON نامعتبر است')
  }
}

/* --------------------------------------------------------------- wiring -- */

watch(form, schedulePush, { deep: true })

onMounted(() => {
  window.addEventListener('message', onPreviewMessage)
  void load()
})
onBeforeUnmount(() => {
  window.removeEventListener('message', onPreviewMessage)
  window.clearTimeout(pushTimer)
})
</script>

<template>
  <div class="studio">
    <header class="studio-bar card">
      <div class="bar-title">
        <AppIcon name="palette" :size="18" />
        <span>استودیو طراحی</span>
        <span v-if="unpublished" class="badge-unpub">منتشرنشده</span>
      </div>
      <div class="grow" />
      <a-button size="small" @click="openExport">خروجی</a-button>
      <a-button size="small" @click="importOpen = true">ورودی</a-button>
      <a-button size="small" :loading="saving" @click="saveDraft">ذخیره پیش‌نویس</a-button>
      <a-button size="small" type="primary" :loading="publishing" @click="doPublish">انتشار</a-button>
      <a-button size="small" @click="doRevert">برگردان</a-button>
      <a-button size="small" danger @click="doReset">پیش‌فرض</a-button>
    </header>

    <a-modal v-model:open="exportOpen" title="خروجی طراحی (JSON)" :footer="null" width="560px">
      <p class="faint" style="margin-top: 0">
        این متن کل طراحی فعلی است. برای انتقال به سرور دیگر، در پنل آن‌جا «ورودی» را بزن و همین را
        بچسبان — یا همین JSON را به تیم بده تا به‌عنوان پیش‌فرضِ کدِ پروژه در گیت‌هاب ثبت شود.
      </p>
      <a-textarea :value="exportText" :rows="14" readonly class="css-editor" />
    </a-modal>

    <a-modal
      v-model:open="importOpen"
      title="ورودی طراحی (JSON)"
      ok-text="اعمال"
      cancel-text="انصراف"
      @ok="applyImport"
    >
      <p class="faint" style="margin-top: 0">JSONی که از «خروجی» گرفته‌ای را این‌جا بچسبان.</p>
      <a-textarea v-model:value="importText" :rows="14" class="css-editor" placeholder="{ ... }" />
    </a-modal>

    <div class="studio-grid">
      <!-- controls -->
      <section class="panel card" :class="{ busy: loading }">
        <a-tabs v-model:activeKey="activeTab" size="small">
          <a-tab-pane key="settings" tab="تنظیمات">
            <a-collapse :bordered="false" :default-active-key="['colors', 'hero']" ghost>
              <a-collapse-panel key="colors" header="رنگ‌ها — حالت روشن">
                <div class="rows">
                  <label v-for="k in COLOR_KEYS" :key="'l-' + k" class="row">
                    <span class="row-lbl">{{ COLOR_LABELS[k] }}</span>
                    <span class="row-ctl">
                      <input type="color" :value="form.light[k] || '#000000'"
                        @input="form.light[k] = ($event.target as HTMLInputElement).value" />
                      <a-input v-model:value="form.light[k]" size="small" placeholder="پیش‌فرض"
                        allow-clear />
                    </span>
                  </label>
                </div>
              </a-collapse-panel>

              <a-collapse-panel key="colorsDark" header="رنگ‌ها — حالت تیره">
                <div class="rows">
                  <label v-for="k in COLOR_KEYS" :key="'d-' + k" class="row">
                    <span class="row-lbl">{{ COLOR_LABELS[k] }}</span>
                    <span class="row-ctl">
                      <input type="color" :value="form.dark[k] || '#000000'"
                        @input="form.dark[k] = ($event.target as HTMLInputElement).value" />
                      <a-input v-model:value="form.dark[k]" size="small" placeholder="پیش‌فرض"
                        allow-clear />
                    </span>
                  </label>
                </div>
              </a-collapse-panel>

              <a-collapse-panel key="shape" header="شکل و فونت">
                <div class="rows">
                  <label class="row">
                    <span class="row-lbl">گِردی گوشه‌ها</span>
                    <span class="row-ctl">
                      <a-input v-model:value="form.shared.radius" size="small"
                        placeholder="مثلاً 12px" allow-clear />
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">فونت رابط</span>
                    <span class="row-ctl">
                      <a-select v-model:value="form.shared['font-ui']" size="small" style="width: 100%">
                        <a-select-option v-for="o in FONT_OPTIONS" :key="o.value" :value="o.value">
                          {{ o.label }}
                        </a-select-option>
                      </a-select>
                    </span>
                  </label>
                </div>
              </a-collapse-panel>

              <a-collapse-panel key="hero" header="هیرو — صفحه‌ی ورود کاربر">
                <div class="rows">
                  <label class="row">
                    <span class="row-lbl">عنوان</span>
                    <span class="row-ctl">
                      <a-input v-model:value="form.hero.title" size="small"
                        placeholder="به چالش برنامه‌نویسی مدرسه داتین خوش آمدید" allow-clear />
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">زیرعنوان</span>
                    <span class="row-ctl">
                      <a-textarea v-model:value="form.hero.subtitle" size="small" :rows="2"
                        placeholder="یک توضیح کوتاه زیر عنوان" allow-clear />
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">نمایش زیرعنوان</span>
                    <span class="row-ctl"><a-switch v-model:checked="form.hero.showSubtitle" /></span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">ترازبندی متن</span>
                    <span class="row-ctl">
                      <a-radio-group v-model:value="form.hero.align" size="small" button-style="solid">
                        <a-radio-button value="">پیش‌فرض</a-radio-button>
                        <a-radio-button value="start">راست</a-radio-button>
                        <a-radio-button value="center">وسط</a-radio-button>
                        <a-radio-button value="end">چپ</a-radio-button>
                      </a-radio-group>
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">پس‌زمینه (رنگ/گرادیان)</span>
                    <span class="row-ctl">
                      <a-input v-model:value="form.hero.background" size="small"
                        placeholder="مثلاً linear-gradient(...)" allow-clear />
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">حداقل ارتفاع</span>
                    <span class="row-ctl">
                      <a-input v-model:value="form.hero.minHeight" size="small" placeholder="مثلاً 70vh"
                        allow-clear />
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">تصویر پس‌زمینه</span>
                    <span class="row-ctl img-ctl">
                      <input type="file" accept="image/*" @change="onImagePick" />
                      <a-button v-if="form.hero.image" size="small" danger @click="form.hero.image = ''">
                        حذف تصویر
                      </a-button>
                    </span>
                  </label>
                </div>
              </a-collapse-panel>

              <a-collapse-panel key="header" header="هدر پنل کاربر">
                <div class="rows">
                  <label class="row">
                    <span class="row-lbl">متن برند</span>
                    <span class="row-ctl">
                      <a-input v-model:value="form.header.brand" size="small" placeholder="dotinschool"
                        allow-clear />
                    </span>
                  </label>
                  <label class="row">
                    <span class="row-lbl">پس‌زمینه‌ی هدر</span>
                    <span class="row-ctl">
                      <a-input v-model:value="form.header.background" size="small" placeholder="رنگ/گرادیان"
                        allow-clear />
                    </span>
                  </label>
                </div>
              </a-collapse-panel>
            </a-collapse>
          </a-tab-pane>

          <a-tab-pane key="elements" tab="عناصر">
            <a-button
              size="small"
              :type="picking ? 'primary' : 'default'"
              block
              @click="togglePick"
            >
              <AppIcon name="target" :size="14" />
              {{ picking ? 'روی یک بخش در پیش‌نمایش کلیک کن…' : 'انتخاب عنصر از پیش‌نمایش' }}
            </a-button>
            <p class="css-hint faint">
              روی هر متن یا بخش در پیش‌نمایش کلیک کن تا این‌جا باز شود — می‌توانی متنش را عوض کنی
              یا مخفی/نمایشش کنی. ادیتور جاوا و بلوک‌های کد دست‌نخورده می‌مانند.
            </p>

            <div v-if="!form.overrides.length" class="faint empty-els">هنوز عنصری انتخاب نشده.</div>

            <div v-for="(o, i) in form.overrides" :key="i" class="el-card"
              :class="{ active: selectedOverride === i }" @click="selectedOverride = i">
              <div class="el-head">
                <span class="el-label">{{ o.label || o.selector }}</span>
                <a-button size="small" type="text" danger @click.stop="removeOverride(i)">
                  <AppIcon name="trash" :size="13" />
                </a-button>
              </div>
              <code class="el-sel">{{ shortSel(o.selector) }}</code>

              <template v-if="selectedOverride === i">
                <label class="el-toggle">
                  <a-switch v-model:checked="o.hidden" size="small" />
                  <span>مخفی کردن این بخش</span>
                </label>
                <label class="el-toggle">
                  <a-switch v-model:checked="o.setText" size="small" :disabled="!o.isLeaf || o.hidden" />
                  <span>تغییر متن</span>
                </label>
                <a-textarea v-if="o.setText && o.isLeaf && !o.hidden" v-model:value="o.text"
                  :rows="2" size="small" placeholder="متن جدید" @click.stop />
                <p v-if="!o.isLeaf" class="faint tiny">
                  این عنصر زیرالمان دارد — فقط استایل و مخفی/نمایش ممکن است.
                </p>

                <template v-if="!o.hidden">
                  <div class="el-field" @click.stop>
                    <span class="el-flbl">اندازه فونت</span>
                    <div class="fs-row">
                      <a-button size="small" title="کوچک‌تر" @click="nudgeFont(o, -1)">
                        <AppIcon name="chevron-down" :size="14" />
                      </a-button>
                      <a-input v-model:value="o.fontSize" size="small" placeholder="خودکار — مثلاً ۱۶px"
                        allow-clear />
                      <a-button size="small" title="بزرگ‌تر" @click="nudgeFont(o, 1)">
                        <AppIcon name="plus" :size="14" />
                      </a-button>
                    </div>
                  </div>
                  <div class="el-field" @click.stop>
                    <span class="el-flbl">ضخامت فونت</span>
                    <a-select v-model:value="o.fontWeight" size="small" style="width: 100%">
                      <a-select-option v-for="w in WEIGHT_OPTIONS" :key="w.value" :value="w.value">
                        {{ w.label }}
                      </a-select-option>
                    </a-select>
                  </div>
                  <div class="el-field" @click.stop>
                    <span class="el-flbl">تراز متن</span>
                    <a-radio-group v-model:value="o.textAlign" size="small" button-style="solid">
                      <a-radio-button value="">پیش‌فرض</a-radio-button>
                      <a-radio-button value="right">راست</a-radio-button>
                      <a-radio-button value="center">وسط</a-radio-button>
                      <a-radio-button value="left">چپ</a-radio-button>
                    </a-radio-group>
                  </div>

                  <div class="el-field" @click.stop>
                    <span class="el-flbl">رنگ متن / پس‌زمینه</span>
                    <div class="fs-row">
                      <input type="color" :value="o.color || '#000000'"
                        @input="o.color = ($event.target as HTMLInputElement).value" />
                      <a-input v-model:value="o.color" size="small" placeholder="رنگ متن" allow-clear />
                    </div>
                    <div class="fs-row">
                      <input type="color" :value="o.background || '#ffffff'"
                        @input="o.background = ($event.target as HTMLInputElement).value" />
                      <a-input v-model:value="o.background" size="small" placeholder="پس‌زمینه" allow-clear />
                    </div>
                  </div>

                  <div class="el-field" @click.stop>
                    <span class="el-flbl">فاصله داخلی / بیرونی (padding / margin)</span>
                    <a-input v-model:value="o.padding" size="small" placeholder="padding — مثلاً 8px 16px"
                      allow-clear />
                    <a-input v-model:value="o.margin" size="small" placeholder="margin — مثلاً 0 auto"
                      allow-clear />
                  </div>

                  <div class="el-field" @click.stop>
                    <span class="el-flbl">حاشیه</span>
                    <div class="fs-row">
                      <a-input v-model:value="o.borderWidth" size="small" placeholder="ضخامت — 1px"
                        allow-clear />
                      <input type="color" :value="o.borderColor || '#000000'"
                        @input="o.borderColor = ($event.target as HTMLInputElement).value" />
                    </div>
                    <a-input v-model:value="o.borderRadius" size="small" placeholder="گِردی گوشه — 12px"
                      allow-clear />
                  </div>

                  <div class="el-field" @click.stop>
                    <span class="el-flbl">اندازه</span>
                    <a-input v-model:value="o.width" size="small" placeholder="width — مثلاً 320px یا 100%"
                      allow-clear />
                    <a-input v-model:value="o.maxWidth" size="small" placeholder="max-width — مثلاً 480px"
                      allow-clear />
                  </div>
                </template>
              </template>
            </div>
          </a-tab-pane>

          <a-tab-pane key="css" tab="CSS سفارشی">
            <p class="css-hint faint">{{ CSS_HINT }}</p>
            <a-textarea v-model:value="form.customCss" class="css-editor" :rows="20" spellcheck="false"
              placeholder="/* مثال */&#10;.challenge-card { border-width: 2px; }" />
            <div class="css-count faint">{{ form.customCss.length }} / ۲۰۰۰۰</div>
          </a-tab-pane>
        </a-tabs>
      </section>

      <!-- live preview -->
      <section class="preview card">
        <div class="preview-bar">
          <a-radio-group v-model:value="device" size="small" button-style="solid">
            <a-radio-button value="desktop">دسکتاپ</a-radio-button>
            <a-radio-button value="mobile">موبایل</a-radio-button>
          </a-radio-group>
          <div class="grow" />
          <a-button size="small" @click="reloadPreview">
            <AppIcon name="refresh" :size="14" /> بارگذاری مجدد
          </a-button>
        </div>
        <div class="preview-stage">
          <iframe ref="iframeEl" :src="previewSrc" class="preview-frame" :class="device"
            title="پیش‌نمایش پنل کاربر" />
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.studio {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}
.studio-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: 10px var(--space-4);
}
.bar-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
}
.badge-unpub {
  margin-inline-start: 8px;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: var(--warning-soft);
  color: var(--warning);
  font-size: 0.72rem;
  font-weight: 700;
}
.grow {
  flex: 1;
}
.studio-grid {
  display: grid;
  grid-template-columns: minmax(360px, 440px) 1fr;
  gap: var(--space-4);
  align-items: start;
}
.panel {
  padding: var(--space-3) var(--space-4) var(--space-4);
  max-height: calc(100dvh - 190px);
  overflow-y: auto;
}
.panel.busy {
  opacity: 0.5;
  pointer-events: none;
}
.rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.row-lbl {
  font-size: 0.78rem;
  color: var(--text-dim);
}
.row-ctl {
  display: flex;
  align-items: center;
  gap: 8px;
}
.row-ctl input[type='color'] {
  width: 34px;
  height: 30px;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: none;
  flex: none;
  cursor: pointer;
}
.img-ctl {
  flex-wrap: wrap;
}
.css-hint {
  font-size: 0.76rem;
  margin: 0 0 8px;
  line-height: 1.8;
}
.css-editor {
  font-family: var(--font-mono);
  font-size: 0.82rem;
}
.css-count {
  text-align: end;
  font-size: 0.72rem;
  margin-top: 4px;
}
.empty-els {
  font-size: 0.8rem;
  padding: 10px 0;
}
.tiny {
  font-size: 0.72rem;
  margin: 4px 0 0;
}
.el-card {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  margin-top: 8px;
  cursor: pointer;
}
.el-card.active {
  border-color: var(--primary);
  background: var(--primary-soft);
}
.el-head {
  display: flex;
  align-items: center;
  gap: 6px;
}
.el-label {
  flex: 1;
  min-width: 0;
  font-size: 0.82rem;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.el-sel {
  display: block;
  font-size: 0.68rem;
  color: var(--text-faint);
  direction: ltr;
  text-align: left;
  word-break: break-all;
}
.el-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  font-size: 0.8rem;
}
.el-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}
.fs-row {
  display: flex;
  align-items: center;
  gap: 4px;
}
.fs-row > .ant-input {
  flex: 1;
  min-width: 0;
}
.fs-row input[type='color'] {
  width: 34px;
  height: 30px;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: none;
  flex: none;
  cursor: pointer;
}
.el-flbl {
  font-size: 0.76rem;
  color: var(--text-dim);
}
.preview {
  padding: 10px;
  position: sticky;
  top: 76px;
}
.preview-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.preview-stage {
  background: var(--surface-2);
  border-radius: var(--radius);
  padding: 10px;
  overflow: auto;
  height: calc(100dvh - 240px);
  display: flex;
  justify-content: center;
}
.preview-frame {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: #fff;
  height: 100%;
  transition: width var(--dur) var(--ease);
}
.preview-frame.desktop {
  width: 100%;
}
.preview-frame.mobile {
  width: 390px;
  flex: none;
}

@media (max-width: 1080px) {
  .studio-grid {
    grid-template-columns: 1fr;
  }
  .preview {
    position: static;
  }
  .preview-stage {
    height: 70vh;
  }
}
</style>
