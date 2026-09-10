<script setup lang="ts">
import { reactive, ref, computed, onBeforeUnmount } from 'vue'
import { message } from 'ant-design-vue'
import { requestOtp, ApiError } from '@/api/client'
import { useParticipantStore } from '@/stores/participant'
import { toFa } from '@/lib/fa'
import AppIcon from '@/components/AppIcon.vue'

const emit = defineEmits<{ authed: [status: string] }>()
const store = useParticipantStore()

type Mode = 'register' | 'login'
const mode = ref<Mode>('register')

const form = reactive({ first: '', last: '', phone: '', otp: '' })
const errors = reactive<Record<string, string>>({})
const otpSent = ref(false)
const sending = ref(false)
const submitting = ref(false)
const cooldown = ref(0)
const shakeOtp = ref(false)
let cdTimer: number | undefined

function normalizePhone(v: string): string {
  return v.replace(/\D/g, '').replace(/^98/, '').replace(/^0/, '')
}
const phoneOk = computed(() => /^9\d{9}$/.test(normalizePhone(form.phone)))

function resetOtp() {
  otpSent.value = false
  form.otp = ''
  errors.otp = ''
  cooldown.value = 0
  if (cdTimer) {
    clearInterval(cdTimer)
    cdTimer = undefined
  }
}
function switchMode(m: Mode) {
  mode.value = m
  Object.keys(errors).forEach((k) => (errors[k] = ''))
  resetOtp()
}
function startCooldown() {
  cooldown.value = 60
  cdTimer = window.setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0 && cdTimer) {
      clearInterval(cdTimer)
      cdTimer = undefined
    }
  }, 1000)
}
onBeforeUnmount(() => cdTimer && clearInterval(cdTimer))

async function sendOtp() {
  errors.phone = ''
  const phone = normalizePhone(form.phone)
  if (!/^9\d{9}$/.test(phone)) {
    errors.phone = 'شماره موبایل باید ۱۰ رقم باشد و با ۹ شروع شود (بدون صفر ابتدایی و بدون +۹۸)، مثل ۹۱۲۳۴۵۶۷۸۹.'
    return
  }
  if (mode.value === 'register') {
    if (!form.first.trim()) return (errors.first = 'لطفاً نام را وارد کنید.') && undefined
    if (!form.last.trim()) return (errors.last = 'لطفاً نام خانوادگی را وارد کنید.') && undefined
  }
  sending.value = true
  try {
    await requestOtp(phone, mode.value)
    otpSent.value = true
    startCooldown()
    message.success('کد تأیید برای شماره‌ی شما پیامک شد')
  } catch (e) {
    errors.phone = e instanceof ApiError ? e.message : 'ارسال کد ناموفق بود'
  } finally {
    sending.value = false
  }
}

async function submit() {
  Object.keys(errors).forEach((k) => (errors[k] = ''))
  const phone = normalizePhone(form.phone)
  if (mode.value === 'register') {
    if (!form.first.trim()) return void (errors.first = 'لطفاً نام را وارد کنید.')
    if (!form.last.trim()) return void (errors.last = 'لطفاً نام خانوادگی را وارد کنید.')
  }
  if (!/^9\d{9}$/.test(phone)) return void (errors.phone = 'شماره موبایل نامعتبر است.')
  if (!form.otp.trim()) return void (errors.otp = 'لطفاً کدی که برایتان پیامک شد را وارد کنید.')

  submitting.value = true
  try {
    const u =
      mode.value === 'register'
        ? await store.register({ firstName: form.first.trim(), lastName: form.last.trim(), phone, otp: form.otp.trim() })
        : await store.loginByPhone(phone, form.otp.trim())
    message.success(mode.value === 'register' ? `ثبت‌نام موفق — خوش آمدید ${u.firstName}` : `خوش آمدید ${u.firstName}`)
    emit('authed', u.status)
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.code === 'DUPLICATE_PHONE') errors.phone = `شما قبلاً با شماره +۹۸ ${toFa(phone)} ثبت‌نام کرده‌اید.`
      else if (['INVALID_OTP', 'OTP_EXPIRED', 'OTP_TOO_MANY_ATTEMPTS', 'OTP_REQUIRED'].includes(e.code)) {
        errors.otp = e.message
        if (e.code === 'INVALID_OTP') {
          shakeOtp.value = true
          setTimeout(() => (shakeOtp.value = false), 450)
        }
      } else if (e.code === 'NOT_FOUND') errors.phone = 'شماره موبایلی با این مشخصات ثبت‌نام نشده — لطفاً ابتدا ثبت‌نام کنید.'
      else errors.phone = e.message
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="auth card card-pad">
    <div class="auth-tabs" role="tablist">
      <button
        type="button"
        role="tab"
        :aria-selected="mode === 'register'"
        :class="{ active: mode === 'register' }"
        @click="switchMode('register')"
      >
        ثبت‌نام
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="mode === 'login'"
        :class="{ active: mode === 'login' }"
        @click="switchMode('login')"
      >
        ورود
      </button>
    </div>

    <p class="auth-lead">
      {{ mode === 'register' ? 'برای دیدن چالش، ابتدا با شماره موبایل خود ثبت‌نام کنید.' : 'اگر قبلاً ثبت‌نام کرده‌اید، با شماره موبایلتان وارد شوید.' }}
    </p>

    <form class="stack" @submit.prevent="submit">
      <template v-if="mode === 'register'">
        <label class="field">
          <span class="field-label">نام</span>
          <a-input v-model:value="form.first" size="large" :status="errors.first ? 'error' : ''" placeholder="مثلاً امیر" @input="errors.first = ''" />
          <span v-if="errors.first" class="field-err">{{ errors.first }}</span>
        </label>
        <label class="field">
          <span class="field-label">نام خانوادگی</span>
          <a-input v-model:value="form.last" size="large" :status="errors.last ? 'error' : ''" placeholder="مثلاً رضایی" @input="errors.last = ''" />
          <span v-if="errors.last" class="field-err">{{ errors.last }}</span>
        </label>
      </template>

      <label class="field">
        <span class="field-label">شماره موبایل</span>
        <a-input
          v-model:value="form.phone"
          size="large"
          dir="ltr"
          inputmode="tel"
          :status="errors.phone ? 'error' : ''"
          placeholder="9123456789"
          @input="(errors.phone = ''), resetOtp()"
          @keydown.enter.prevent="sendOtp()"
        />
        <span v-if="errors.phone" class="field-err">{{ errors.phone }}</span>
      </label>

      <a-button
        type="dashed"
        size="large"
        block
        :loading="sending"
        :disabled="cooldown > 0 || !phoneOk"
        @click="sendOtp"
      >
        <template #icon><AppIcon name="send" :size="16" /></template>
        {{ cooldown > 0 ? `ارسال مجدد (${toFa(cooldown)})` : otpSent ? 'ارسال مجدد کد' : 'ارسال کد تأیید پیامکی' }}
      </a-button>

      <label v-if="otpSent" class="field" :class="{ shake: shakeOtp }">
        <span class="field-label">کد تأیید پیامک‌شده</span>
        <a-input
          v-model:value="form.otp"
          size="large"
          dir="ltr"
          inputmode="numeric"
          :maxlength="5"
          class="otp-input num"
          :status="errors.otp ? 'error' : ''"
          placeholder="- - - - -"
          @input="errors.otp = ''"
        />
        <span v-if="errors.otp" class="field-err">{{ errors.otp }}</span>
      </label>

      <a-button type="primary" size="large" block html-type="submit" :loading="submitting" :disabled="!otpSent">
        {{ mode === 'register' ? 'ثبت‌نام و مشاهده چالش' : 'ورود' }}
      </a-button>

      <p class="auth-hint faint">
        نشست شما روی این مرورگر به‌خاطر سپرده می‌شود؛ دفعه‌ی بعد بدون کد وارد می‌شوید.
      </p>
    </form>
  </div>
</template>

<style scoped>
.auth {
  max-width: min(480px, 100%);
  width: 100%;
  margin: 0 auto;
}
.auth-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  padding: 4px;
  background: var(--surface-2);
  border-radius: var(--radius);
  margin-bottom: var(--space-4);
}
.auth-tabs button {
  padding: 9px 0;
  border: none;
  background: transparent;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.88rem;
  color: var(--text-dim);
  transition: all var(--dur) var(--ease);
}
.auth-tabs button.active {
  background: var(--surface);
  color: var(--text);
  box-shadow: var(--shadow-1);
}
.auth-lead {
  font-size: 0.86rem;
  color: var(--text-dim);
  margin-bottom: var(--space-4);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-dim);
}
.field-err {
  font-size: 0.75rem;
  color: var(--danger);
  line-height: 1.7;
}
.otp-input {
  letter-spacing: 0.5em;
  text-align: center;
  font-size: 1.15rem;
}
.auth-hint {
  font-size: 0.72rem;
  margin: 0;
}
.shake {
  animation: shake 0.4s var(--ease);
}
@keyframes shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-5px); }
  75% { transform: translateX(5px); }
}

/* Genuinely short screens only — a gentle nudge so the form still fits without
   a page scroll. The comfortable layout above is unchanged on normal screens. */
@media (max-height: 680px) {
  .auth :deep(.stack) {
    gap: var(--space-3);
  }
  .auth-tabs {
    margin-bottom: var(--space-3);
  }
  .auth-lead {
    margin-bottom: var(--space-3);
  }
  .auth-hint {
    display: none;
  }
}
</style>
