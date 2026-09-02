<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useAdminStore } from '@/stores/admin'
import { useEventStore } from '@/stores/event'
import { getConfig, setConfig } from '@/api/client'
import type { Challenge } from '@/api/types'
import ChallengeCard from '@/components/ChallengeCard.vue'
import AppIcon from '@/components/AppIcon.vue'

const admin = useAdminStore()
const eStore = useEventStore()

const DEFAULT_SUBMIT_MESSAGE =
  'کدت با موفقیت ثبت شد. پس از بررسی مدیر سامانه، نتیجه‌ی چالش به شما اعلام خواهد شد.'

const form = reactive<Challenge>({
  title: '',
  diff: 'medium',
  tag: '',
  desc: '',
  sampleIn: '',
  sampleOut: '',
  image: null,
  mode: 'correctness',
  timeLimitMs: 200,
  submitMessage: DEFAULT_SUBMIT_MESSAGE,
})
const savingAll = ref(false)
const savingMsg = ref(false)
const fileInput = ref<HTMLInputElement>()

onMounted(async () => {
  eStore.start()
  try {
    const cfg = await getConfig()
    Object.assign(form, cfg.challenge)
    if (!form.submitMessage) form.submitMessage = DEFAULT_SUBMIT_MESSAGE
  } catch (e) {
    admin.toastError(e, 'خطا در بارگذاری چالش')
  }
})

const preview = computed<Challenge>(() => ({ ...form }))

function resizeImage(file: File, maxW = 960, quality = 0.82): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(reader.error || new Error('خطا در خواندن فایل'))
    reader.onload = () => {
      const img = new Image()
      img.onerror = () => reject(new Error('فایل تصویر نامعتبر است'))
      img.onload = () => {
        const scale = Math.min(1, maxW / img.width)
        const w = Math.max(1, Math.round(img.width * scale))
        const h = Math.max(1, Math.round(img.height * scale))
        const canvas = document.createElement('canvas')
        canvas.width = w
        canvas.height = h
        canvas.getContext('2d')!.drawImage(img, 0, 0, w, h)
        resolve(canvas.toDataURL('image/jpeg', quality))
      }
      img.src = String(reader.result)
    }
    reader.readAsDataURL(file)
  })
}
async function onImage(ev: Event) {
  const file = (ev.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (!file.type.startsWith('image/')) {
    message.error('لطفاً یک فایل تصویری انتخاب کنید')
    ;(ev.target as HTMLInputElement).value = ''
    return
  }
  try {
    form.image = await resizeImage(file)
    message.success('تصویر بارگذاری شد — برای انتشار «ذخیره و انتشار چالش» را بزنید')
  } catch (e) {
    message.error(e instanceof Error ? e.message : 'خطا در بارگذاری تصویر')
  } finally {
    ;(ev.target as HTMLInputElement).value = ''
  }
}
function removeImage() {
  form.image = null
}

async function saveChallenge() {
  savingAll.value = true
  try {
    await setConfig({
      challenge: { ...form, submitMessage: (form.submitMessage || '').trim() || DEFAULT_SUBMIT_MESSAGE },
    })
    await eStore.refresh()
    message.success('چالش ذخیره و برای کاربران منتشر شد')
  } catch (e) {
    admin.toastError(e, 'خطا در ذخیره‌ی چالش')
  } finally {
    savingAll.value = false
  }
}
async function saveSubmitMessage() {
  savingMsg.value = true
  try {
    const cfg = await getConfig()
    await setConfig({
      challenge: { ...cfg.challenge, submitMessage: (form.submitMessage || '').trim() || DEFAULT_SUBMIT_MESSAGE },
    })
    await eStore.refresh()
    message.success('پیام پس از ارسال به‌روزرسانی و منتشر شد')
  } catch (e) {
    admin.toastError(e, 'خطا در ذخیره‌ی پیام')
  } finally {
    savingMsg.value = false
  }
}
function resetSubmitMessage() {
  form.submitMessage = DEFAULT_SUBMIT_MESSAGE
  message.info('متن به حالت پیش‌فرض برگشت — برای اعمال «ذخیره پیام» را بزنید')
}
</script>

<template>
  <div class="editor-layout">
    <section class="card card-pad form-col">
      <span class="section-title">طراحی چالش</span>

      <label class="f"><span>عنوان</span><a-input v-model:value="form.title" /></label>

      <div class="f-row">
        <label class="f"><span>سطح دشواری</span>
          <a-select
            v-model:value="form.diff"
            :options="[
              { value: 'easy', label: 'آسان' },
              { value: 'medium', label: 'متوسط' },
              { value: 'hard', label: 'دشوار' },
            ]"
          />
        </label>
        <label class="f grow"><span>برچسب موضوعی</span><a-input v-model:value="form.tag" /></label>
      </div>

      <label class="f"><span>متن کامل صورت مسئله</span>
        <a-textarea v-model:value="form.desc" :auto-size="{ minRows: 5, maxRows: 14 }" />
      </label>

      <div class="f-row">
        <label class="f grow"><span>ورودی نمونه</span><a-input v-model:value="form.sampleIn" dir="ltr" /></label>
        <label class="f grow"><span>خروجی نمونه</span><a-input v-model:value="form.sampleOut" dir="ltr" /></label>
      </div>

      <div class="f-row">
        <label class="f"><span>معیار امتیازدهی</span>
          <a-select
            v-model:value="form.mode"
            :options="[
              { value: 'correctness', label: 'درستی خروجی' },
              { value: 'performance', label: 'سرعت اجرا' },
            ]"
          />
        </label>
        <label v-if="form.mode === 'performance'" class="f"><span>زمان هدف (ms)</span>
          <a-input-number v-model:value="form.timeLimitMs" :min="1" />
        </label>
      </div>

      <div class="f">
        <span>تصویر پیوست (اختیاری)</span>
        <div class="row-wrap">
          <a-button @click="fileInput?.click()">
            <template #icon><AppIcon name="upload" :size="14" /></template>انتخاب تصویر
          </a-button>
          <a-button v-if="form.image" danger @click="removeImage">
            <template #icon><AppIcon name="trash" :size="14" /></template>حذف تصویر
          </a-button>
          <input ref="fileInput" type="file" accept="image/*" hidden @change="onImage" />
        </div>
        <img v-if="form.image" :src="form.image" alt="پیش‌نمایش تصویر" class="img-preview" />
      </div>

      <a-button type="primary" size="large" block :loading="savingAll" @click="saveChallenge">
        <template #icon><AppIcon name="check" :size="16" /></template>ذخیره و انتشار چالش
      </a-button>

      <div class="submsg-card">
        <span class="section-title">پیام پس از ارسال کد</span>
        <p class="faint hint">به‌جای نمایش امتیاز، همین متن به شرکت‌کننده نشان داده می‌شود.</p>
        <a-textarea
          v-model:value="form.submitMessage"
          :auto-size="{ minRows: 3, maxRows: 8 }"
          :maxlength="600"
          show-count
        />
        <div class="row-wrap">
          <a-button type="primary" :loading="savingMsg" @click="saveSubmitMessage">ذخیره پیام</a-button>
          <a-button @click="resetSubmitMessage">بازگردانی به پیش‌فرض</a-button>
        </div>
      </div>
    </section>

    <aside class="card card-pad preview-col">
      <span class="section-title">پیش‌نمایش زنده — همان‌طور که کاربر می‌بیند</span>
      <ChallengeCard :challenge="preview" />
      <div class="submsg-preview">
        <span class="section-title">پیام پس از ارسال</span>
        <p>{{ (form.submitMessage || '').trim() || DEFAULT_SUBMIT_MESSAGE }}</p>
      </div>
    </aside>
  </div>
</template>

<style scoped>
.editor-layout {
  display: grid;
  gap: var(--space-4);
}
@media (min-width: 1080px) {
  .editor-layout {
    grid-template-columns: 1fr 420px;
    align-items: start;
  }
  .preview-col {
    position: sticky;
    top: calc(var(--header-h) + 16px);
    max-height: calc(100dvh - var(--header-h) - 32px);
    overflow-y: auto;
  }
}
.form-col {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
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
.f-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.f-row .f {
  min-width: 140px;
}
.img-preview {
  margin-top: 10px;
  max-height: 180px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
}
.submsg-card,
.submsg-preview {
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius);
  padding: var(--space-4);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.hint {
  font-size: 0.74rem;
  margin: 0;
}
.submsg-preview p {
  font-size: 0.84rem;
  line-height: 2;
  color: var(--text-dim);
  white-space: pre-line;
  margin: 0;
}
</style>
